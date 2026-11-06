package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// calendarSecretPrefix marks Calendar Connection secrets so they can't be
// mistaken for API tokens.
const calendarSecretPrefix = "hbcal_"

// Calendar Connection kinds.
const (
	CalendarConnectionIOS   = "ios"
	CalendarConnectionOther = "other"
)

// CalendarConnection is one device a Member has connected to Homebase over
// CalDAV. Its secret is never stored or returned after creation.
type CalendarConnection struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"user_id"`
	HouseholdID  int64      `json:"household_id"`
	Name         string     `json:"name"`
	Kind         string     `json:"kind"`
	SecretPrefix string     `json:"secret_prefix"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// CalendarConnectionInput names a new Calendar Connection.
type CalendarConnectionInput struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// CalendarConnectionWithSecret is a just-created Calendar Connection and its
// secret, the only time the secret is available.
type CalendarConnectionWithSecret struct {
	CalendarConnection
	Secret string
}

// CalendarLogin is who a CalDAV request authenticated as.
type CalendarLogin struct {
	Connection CalendarConnection
	User       User
	Household  Household
}

const calendarConnectionColumns = `id, user_id, household_id, name, kind, secret_prefix, last_used_at, revoked_at, created_at`

func scanCalendarConnection(scanner interface{ Scan(dest ...any) error }) (CalendarConnection, error) {
	var c CalendarConnection
	err := scanner.Scan(&c.ID, &c.UserID, &c.HouseholdID, &c.Name, &c.Kind, &c.SecretPrefix, &c.LastUsedAt, &c.RevokedAt, &c.CreatedAt)
	return c, err
}

// ListCalendarConnections returns a Member's Calendar Connections in a
// Household, active ones first.
func (s *Store) ListCalendarConnections(ctx context.Context, userID, householdID int64) ([]CalendarConnection, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+calendarConnectionColumns+`
		FROM calendar_connections
		WHERE user_id = $1 AND household_id = $2
		ORDER BY revoked_at NULLS FIRST, created_at DESC, id DESC
	`, userID, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	connections := []CalendarConnection{}
	for rows.Next() {
		c, err := scanCalendarConnection(rows)
		if err != nil {
			return nil, err
		}
		connections = append(connections, c)
	}
	return connections, rows.Err()
}

// CreateCalendarConnection generates a secret for a new Calendar Connection
// and stores only its hash.
func (s *Store) CreateCalendarConnection(ctx context.Context, userID, householdID int64, input CalendarConnectionInput) (CalendarConnectionWithSecret, error) {
	c, secret, _, err := s.insertCalendarConnection(ctx, userID, householdID, input, nil)
	if err != nil {
		return CalendarConnectionWithSecret{}, err
	}
	return CalendarConnectionWithSecret{CalendarConnection: c, Secret: secret}, nil
}

// CreateCalendarConnectionWithProfileLink creates a Calendar Connection whose
// secret is never revealed and returns a token for a single-use setup profile
// link that expires at linkExpiresAt. Redeeming the link issues the secret.
func (s *Store) CreateCalendarConnectionWithProfileLink(ctx context.Context, userID, householdID int64, input CalendarConnectionInput, linkExpiresAt time.Time) (CalendarConnection, string, error) {
	c, _, token, err := s.insertCalendarConnection(ctx, userID, householdID, input, &linkExpiresAt)
	return c, token, err
}

func (s *Store) insertCalendarConnection(ctx context.Context, userID, householdID int64, input CalendarConnectionInput, linkExpiresAt *time.Time) (CalendarConnection, string, string, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return CalendarConnection{}, "", "", errors.New("name is required")
	}
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	if input.Kind != CalendarConnectionIOS && input.Kind != CalendarConnectionOther {
		return CalendarConnection{}, "", "", errors.New("kind must be ios or other")
	}

	secret, err := generateSecret(calendarSecretPrefix)
	if err != nil {
		return CalendarConnection{}, "", "", err
	}
	var token string
	var tokenHash *string
	if linkExpiresAt != nil {
		if token, err = generateSecret(""); err != nil {
			return CalendarConnection{}, "", "", err
		}
		hash := hashSecret(token)
		tokenHash = &hash
	}
	c, err := scanCalendarConnection(s.db.QueryRowContext(ctx, `
		INSERT INTO calendar_connections (user_id, household_id, name, kind, secret_hash, secret_prefix, profile_token_hash, profile_token_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+calendarConnectionColumns+`
	`, userID, householdID, input.Name, input.Kind, hashSecret(secret), tokenDisplayPrefix(secret), tokenHash, linkExpiresAt))
	if err != nil {
		return CalendarConnection{}, "", "", err
	}
	return c, secret, token, nil
}

// CalendarProfile is what a redeemed setup profile link carries: the
// connection, its Member, and the connection's newly issued secret.
type CalendarProfile struct {
	Connection CalendarConnection
	User       User
	Secret     string
}

// RedeemCalendarProfileLink uses up a setup profile link: if token belongs to
// an active Calendar Connection, its Member still belongs to the Household and
// the link hasn't expired as of now, it issues the connection a new secret and
// clears the link. Any failure is ErrNotFound.
func (s *Store) RedeemCalendarProfileLink(ctx context.Context, token string, now time.Time) (CalendarProfile, error) {
	if strings.TrimSpace(token) == "" {
		return CalendarProfile{}, ErrNotFound
	}
	secret, err := generateSecret(calendarSecretPrefix)
	if err != nil {
		return CalendarProfile{}, err
	}
	profile := CalendarProfile{Secret: secret}
	c := &profile.Connection
	u := &profile.User
	err = s.db.QueryRowContext(ctx, `
		UPDATE calendar_connections cc
		SET secret_hash = $3, secret_prefix = $4, profile_token_hash = NULL, profile_token_expires_at = NULL
		FROM users u, household_members hm
		WHERE cc.profile_token_hash = $1
		  AND cc.profile_token_expires_at > $2
		  AND cc.revoked_at IS NULL
		  AND u.id = cc.user_id
		  AND hm.user_id = cc.user_id AND hm.household_id = cc.household_id
		RETURNING cc.id, cc.user_id, cc.household_id, cc.name, cc.kind, cc.secret_prefix, cc.last_used_at, cc.revoked_at, cc.created_at,
		          u.id, u.email, u.name, u.avatar_url, u.created_at
	`, hashSecret(token), now, hashSecret(secret), tokenDisplayPrefix(secret)).Scan(
		&c.ID, &c.UserID, &c.HouseholdID, &c.Name, &c.Kind, &c.SecretPrefix, &c.LastUsedAt, &c.RevokedAt, &c.CreatedAt,
		&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return CalendarProfile{}, ErrNotFound
	}
	if err != nil {
		return CalendarProfile{}, err
	}
	return profile, nil
}

// RevokeCalendarConnection disconnects one of a Member's Calendar
// Connections. It returns ErrNotFound if there is no such active connection.
func (s *Store) RevokeCalendarConnection(ctx context.Context, userID, householdID, id int64, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE calendar_connections
		SET revoked_at = $4
		WHERE id = $1 AND user_id = $2 AND household_id = $3 AND revoked_at IS NULL
	`, id, userID, householdID, now)
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

// AuthenticateCalendarConnection checks a CalDAV login: the secret must belong
// to an active Calendar Connection, the username must be its Member's email,
// and that Member must still belong to the Household. On success it records
// now as the connection's last use. Any failure is ErrNotFound.
func (s *Store) AuthenticateCalendarConnection(ctx context.Context, email, secret string, now time.Time) (CalendarLogin, error) {
	email = strings.TrimSpace(email)
	if email == "" || !strings.HasPrefix(strings.TrimSpace(secret), calendarSecretPrefix) {
		return CalendarLogin{}, ErrNotFound
	}
	var login CalendarLogin
	c := &login.Connection
	u := &login.User
	h := &login.Household
	err := s.db.QueryRowContext(ctx, `
		UPDATE calendar_connections cc
		SET last_used_at = $3
		FROM users u, household_members hm, households h
		WHERE cc.secret_hash = $1
		  AND cc.revoked_at IS NULL
		  AND u.id = cc.user_id
		  AND lower(u.email) = lower($2)
		  AND hm.user_id = cc.user_id AND hm.household_id = cc.household_id
		  AND h.id = cc.household_id
		RETURNING cc.id, cc.user_id, cc.household_id, cc.name, cc.kind, cc.secret_prefix, cc.last_used_at, cc.revoked_at, cc.created_at,
		          u.id, u.email, u.name, u.avatar_url, u.created_at,
		          h.id, h.name, hm.role, h.created_at
	`, hashSecret(secret), email, now).Scan(
		&c.ID, &c.UserID, &c.HouseholdID, &c.Name, &c.Kind, &c.SecretPrefix, &c.LastUsedAt, &c.RevokedAt, &c.CreatedAt,
		&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt,
		&h.ID, &h.Name, &h.Role, &h.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return CalendarLogin{}, ErrNotFound
	}
	if err != nil {
		return CalendarLogin{}, err
	}
	return login, nil
}
