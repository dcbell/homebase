package push

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"homebase/internal/store"
)

// EnsureVAPIDKeys returns the stored VAPID key pair, generating and storing
// one first if there is none. Concurrent callers all end up with the same pair.
func EnsureVAPIDKeys(ctx context.Context, st *store.Store) (store.VAPIDKeys, error) {
	keys, err := st.VAPIDKeys(ctx)
	if err == nil {
		return keys, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return store.VAPIDKeys{}, err
	}
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return store.VAPIDKeys{}, fmt.Errorf("generate VAPID keys: %w", err)
	}
	return st.SaveVAPIDKeysIfAbsent(ctx, store.VAPIDKeys{PublicKey: public, PrivateKey: private})
}

// WebPush sends Push with the standard Web Push protocol, signed with the
// VAPID keys stored in the database.
type WebPush struct {
	store   *store.Store
	subject string
	client  *http.Client

	mu   sync.Mutex
	keys *store.VAPIDKeys
}

// NewWebPush returns the production Sender. subject identifies the operator
// to push services: an email address (without "mailto:") or an https URL.
func NewWebPush(st *store.Store, subject string) *WebPush {
	return &WebPush{
		store:   st,
		subject: strings.TrimPrefix(subject, "mailto:"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WebPush) vapidKeys(ctx context.Context) (store.VAPIDKeys, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.keys != nil {
		return *w.keys, nil
	}
	keys, err := EnsureVAPIDKeys(ctx, w.store)
	if err != nil {
		return store.VAPIDKeys{}, err
	}
	w.keys = &keys
	return keys, nil
}

func (w *WebPush) Send(ctx context.Context, sub Subscription, payload []byte) (Result, error) {
	keys, err := w.vapidKeys(ctx)
	if err != nil {
		return Failed, err
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		HTTPClient:      w.client,
		Subscriber:      w.subject,
		VAPIDPublicKey:  keys.PublicKey,
		VAPIDPrivateKey: keys.PrivateKey,
		TTL:             24 * 60 * 60,
		Urgency:         webpush.UrgencyHigh,
	})
	if err != nil {
		return Failed, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return Delivered, nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return Gone, fmt.Errorf("push service returned %s", resp.Status)
	default:
		return Failed, fmt.Errorf("push service returned %s", resp.Status)
	}
}
