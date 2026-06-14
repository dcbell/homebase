package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

const apiTokenPrefix = "hb_"

func (s *Store) ListAPITokens(ctx context.Context, userID, householdID int64) ([]APIToken, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, household_id, name, token_prefix, scope, last_used_at, revoked_at, created_at
		FROM api_tokens
		WHERE user_id = $1 AND household_id = $2
		ORDER BY revoked_at NULLS FIRST, created_at DESC
	`, userID, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := []APIToken{}
	for rows.Next() {
		token, err := scanAPIToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

func (s *Store) CreateAPIToken(ctx context.Context, userID, householdID int64, input APITokenInput) (APITokenWithSecret, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return APITokenWithSecret{}, errors.New("name is required")
	}
	input.Scope = strings.ToLower(strings.TrimSpace(input.Scope))
	if input.Scope == "" {
		input.Scope = "read"
	}
	if input.Scope != "read" && input.Scope != "write" {
		return APITokenWithSecret{}, errors.New("scope must be read or write")
	}

	secret, err := generateAPITokenSecret()
	if err != nil {
		return APITokenWithSecret{}, err
	}
	hash := hashAPIToken(secret)
	prefix := tokenDisplayPrefix(secret)

	var token APIToken
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO api_tokens (user_id, household_id, name, token_hash, token_prefix, scope)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, household_id, name, token_prefix, scope, last_used_at, revoked_at, created_at
	`, userID, householdID, input.Name, hash, prefix, input.Scope).Scan(
		&token.ID,
		&token.UserID,
		&token.HouseholdID,
		&token.Name,
		&token.Prefix,
		&token.Scope,
		&token.LastUsedAt,
		&token.RevokedAt,
		&token.CreatedAt,
	)
	if err != nil {
		return APITokenWithSecret{}, err
	}

	return APITokenWithSecret{APIToken: token, Token: secret}, nil
}

func (s *Store) RevokeAPIToken(ctx context.Context, userID, householdID, tokenID int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE api_tokens
		SET revoked_at = now()
		WHERE id = $1 AND user_id = $2 AND household_id = $3 AND revoked_at IS NULL
	`, tokenID, userID, householdID)
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

func (s *Store) SessionContextByAPIToken(ctx context.Context, rawToken string) (User, Household, APIToken, error) {
	hash := hashAPIToken(rawToken)

	var user User
	var household Household
	var token APIToken
	err := s.db.QueryRowContext(ctx, `
		UPDATE api_tokens
		SET last_used_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
		RETURNING id, user_id, household_id, name, token_prefix, scope, last_used_at, revoked_at, created_at
	`, hash).Scan(
		&token.ID,
		&token.UserID,
		&token.HouseholdID,
		&token.Name,
		&token.Prefix,
		&token.Scope,
		&token.LastUsedAt,
		&token.RevokedAt,
		&token.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Household{}, APIToken{}, ErrNotFound
	}
	if err != nil {
		return User{}, Household{}, APIToken{}, err
	}

	err = s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.name, u.avatar_url, u.created_at,
		       h.id, h.name, hm.role, h.created_at
		FROM users u
		JOIN household_members hm ON hm.user_id = u.id AND hm.household_id = $2
		JOIN households h ON h.id = hm.household_id
		WHERE u.id = $1
	`, token.UserID, token.HouseholdID).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt,
		&household.ID, &household.Name, &household.Role, &household.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Household{}, APIToken{}, ErrNotFound
	}
	if err != nil {
		return User{}, Household{}, APIToken{}, err
	}

	return user, household, token, nil
}

func scanAPIToken(scanner interface {
	Scan(dest ...any) error
}) (APIToken, error) {
	var token APIToken
	err := scanner.Scan(
		&token.ID,
		&token.UserID,
		&token.HouseholdID,
		&token.Name,
		&token.Prefix,
		&token.Scope,
		&token.LastUsedAt,
		&token.RevokedAt,
		&token.CreatedAt,
	)
	return token, err
}

func generateAPITokenSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return apiTokenPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func tokenDisplayPrefix(token string) string {
	token = strings.TrimSpace(token)
	if len(token) <= 14 {
		return token
	}
	return token[:14]
}
