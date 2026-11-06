package api

import (
	"context"
	"errors"
	"strings"

	"homebase/internal/caldav"
	"homebase/internal/store"
)

// caldavRoutes mounts the CalDAV server. It lives in the API process because
// the API owns login and business rules (ADR-0001); the reverse proxy routes
// /caldav/ and /.well-known/caldav here.
func (s *Server) caldavRoutes() {
	dav := caldav.New(calendarAuthenticator{s}, s.clock, s.logger,
		caldav.Appointments{Store: s.store, Saves: caldavSaves{s}, WebBaseURL: s.cfg.WebBaseURL},
		caldav.Tasks{Store: s.store, Saves: caldavSaves{s}, WebBaseURL: s.cfg.WebBaseURL},
		caldav.Deadlines{Store: s.store, WebBaseURL: s.cfg.WebBaseURL},
	)
	s.mux.Handle(caldav.Root, dav)
	s.mux.Handle(strings.TrimSuffix(caldav.Root, "/"), dav)
	s.mux.HandleFunc(caldav.WellKnownPath, dav.WellKnown)
}

// calendarAuthenticator checks CalDAV logins against Calendar Connections.
type calendarAuthenticator struct{ s *Server }

func (a calendarAuthenticator) AuthenticateCalendar(ctx context.Context, username, password string) (caldav.CalendarLogin, error) {
	login, err := a.s.store.AuthenticateCalendarConnection(ctx, username, password, a.s.clock.Now())
	if errors.Is(err, store.ErrNotFound) {
		return caldav.CalendarLogin{}, caldav.ErrUnauthorized
	}
	if err != nil {
		return caldav.CalendarLogin{}, err
	}
	return caldav.CalendarLogin{
		UserID:       login.User.ID,
		HouseholdID:  login.Household.ID,
		Email:        login.User.Email,
		Name:         login.User.Name,
		ConnectionID: login.Connection.ID,
	}, nil
}

// caldavSaves gives CalDAV collections the shared save operations, acting as
// the connected Member.
type caldavSaves struct{ s *Server }

func (c caldavSaves) actor(m caldav.CalendarLogin) (store.Household, store.User) {
	return store.Household{ID: m.HouseholdID}, store.User{ID: m.UserID, Email: m.Email, Name: m.Name}
}

func (c caldavSaves) CreateAppointment(ctx context.Context, m caldav.CalendarLogin, input store.EventInput) (store.Event, error) {
	household, actor := c.actor(m)
	return c.s.CreateAppointment(ctx, household, actor, input)
}

func (c caldavSaves) UpdateAppointment(ctx context.Context, m caldav.CalendarLogin, id int64, input store.EventInput) (store.Event, error) {
	household, actor := c.actor(m)
	return c.s.UpdateAppointment(ctx, household, actor, id, input)
}

func (c caldavSaves) ArchiveAppointment(ctx context.Context, m caldav.CalendarLogin, id int64) error {
	household, actor := c.actor(m)
	return c.s.ArchiveAppointment(ctx, household, actor, id)
}

func (c caldavSaves) CreateTask(ctx context.Context, m caldav.CalendarLogin, input store.TaskInput) (store.Task, error) {
	household, actor := c.actor(m)
	return c.s.CreateTask(ctx, household, actor, input)
}

func (c caldavSaves) UpdateTask(ctx context.Context, m caldav.CalendarLogin, id int64, input store.TaskInput) (store.Task, error) {
	household, actor := c.actor(m)
	return c.s.UpdateTask(ctx, household, actor, id, input)
}

func (c caldavSaves) CompleteTask(ctx context.Context, m caldav.CalendarLogin, id int64) (store.Task, error) {
	household, actor := c.actor(m)
	return c.s.CompleteTask(ctx, household, actor, id)
}

func (c caldavSaves) ReopenTask(ctx context.Context, m caldav.CalendarLogin, id int64) (store.Task, error) {
	household, actor := c.actor(m)
	return c.s.ReopenTask(ctx, household, actor, id)
}

func (c caldavSaves) ArchiveTask(ctx context.Context, m caldav.CalendarLogin, id int64) error {
	household, actor := c.actor(m)
	return c.s.ArchiveTask(ctx, household, actor, id)
}
