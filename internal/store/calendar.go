package store

import (
	"context"
	"fmt"
	"sort"
	"time"
)

func (s *Store) CalendarMonth(ctx context.Context, householdID int64, month time.Time) (CalendarMonth, error) {
	location := time.Local
	if month.IsZero() {
		month = time.Now()
	}
	monthStart := time.Date(month.In(location).Year(), month.In(location).Month(), 1, 0, 0, 0, 0, location)
	gridStart := monthStart.AddDate(0, 0, -int(monthStart.Weekday()))
	gridEnd := gridStart.AddDate(0, 0, 42)

	entriesByDay := map[string][]CalendarEntry{}
	if err := s.addCalendarEvents(ctx, householdID, gridStart, gridEnd, entriesByDay); err != nil {
		return CalendarMonth{}, err
	}
	if err := s.addCalendarTasks(ctx, householdID, gridStart, gridEnd, entriesByDay); err != nil {
		return CalendarMonth{}, err
	}
	if err := s.addCalendarProjects(ctx, householdID, gridStart, gridEnd, entriesByDay); err != nil {
		return CalendarMonth{}, err
	}
	if err := s.addCalendarRoutines(ctx, householdID, gridStart, gridEnd, entriesByDay); err != nil {
		return CalendarMonth{}, err
	}

	todayKey := dayKey(time.Now())
	days := make([]CalendarDay, 0, 42)
	for day := gridStart; day.Before(gridEnd); day = day.AddDate(0, 0, 1) {
		key := dayKey(day)
		entries := entriesByDay[key]
		if entries == nil {
			entries = []CalendarEntry{}
		}
		sort.SliceStable(entries, func(i, j int) bool {
			left, right := entries[i], entries[j]
			if left.Time != nil && right.Time != nil && !left.Time.Equal(*right.Time) {
				return left.Time.Before(*right.Time)
			}
			if left.Time != nil != (right.Time != nil) {
				return left.Time != nil
			}
			if left.Type != right.Type {
				return left.Type < right.Type
			}
			return left.Title < right.Title
		})
		days = append(days, CalendarDay{
			Date:    day,
			InMonth: day.Month() == monthStart.Month(),
			IsToday: key == todayKey,
			Entries: entries,
		})
	}

	return CalendarMonth{
		Month:     monthStart,
		PrevMonth: monthStart.AddDate(0, -1, 0).Format("2006-01"),
		NextMonth: monthStart.AddDate(0, 1, 0).Format("2006-01"),
		Days:      days,
	}, nil
}

func (s *Store) addCalendarEvents(ctx context.Context, householdID int64, start, end time.Time, entries map[string][]CalendarEntry) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, starts_at, location
		FROM events
		WHERE household_id = $1
			AND starts_at >= $2
			AND starts_at < $3
		ORDER BY starts_at
	`, householdID, start, end)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var title, location string
		var startsAt time.Time
		if err := rows.Scan(&id, &title, &startsAt, &location); err != nil {
			return err
		}
		meta := "Appointment"
		if location != "" {
			meta = location
		}
		entries[dayKey(startsAt)] = append(entries[dayKey(startsAt)], CalendarEntry{
			Type:  "appointment",
			ID:    id,
			Title: title,
			URL:   fmt.Sprintf("/events/%d", id),
			Time:  &startsAt,
			Meta:  meta,
		})
	}
	return rows.Err()
}

func (s *Store) addCalendarTasks(ctx context.Context, householdID int64, start, end time.Time, entries map[string][]CalendarEntry) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.title, t.status, t.priority, t.due_at, COALESCE(assigned.name, '')
		FROM tasks t
		LEFT JOIN users assigned ON assigned.id = t.assigned_to
		WHERE t.household_id = $1
			AND t.status <> 'archived'
			AND t.due_at >= $2
			AND t.due_at < $3
		ORDER BY t.due_at
	`, householdID, start, end)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var title, status, priority, assigned string
		var dueAt time.Time
		if err := rows.Scan(&id, &title, &status, &priority, &dueAt, &assigned); err != nil {
			return err
		}
		meta := "Task"
		if assigned != "" {
			meta = "Task · " + assigned
		}
		entries[dayKey(dueAt)] = append(entries[dayKey(dueAt)], CalendarEntry{
			Type:     "task",
			ID:       id,
			Title:    title,
			URL:      fmt.Sprintf("/tasks/%d", id),
			Time:     &dueAt,
			Meta:     meta,
			Status:   status,
			Priority: priority,
		})
	}
	return rows.Err()
}

func (s *Store) addCalendarProjects(ctx context.Context, householdID int64, start, end time.Time, entries map[string][]CalendarEntry) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, status, priority, due_date
		FROM projects
		WHERE household_id = $1
			AND status <> 'archived'
			AND due_date >= $2::date
			AND due_date < $3::date
		ORDER BY due_date
	`, householdID, start, end)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var title, status, priority string
		var dueDate time.Time
		if err := rows.Scan(&id, &title, &status, &priority, &dueDate); err != nil {
			return err
		}
		entries[dayKey(dueDate)] = append(entries[dayKey(dueDate)], CalendarEntry{
			Type:     "project",
			ID:       id,
			Title:    title,
			URL:      fmt.Sprintf("/projects/%d", id),
			Meta:     "Project due",
			Status:   status,
			Priority: priority,
		})
	}
	return rows.Err()
}

func (s *Store) addCalendarRoutines(ctx context.Context, householdID int64, start, end time.Time, entries map[string][]CalendarEntry) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.title, r.cadence, r.next_due_at, COALESCE(assigned.name, '')
		FROM routines r
		LEFT JOIN users assigned ON assigned.id = r.assigned_to
		WHERE r.household_id = $1
			AND r.status = 'active'
			AND r.next_due_at >= $2
			AND r.next_due_at < $3
			AND NOT EXISTS (
				SELECT 1
				FROM tasks t
				WHERE t.routine_id = r.id
					AND t.status <> 'archived'
					AND t.due_at IS NOT DISTINCT FROM r.next_due_at
			)
		ORDER BY r.next_due_at
	`, householdID, start, end)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var title, cadence, assigned string
		var nextDueAt time.Time
		if err := rows.Scan(&id, &title, &cadence, &nextDueAt, &assigned); err != nil {
			return err
		}
		meta := "Routine · " + cadence
		if assigned != "" {
			meta += " · " + assigned
		}
		entries[dayKey(nextDueAt)] = append(entries[dayKey(nextDueAt)], CalendarEntry{
			Type:  "routine",
			ID:    id,
			Title: title,
			URL:   fmt.Sprintf("/routines/%d", id),
			Time:  &nextDueAt,
			Meta:  meta,
		})
	}
	return rows.Err()
}

func dayKey(t time.Time) string {
	return t.In(time.Local).Format("2006-01-02")
}
