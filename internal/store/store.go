package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrations embed.FS

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		body, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}

		if _, err := s.db.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("run migration %s: %w", entry.Name(), err)
		}
	}

	return nil
}

func (s *Store) UpsertUserWithHousehold(ctx context.Context, oauthSubject, email, name, avatarURL string) (User, Household, error) {
	if strings.TrimSpace(name) == "" {
		name = email
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, Household{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var user User
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (oauth_subject, email, name, avatar_url)
		VALUES (NULLIF($1, ''), $2, $3, $4)
		ON CONFLICT (email) DO UPDATE SET
			oauth_subject = COALESCE(users.oauth_subject, EXCLUDED.oauth_subject),
			name = EXCLUDED.name,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = now()
		RETURNING id, email, name, avatar_url, created_at
	`, oauthSubject, email, name, avatarURL).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt)
	if err != nil {
		return User{}, Household{}, err
	}

	var household Household
	err = tx.QueryRowContext(ctx, `
		SELECT h.id, h.name, hm.role, h.created_at
		FROM household_members hm
		JOIN households h ON h.id = hm.household_id
		WHERE hm.user_id = $1
		ORDER BY h.created_at
		LIMIT 1
	`, user.ID).Scan(&household.ID, &household.Name, &household.Role, &household.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO households (name)
			VALUES ($1)
			RETURNING id, name, created_at
		`, defaultHouseholdName(user.Name)).Scan(&household.ID, &household.Name, &household.CreatedAt)
		if err != nil {
			return User{}, Household{}, err
		}
		household.Role = "owner"

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO household_members (household_id, user_id, role)
			VALUES ($1, $2, 'owner')
		`, household.ID, user.ID); err != nil {
			return User{}, Household{}, err
		}
	} else if err != nil {
		return User{}, Household{}, err
	}

	if err := tx.Commit(); err != nil {
		return User{}, Household{}, err
	}

	return user, household, nil
}

func (s *Store) CreateSession(ctx context.Context, userID, householdID int64, ttl time.Duration) (Session, error) {
	id, err := randomHex(32)
	if err != nil {
		return Session{}, err
	}

	session := Session{
		ID:          id,
		UserID:      userID,
		HouseholdID: householdID,
		ExpiresAt:   time.Now().Add(ttl).UTC(),
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, household_id, expires_at)
		VALUES ($1, $2, $3, $4)
	`, session.ID, session.UserID, session.HouseholdID, session.ExpiresAt)
	if err != nil {
		return Session{}, err
	}

	return session, nil
}

func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	return err
}

func (s *Store) SessionContext(ctx context.Context, sessionID string) (User, Household, error) {
	var user User
	var household Household

	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.name, u.avatar_url, u.created_at,
		       h.id, h.name, hm.role, h.created_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		JOIN households h ON h.id = s.household_id
		JOIN household_members hm ON hm.household_id = h.id AND hm.user_id = u.id
		WHERE s.id = $1 AND s.expires_at > now()
	`, sessionID).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt,
		&household.ID, &household.Name, &household.Role, &household.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Household{}, ErrNotFound
	}
	if err != nil {
		return User{}, Household{}, err
	}

	return user, household, nil
}

func (s *Store) Dashboard(ctx context.Context, user User, household Household, budgetURL string) (Dashboard, error) {
	members, err := s.ListMembers(ctx, household.ID)
	if err != nil {
		return Dashboard{}, err
	}

	projects, err := s.ListProjects(ctx, household.ID)
	if err != nil {
		return Dashboard{}, err
	}

	tasks, err := s.ListTasks(ctx, household.ID)
	if err != nil {
		return Dashboard{}, err
	}

	events, err := s.ListEvents(ctx, household.ID)
	if err != nil {
		return Dashboard{}, err
	}

	routines, err := s.ListRoutines(ctx, household.ID)
	if err != nil {
		return Dashboard{}, err
	}

	notices, err := s.RoutineNotices(ctx, household.ID)
	if err != nil {
		return Dashboard{}, err
	}

	tileOrder, err := s.DashboardTileOrder(ctx, household.ID)
	if err != nil {
		return Dashboard{}, err
	}

	return Dashboard{
		Household:      household,
		CurrentUser:    user,
		Members:        members,
		Projects:       projects,
		Tasks:          tasks,
		Events:         events,
		Routines:       routines,
		Notices:        notices,
		TileOrder:      tileOrder,
		AvailableTiles: DashboardTiles(),
		BudgetAppURL:   budgetURL,
	}, nil
}

func defaultHouseholdName(name string) string {
	first := strings.TrimSpace(name)
	if first == "" {
		return "My Household"
	}
	return first + "'s Household"
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
