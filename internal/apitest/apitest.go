// Package apitest drives the API server over HTTP for integration tests.
//
//	h := apitest.New(t)
//	household := h.SeedHousehold()
//	task := apitest.Decode[store.Task](t, household.Owner.Post("/api/v1/tasks", body).Expect(http.StatusCreated))
//	h.Clock.Advance(24 * time.Hour)
//
// Each Harness has its own migrated database schema (see storetest) and a fake
// Clock, so tests can run in parallel. Requests go straight to the server's
// handler; no network listener is involved.
package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"homebase/internal/api"
	"homebase/internal/clock"
	"homebase/internal/config"
	"homebase/internal/store"
	"homebase/internal/store/storetest"
)

// DefaultNow is where every Harness's fake Clock starts.
var DefaultNow = time.Date(2030, time.January, 15, 12, 0, 0, 0, time.UTC)

// sessionTTL keeps test sessions valid whatever the fake Clock says, because
// session expiry is checked against the database's now().
const sessionTTL = 24 * time.Hour

// Harness is an API server wired to an isolated test database and a fake Clock.
type Harness struct {
	t      testing.TB
	Store  *store.Store
	Clock  *clock.Fake
	Config config.Config
	Server *api.Server

	seeded int
}

// New builds a Harness. opts are applied after the fake Clock, so a test can
// override any dependency.
func New(t testing.TB, opts ...api.Option) *Harness {
	t.Helper()
	h := &Harness{
		t:     t,
		Store: storetest.Open(t),
		Clock: clock.NewFake(DefaultNow),
		Config: config.Config{
			Env:                 "test",
			SessionCookieName:   "homebase_session",
			WebBaseURL:          "http://homebase.test",
			BootstrapOwnerEmail: "owner@example.test",
			DocumentUploadDir:   t.TempDir(),
			DocumentMaxBytes:    10 << 20,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h.Server = api.New(h.Config, h.Store, logger, append([]api.Option{api.WithClock(h.Clock)}, opts...)...)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.Server.Close(ctx)
	})
	return h
}

// Household is a seeded Household with one Owner and one Member, each signed
// in with a browser session.
type Household struct {
	store.Household
	Owner  *Client
	Member *Client
}

// SeedHousehold creates a new Household with an Owner and a Member. Call it
// more than once for tests that span Households.
func (h *Harness) SeedHousehold() Household {
	h.t.Helper()
	ctx := context.Background()
	h.seeded++
	n := h.seeded

	owner, household, err := h.Store.UpsertUserWithHousehold(ctx,
		fmt.Sprintf("test-owner-%d", n), fmt.Sprintf("owner%d@example.test", n), fmt.Sprintf("Owner %d", n), "")
	if err != nil {
		h.t.Fatalf("seed Owner: %v", err)
	}
	member, err := h.Store.AddMember(ctx, household.ID, store.MemberInput{
		Email: fmt.Sprintf("member%d@example.test", n),
		Name:  fmt.Sprintf("Member %d", n),
		Role:  "member",
	})
	if err != nil {
		h.t.Fatalf("seed Member: %v", err)
	}

	return Household{
		Household: household,
		Owner:     h.SignIn(owner, household.ID),
		Member:    h.SignIn(member, household.ID),
	}
}

// SignIn returns a Client that makes requests as user in the given Household
// using a browser session.
func (h *Harness) SignIn(user store.User, householdID int64) *Client {
	h.t.Helper()
	session, err := h.Store.CreateSession(context.Background(), user.ID, householdID, sessionTTL)
	if err != nil {
		h.t.Fatalf("create session for user %d: %v", user.ID, err)
	}
	cookie := &http.Cookie{Name: h.Config.SessionCookieName, Value: session.ID}
	return &Client{h: h, User: user, HouseholdID: householdID, authorize: func(r *http.Request) { r.AddCookie(cookie) }}
}

// Anonymous returns a Client that sends no credentials.
func (h *Harness) Anonymous() *Client {
	return &Client{h: h, authorize: func(*http.Request) {}}
}

// Client makes requests to the Harness's server as one Member.
type Client struct {
	h           *Harness
	User        store.User
	HouseholdID int64
	authorize   func(*http.Request)
}

// WithToken returns a Client for the same Member that authenticates with a new
// API token of the given scope ("read" or "write") instead of a session.
func (c *Client) WithToken(scope string) *Client {
	c.h.t.Helper()
	token, err := c.h.Store.CreateAPIToken(context.Background(), c.User.ID, c.HouseholdID, store.APITokenInput{Name: "test " + scope, Scope: scope})
	if err != nil {
		c.h.t.Fatalf("create %s API token: %v", scope, err)
	}
	bearer := "Bearer " + token.Token
	return &Client{h: c.h, User: c.User, HouseholdID: c.HouseholdID, authorize: func(r *http.Request) { r.Header.Set("Authorization", bearer) }}
}

func (c *Client) Get(path string) *Response {
	c.h.t.Helper()
	return c.Do(http.MethodGet, path, nil)
}

func (c *Client) Post(path string, body any) *Response {
	c.h.t.Helper()
	return c.Do(http.MethodPost, path, body)
}

func (c *Client) Patch(path string, body any) *Response {
	c.h.t.Helper()
	return c.Do(http.MethodPatch, path, body)
}

func (c *Client) Delete(path string) *Response {
	c.h.t.Helper()
	return c.Do(http.MethodDelete, path, nil)
}

// Do sends a request with body encoded as JSON (no body when nil).
func (c *Client) Do(method, path string, body any) *Response {
	c.h.t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			c.h.t.Fatalf("encode %s %s body: %v", method, path, err)
		}
		reader = bytes.NewReader(encoded)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.authorize(req)

	rec := httptest.NewRecorder()
	c.h.Server.ServeHTTP(rec, req)
	// Push goes out in the background; wait for it so tests see its effects
	// (Pushes sent, gone Push Subscriptions forgotten) once the request returns.
	c.h.Server.WaitForPush()
	return &Response{t: c.h.t, Method: method, Path: path, Status: rec.Code, Header: rec.Header(), Body: rec.Body.Bytes()}
}

// Response is a recorded API response.
type Response struct {
	t      testing.TB
	Method string
	Path   string
	Status int
	Header http.Header
	Body   []byte
}

// Expect fails the test unless the response has the given status.
func (r *Response) Expect(status int) *Response {
	r.t.Helper()
	if r.Status != status {
		r.t.Fatalf("%s %s: status = %d, want %d; body: %s", r.Method, r.Path, r.Status, status, r.Body)
	}
	return r
}

// Decode unmarshals a JSON response body into a T.
func Decode[T any](t testing.TB, r *Response) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(r.Body, &v); err != nil {
		t.Fatalf("%s %s: decode %T: %v; body: %s", r.Method, r.Path, v, err, r.Body)
	}
	return v
}
