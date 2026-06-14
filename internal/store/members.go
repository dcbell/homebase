package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) ListMembers(ctx context.Context, householdID int64) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.email, u.name, u.avatar_url, hm.role, u.created_at
		FROM household_members hm
		JOIN users u ON u.id = hm.user_id
		WHERE hm.household_id = $1
		ORDER BY CASE WHEN hm.role = 'owner' THEN 0 ELSE 1 END, lower(u.name), lower(u.email)
	`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Role, &user.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, user)
	}
	return members, rows.Err()
}

func (s *Store) AddMember(ctx context.Context, householdID int64, input MemberInput) (User, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	if input.Email == "" {
		return User{}, errors.New("email is required")
	}
	if input.Name == "" {
		input.Name = input.Email
	}
	if input.Role == "" {
		input.Role = "member"
	}
	if input.Role != "owner" && input.Role != "member" {
		return User{}, errors.New("role must be owner or member")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var user User
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (email, name)
		VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET
			name = CASE WHEN users.oauth_subject IS NULL THEN EXCLUDED.name ELSE users.name END,
			updated_at = now()
		RETURNING id, email, name, avatar_url, created_at
	`, input.Email, input.Name).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt)
	if err != nil {
		return User{}, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO household_members (household_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (household_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, householdID, user.ID, input.Role)
	if err != nil {
		return User{}, err
	}

	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) UpdateMember(ctx context.Context, householdID, memberID int64, input MemberInput) (User, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	if input.Email == "" {
		return User{}, errors.New("email is required")
	}
	if input.Name == "" {
		input.Name = input.Email
	}
	if input.Role == "" {
		input.Role = "member"
	}
	if input.Role != "owner" && input.Role != "member" {
		return User{}, errors.New("role must be owner or member")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var currentRole string
	err = tx.QueryRowContext(ctx, `
		SELECT role
		FROM household_members
		WHERE household_id = $1 AND user_id = $2
	`, householdID, memberID).Scan(&currentRole)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}

	if currentRole == "owner" && input.Role != "owner" {
		ok, err := s.hasAnotherOwner(ctx, tx, householdID, memberID)
		if err != nil {
			return User{}, err
		}
		if !ok {
			return User{}, errors.New("household must have at least one owner")
		}
	}

	var user User
	err = tx.QueryRowContext(ctx, `
		UPDATE users
		SET email = $2,
			name = $3,
			updated_at = now()
		WHERE id = $1
		RETURNING id, email, name, avatar_url, created_at
	`, memberID, input.Email, input.Name).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE household_members
		SET role = $3
		WHERE household_id = $1 AND user_id = $2
	`, householdID, memberID, input.Role); err != nil {
		return User{}, err
	}
	user.Role = input.Role

	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) RemoveMember(ctx context.Context, householdID, memberID, actorID int64) error {
	if memberID == actorID {
		return ErrForbidden
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var currentRole string
	err = tx.QueryRowContext(ctx, `
		SELECT role
		FROM household_members
		WHERE household_id = $1 AND user_id = $2
	`, householdID, memberID).Scan(&currentRole)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if currentRole == "owner" {
		ok, err := s.hasAnotherOwner(ctx, tx, householdID, memberID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("household must have at least one owner")
		}
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE household_id = $1 AND user_id = $2
	`, householdID, memberID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM household_members
		WHERE household_id = $1 AND user_id = $2
	`, householdID, memberID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) hasAnotherOwner(ctx context.Context, tx *sql.Tx, householdID, exceptUserID int64) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM household_members
			WHERE household_id = $1
				AND user_id <> $2
				AND role = 'owner'
		)
	`, householdID, exceptUserID).Scan(&exists)
	return exists, err
}

func (s *Store) EnsureBootstrapOwner(ctx context.Context, email, name, householdName string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	householdName = strings.TrimSpace(householdName)
	if email == "" {
		return nil
	}
	if name == "" {
		name = email
	}
	if householdName == "" {
		householdName = "My Household"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var userID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO users (email, name)
		VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET updated_at = now()
		RETURNING id
	`, email, name).Scan(&userID); err != nil {
		return err
	}

	var householdID int64
	err = tx.QueryRowContext(ctx, `
		SELECT h.id
		FROM household_members hm
		JOIN households h ON h.id = hm.household_id
		WHERE hm.user_id = $1
		ORDER BY h.created_at
		LIMIT 1
	`, userID).Scan(&householdID)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO households (name)
			VALUES ($1)
			RETURNING id
		`, householdName).Scan(&householdID); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO household_members (household_id, user_id, role)
		VALUES ($1, $2, 'owner')
		ON CONFLICT (household_id, user_id) DO UPDATE SET role = 'owner'
	`, householdID, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) ActivatePreauthorizedOAuthUser(ctx context.Context, providerSubject, email, name, avatarURL string) (User, Household, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return User{}, Household{}, errors.New("email is required")
	}
	if strings.TrimSpace(name) == "" {
		name = email
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, Household{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var userID int64
	var household Household
	err = tx.QueryRowContext(ctx, `
		SELECT u.id, h.id, h.name, hm.role, h.created_at
		FROM users u
		JOIN household_members hm ON hm.user_id = u.id
		JOIN households h ON h.id = hm.household_id
		WHERE lower(u.email) = lower($1)
		ORDER BY h.created_at
		LIMIT 1
	`, email).Scan(&userID, &household.ID, &household.Name, &household.Role, &household.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Household{}, ErrNotFound
	}
	if err != nil {
		return User{}, Household{}, err
	}

	var user User
	err = tx.QueryRowContext(ctx, `
		UPDATE users
		SET oauth_subject = $2,
			name = $3,
			avatar_url = $4,
			updated_at = now()
		WHERE id = $1
		RETURNING id, email, name, avatar_url, created_at
	`, userID, providerSubject, name, avatarURL).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt)
	if err != nil {
		return User{}, Household{}, err
	}

	if err := tx.Commit(); err != nil {
		return User{}, Household{}, err
	}
	return user, household, nil
}

func (s *Store) userInHousehold(ctx context.Context, householdID, userID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM household_members
			WHERE household_id = $1 AND user_id = $2
		)
	`, householdID, userID).Scan(&exists)
	return exists, err
}
