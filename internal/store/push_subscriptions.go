package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// PushSubscription is one browser or installed app of a Member that receives
// Push. The keys are only used to encrypt Push payloads and are never returned
// by the API.
type PushSubscription struct {
	ID          int64      `json:"id"`
	HouseholdID int64      `json:"household_id"`
	UserID      int64      `json:"user_id"`
	Name        string     `json:"name"`
	Endpoint    string     `json:"endpoint"`
	P256dh      string     `json:"-"`
	Auth        string     `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

// PushSubscriptionInput is what a browser reports from
// PushSubscription.toJSON(), plus a display name.
type PushSubscriptionInput struct {
	Endpoint string
	P256dh   string
	Auth     string
	Name     string
}

// VAPIDKeys is the key pair that signs every Push, as base64url strings.
type VAPIDKeys struct {
	PublicKey  string
	PrivateKey string
}

const pushSubscriptionColumns = `id, household_id, user_id, name, endpoint, p256dh, auth, created_at, last_used_at`

// SavePushSubscription registers a Push Subscription for a Member. Registering
// the same endpoint again updates it in place (moving it to this Member if the
// device changed hands) and reports created=false.
func (s *Store) SavePushSubscription(ctx context.Context, householdID, userID int64, input PushSubscriptionInput, at time.Time) (PushSubscription, bool, error) {
	var sub PushSubscription
	var created bool
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO push_subscriptions (household_id, user_id, endpoint, p256dh, auth, name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (endpoint) DO UPDATE
		SET household_id = EXCLUDED.household_id,
		    user_id = EXCLUDED.user_id,
		    p256dh = EXCLUDED.p256dh,
		    auth = EXCLUDED.auth,
		    name = EXCLUDED.name
		RETURNING `+pushSubscriptionColumns+`, (xmax = 0)
	`, householdID, userID, input.Endpoint, input.P256dh, input.Auth, input.Name, at).Scan(
		&sub.ID, &sub.HouseholdID, &sub.UserID, &sub.Name, &sub.Endpoint, &sub.P256dh, &sub.Auth, &sub.CreatedAt, &sub.LastUsedAt, &created)
	return sub, created, err
}

// ListPushSubscriptions returns a Member's Push Subscriptions, oldest first.
func (s *Store) ListPushSubscriptions(ctx context.Context, householdID, userID int64) ([]PushSubscription, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+pushSubscriptionColumns+`
		FROM push_subscriptions
		WHERE household_id = $1 AND user_id = $2
		ORDER BY id
	`, householdID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subs := []PushSubscription{}
	for rows.Next() {
		var sub PushSubscription
		if err := rows.Scan(&sub.ID, &sub.HouseholdID, &sub.UserID, &sub.Name, &sub.Endpoint, &sub.P256dh, &sub.Auth, &sub.CreatedAt, &sub.LastUsedAt); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// DeletePushSubscription removes one of a Member's Push Subscriptions.
func (s *Store) DeletePushSubscription(ctx context.Context, householdID, userID, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM push_subscriptions WHERE household_id = $1 AND user_id = $2 AND id = $3
	`, householdID, userID, id)
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

// ForgetPushSubscription removes a Push Subscription that its push service
// reported as gone.
func (s *Store) ForgetPushSubscription(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE id = $1`, id)
	return err
}

// TouchPushSubscription records that a Push was accepted for a subscription.
func (s *Store) TouchPushSubscription(ctx context.Context, id int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE push_subscriptions SET last_used_at = $2 WHERE id = $1`, id, at)
	return err
}

// VAPIDKeys returns the stored VAPID key pair, or ErrNotFound before one has
// been generated.
func (s *Store) VAPIDKeys(ctx context.Context) (VAPIDKeys, error) {
	var keys VAPIDKeys
	err := s.db.QueryRowContext(ctx, `SELECT public_key, private_key FROM vapid_keys WHERE id = 1`).Scan(&keys.PublicKey, &keys.PrivateKey)
	if errors.Is(err, sql.ErrNoRows) {
		return VAPIDKeys{}, ErrNotFound
	}
	return keys, err
}

// SaveVAPIDKeysIfAbsent stores keys unless a pair already exists, and returns
// whichever pair is stored. It is safe for API instances starting at once.
func (s *Store) SaveVAPIDKeysIfAbsent(ctx context.Context, keys VAPIDKeys) (VAPIDKeys, error) {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO vapid_keys (id, public_key, private_key) VALUES (1, $1, $2)
		ON CONFLICT (id) DO NOTHING
	`, keys.PublicKey, keys.PrivateKey); err != nil {
		return VAPIDKeys{}, err
	}
	return s.VAPIDKeys(ctx)
}
