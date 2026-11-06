package caldav

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/emersion/go-webdav/caldav"

	"homebase/internal/store"
)

// uids is how a writable collection names its items. An item made in
// Homebase has a generated UID, prefix + id. An item made on a device keeps
// the device's UID, stored as its calendar_uid, and is reachable only by it.
type uids[T any] struct {
	prefix string
	// get loads an item by id, find by device UID; both return
	// store.ErrNotFound when there is none.
	get  func(ctx context.Context, householdID, id int64) (T, error)
	find func(ctx context.Context, householdID int64, uid string) (T, error)
	// deviceUID is an item's device UID, "" for an item made in Homebase.
	deviceUID func(T) string
}

// generatedUID is the generated UID for id.
func generatedUID(prefix string, id int64) string {
	return fmt.Sprintf("%s%d", prefix, id)
}

// of is an item's UID.
func (u uids[T]) of(id int64, item T) string {
	if uid := u.deviceUID(item); uid != "" {
		return uid
	}
	return generatedUID(u.prefix, id)
}

// id is the id in a generated UID.
func (u uids[T]) id(uid string) (int64, bool) {
	rest, ok := strings.CutPrefix(uid, u.prefix)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || id <= 0 || generatedUID(u.prefix, id) != uid {
		return 0, false
	}
	return id, true
}

// lookup returns the Household's item with this UID, whoever it concerns,
// or store.ErrNotFound.
func (u uids[T]) lookup(ctx context.Context, householdID int64, uid string) (T, error) {
	if id, ok := u.id(uid); ok {
		item, err := u.get(ctx, householdID, id)
		if err == nil && u.deviceUID(item) != "" {
			var none T
			return none, store.ErrNotFound
		}
		return item, err
	}
	return u.find(ctx, householdID, uid)
}

// checkNew refuses to create an item with uid that isn't in the Member's
// view: a generated UID (archived, or not theirs; devices never mint these),
// or a device UID another Member's device already used.
func (u uids[T]) checkNew(ctx context.Context, householdID int64, uid string) error {
	if _, ours := u.id(uid); ours {
		return refuse(caldav.PreconditionNoUIDConflict)
	}
	_, err := u.find(ctx, householdID, uid)
	if err == nil {
		return refuse(caldav.PreconditionNoUIDConflict)
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	return err
}
