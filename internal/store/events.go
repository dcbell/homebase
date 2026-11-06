package store

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
)

type EventInput struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Location    string    `json:"location"`
	// AttendeeIDs are the Members the Appointment is for. Empty means the
	// whole Household. On update, omitting the field (nil) keeps the current
	// Attendees and an empty list clears them.
	AttendeeIDs []int64 `json:"attendee_ids"`
	// CalendarUID is the UID a device chose when it created the Appointment
	// over CalDAV. It is only set on create.
	CalendarUID string `json:"-"`
}

// ErrAttendeeNotMember is returned when an Attendee id is not a Member of the
// Household.
var ErrAttendeeNotMember = errors.New("attendees must be members of the household")

const eventColumns = `id, household_id, title, description, starts_at, ends_at, location, status, created_by, created_at, updated_at, COALESCE(calendar_uid, '')`

type eventScanner interface {
	Scan(dest ...any) error
}

func scanEvent(scanner eventScanner) (Event, error) {
	var e Event
	err := scanner.Scan(&e.ID, &e.HouseholdID, &e.Title, &e.Description, &e.StartsAt, &e.EndsAt, &e.Location, &e.Status, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt, &e.CalendarUID)
	return e, err
}

// queryer is satisfied by *sql.DB and *sql.Tx.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (s *Store) ListEvents(ctx context.Context, householdID int64) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+eventColumns+`
		FROM events
		WHERE household_id = $1
		  AND status <> 'archived'
		  AND starts_at >= now() - interval '1 day'
		ORDER BY starts_at
		LIMIT 50
	`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	if err := loadAttendees(ctx, s.db, events); err != nil {
		return nil, err
	}
	return events, nil
}

// ListWholeHouseholdEventsAfter returns the Household's active Appointments
// with no Attendees (so for every Member) that start after the given time.
func (s *Store) ListWholeHouseholdEventsAfter(ctx context.Context, householdID int64, after time.Time) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+eventColumns+`
		FROM events
		WHERE household_id = $1
		  AND status <> 'archived'
		  AND starts_at > $2
		  AND NOT EXISTS (SELECT 1 FROM event_attendees ea WHERE ea.event_id = events.id)
		ORDER BY starts_at, id
	`, householdID, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		e.Attendees = []Attendee{}
		e.AttendeeIDs = []int64{}
		events = append(events, e)
	}
	return events, rows.Err()
}

// ListAppointmentsConcerning returns the Household's active Appointments that
// concern a Member, meaning the Member is an Attendee or there are no
// Attendees, and that overlap [from, to). Attendees are included.
func (s *Store) ListAppointmentsConcerning(ctx context.Context, householdID, userID int64, from, to time.Time) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+eventColumns+`
		FROM events
		WHERE household_id = $1
		  AND status <> 'archived'
		  AND ends_at > $3
		  AND starts_at < $4
		  AND (
			EXISTS (SELECT 1 FROM event_attendees ea WHERE ea.event_id = events.id AND ea.user_id = $2)
			OR NOT EXISTS (SELECT 1 FROM event_attendees ea WHERE ea.event_id = events.id)
		  )
		ORDER BY starts_at, id
	`, householdID, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	if err := loadAttendees(ctx, s.db, events); err != nil {
		return nil, err
	}
	return events, nil
}

func validateEventInput(input *EventInput) error {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return errors.New("title is required")
	}
	if input.StartsAt.IsZero() || input.EndsAt.IsZero() || !input.EndsAt.After(input.StartsAt) {
		return errors.New("valid starts_at and ends_at are required")
	}
	return nil
}

func (s *Store) CreateEvent(ctx context.Context, householdID, userID int64, input EventInput) (Event, error) {
	if err := validateEventInput(&input); err != nil {
		return Event{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, err
	}
	defer tx.Rollback()

	e, err := scanEvent(tx.QueryRowContext(ctx, `
		INSERT INTO events (household_id, title, description, starts_at, ends_at, location, created_by, calendar_uid)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''))
		RETURNING `+eventColumns+`
	`, householdID, input.Title, input.Description, input.StartsAt, input.EndsAt, input.Location, userID, input.CalendarUID))
	if err != nil {
		return Event{}, err
	}
	if err := setAttendees(ctx, tx, householdID, e.ID, input.AttendeeIDs); err != nil {
		return Event{}, err
	}
	events := []Event{e}
	if err := loadAttendees(ctx, tx, events); err != nil {
		return Event{}, err
	}
	return events[0], tx.Commit()
}

func (s *Store) GetEvent(ctx context.Context, householdID, id int64) (Event, error) {
	return getEvent(ctx, s.db, householdID, id, false)
}

func getEvent(ctx context.Context, q queryer, householdID, id int64, forUpdate bool) (Event, error) {
	query := `
		SELECT ` + eventColumns + `
		FROM events
		WHERE household_id = $1 AND id = $2`
	if forUpdate {
		query += " FOR UPDATE"
	}
	e, err := scanEvent(q.QueryRowContext(ctx, query, householdID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, err
	}
	events := []Event{e}
	if err := loadAttendees(ctx, q, events); err != nil {
		return Event{}, err
	}
	return events[0], nil
}

// FindEventByCalendarUID returns the Household's active Appointment that a
// device created with the given UID, or ErrNotFound.
func (s *Store) FindEventByCalendarUID(ctx context.Context, householdID int64, uid string) (Event, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM events
		WHERE household_id = $1 AND calendar_uid = $2 AND status <> 'archived'
	`, householdID, uid).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, err
	}
	return s.GetEvent(ctx, householdID, id)
}

// UpdateEvent replaces an Appointment's fields and returns it as it is now and
// as it was before, Attendees included.
func (s *Store) UpdateEvent(ctx context.Context, householdID, id int64, input EventInput) (updated Event, previous Event, err error) {
	if err := validateEventInput(&input); err != nil {
		return Event{}, Event{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, Event{}, err
	}
	defer tx.Rollback()

	previous, err = getEvent(ctx, tx, householdID, id, true)
	if err != nil {
		return Event{}, Event{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE events
		SET title = $3,
			description = $4,
			starts_at = $5,
			ends_at = $6,
			location = $7,
			updated_at = now()
		WHERE household_id = $1 AND id = $2
	`, householdID, id, input.Title, input.Description, input.StartsAt, input.EndsAt, input.Location); err != nil {
		return Event{}, Event{}, err
	}
	if input.AttendeeIDs != nil {
		if err := setAttendees(ctx, tx, householdID, id, input.AttendeeIDs); err != nil {
			return Event{}, Event{}, err
		}
	}
	updated, err = getEvent(ctx, tx, householdID, id, false)
	if err != nil {
		return Event{}, Event{}, err
	}
	return updated, previous, tx.Commit()
}

// ArchiveEvent archives an Appointment, which is how one is removed.
func (s *Store) ArchiveEvent(ctx context.Context, householdID, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE events
		SET status = 'archived', updated_at = now()
		WHERE household_id = $1 AND id = $2
	`, householdID, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// setAttendees replaces an Appointment's Attendees after checking every one is
// a Member of the Household.
func setAttendees(ctx context.Context, q queryer, householdID, eventID int64, attendeeIDs []int64) error {
	ids := uniqueIDs(attendeeIDs)
	if len(ids) > 0 {
		var members int
		if err := q.QueryRowContext(ctx, `
			SELECT count(*) FROM household_members
			WHERE household_id = $1 AND user_id = ANY($2)
		`, householdID, ids).Scan(&members); err != nil {
			return err
		}
		if members != len(ids) {
			return ErrAttendeeNotMember
		}
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM event_attendees WHERE event_id = $1`, eventID); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO event_attendees (event_id, user_id)
		SELECT $1, unnest($2::bigint[])
	`, eventID, ids)
	return err
}

// loadAttendees fills in the Attendees of each event, ordered by name.
func loadAttendees(ctx context.Context, q queryer, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	ids := make([]int64, len(events))
	byID := make(map[int64]*Event, len(events))
	for i := range events {
		events[i].Attendees = []Attendee{}
		events[i].AttendeeIDs = []int64{}
		ids[i] = events[i].ID
		byID[events[i].ID] = &events[i]
	}
	rows, err := q.QueryContext(ctx, `
		SELECT ea.event_id, u.id, u.name
		FROM event_attendees ea
		JOIN users u ON u.id = ea.user_id
		WHERE ea.event_id = ANY($1)
		ORDER BY u.name, u.id
	`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var eventID int64
		var a Attendee
		if err := rows.Scan(&eventID, &a.ID, &a.Name); err != nil {
			return err
		}
		e := byID[eventID]
		e.Attendees = append(e.Attendees, a)
		e.AttendeeIDs = append(e.AttendeeIDs, a.ID)
	}
	return rows.Err()
}

func uniqueIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
