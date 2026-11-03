package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Notification kinds.
const (
	NotificationAssignment          = "assignment"
	NotificationTaskReminder        = "task_reminder"
	NotificationAppointmentReminder = "appointment_reminder"
)

// Notification subject types: the entity a Notification is about.
const (
	SubjectTask        = "task"
	SubjectListItem    = "list_item"
	SubjectRoutine     = "routine"
	SubjectAppointment = "appointment"
)

const (
	defaultNotificationLimit = 50
	maxNotificationLimit     = 100
)

// NotificationInput is a Notification to store. The Notifier decides whether
// and to whom one is created; the store only records it.
type NotificationInput struct {
	HouseholdID int64
	RecipientID int64
	Kind        string
	Stage       string
	SubjectType string
	SubjectID   int64
	Title       string
	Body        string
	CreatedAt   time.Time
}

// CreateNotification stores a Notification and returns it with its URL.
func (s *Store) CreateNotification(ctx context.Context, input NotificationInput) (Notification, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO notifications (household_id, recipient_id, kind, stage, subject_type, subject_id, title, body, created_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8, $9)
		RETURNING id
	`, input.HouseholdID, input.RecipientID, input.Kind, input.Stage, input.SubjectType, input.SubjectID, input.Title, input.Body, input.CreatedAt).Scan(&id)
	if err != nil {
		return Notification{}, err
	}
	return s.getNotification(ctx, input.HouseholdID, input.RecipientID, id)
}

const notificationColumns = `
	n.id, n.household_id, n.recipient_id, n.kind, COALESCE(n.stage, ''), n.subject_type, n.subject_id, li.list_id, n.title, n.body, n.created_at, n.read_at
	FROM notifications n
	LEFT JOIN list_items li ON n.subject_type = 'list_item' AND li.id = n.subject_id
`

func (s *Store) getNotification(ctx context.Context, householdID, recipientID, id int64) (Notification, error) {
	n, err := scanNotification(s.db.QueryRowContext(ctx, `
		SELECT `+notificationColumns+`
		WHERE n.household_id = $1 AND n.recipient_id = $2 AND n.id = $3
	`, householdID, recipientID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Notification{}, ErrNotFound
	}
	return n, err
}

// ListNotifications returns a page of a Member's Notifications, newest first,
// with their Unread count. beforeID, when non-zero, continues from an earlier
// page.
func (s *Store) ListNotifications(ctx context.Context, householdID, recipientID int64, limit int, beforeID int64) (NotificationList, error) {
	if limit <= 0 {
		limit = defaultNotificationLimit
	}
	if limit > maxNotificationLimit {
		limit = maxNotificationLimit
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT `+notificationColumns+`
		WHERE n.household_id = $1 AND n.recipient_id = $2 AND ($3 = 0 OR n.id < $3)
		ORDER BY n.id DESC
		LIMIT $4
	`, householdID, recipientID, beforeID, limit)
	if err != nil {
		return NotificationList{}, err
	}
	defer rows.Close()

	list := NotificationList{Notifications: []Notification{}}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return NotificationList{}, err
		}
		list.Notifications = append(list.Notifications, n)
	}
	if err := rows.Err(); err != nil {
		return NotificationList{}, err
	}

	list.UnreadCount, err = s.UnreadNotificationCount(ctx, householdID, recipientID)
	return list, err
}

// UnreadNotificationCount is the number of a Member's Unread Notifications.
func (s *Store) UnreadNotificationCount(ctx context.Context, householdID, recipientID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM notifications
		WHERE household_id = $1 AND recipient_id = $2 AND read_at IS NULL
	`, householdID, recipientID).Scan(&count)
	return count, err
}

// MarkNotificationRead marks one of a Member's Notifications read at the given
// time. Marking an already-read Notification keeps its original read_at.
func (s *Store) MarkNotificationRead(ctx context.Context, householdID, recipientID, id int64, at time.Time) (Notification, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE notifications
		SET read_at = COALESCE(read_at, $4)
		WHERE household_id = $1 AND recipient_id = $2 AND id = $3
	`, householdID, recipientID, id, at)
	if err != nil {
		return Notification{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Notification{}, err
	}
	if affected == 0 {
		return Notification{}, ErrNotFound
	}
	return s.getNotification(ctx, householdID, recipientID, id)
}

// MarkAllNotificationsRead marks every Unread Notification of a Member read.
func (s *Store) MarkAllNotificationsRead(ctx context.Context, householdID, recipientID int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE notifications
		SET read_at = $3
		WHERE household_id = $1 AND recipient_id = $2 AND read_at IS NULL
	`, householdID, recipientID, at)
	return err
}

func scanNotification(scanner taskScanner) (Notification, error) {
	var n Notification
	var listID *int64
	err := scanner.Scan(&n.ID, &n.HouseholdID, &n.RecipientID, &n.Kind, &n.Stage, &n.SubjectType, &n.SubjectID, &listID, &n.Title, &n.Body, &n.CreatedAt, &n.ReadAt)
	if err != nil {
		return Notification{}, err
	}
	n.URL = notificationURL(n.SubjectType, n.SubjectID, listID)
	return n, nil
}

// notificationURL is the web page that opens a Notification's subject. List
// Items have no page of their own, so they open their List.
func notificationURL(subjectType string, subjectID int64, listID *int64) string {
	switch subjectType {
	case SubjectTask:
		return fmt.Sprintf("/tasks/%d", subjectID)
	case SubjectRoutine:
		return fmt.Sprintf("/routines/%d", subjectID)
	case SubjectAppointment:
		return fmt.Sprintf("/events/%d", subjectID)
	case SubjectListItem:
		if listID != nil {
			return fmt.Sprintf("/lists/%d", *listID)
		}
		return "/lists"
	default:
		return "/"
	}
}
