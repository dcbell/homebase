package storetest_test

import (
	"context"
	"testing"

	"homebase/internal/store"
	"homebase/internal/store/storetest"
)

func TestOpenReturnsMigratedStore(t *testing.T) {
	t.Parallel()
	st := storetest.Open(t)

	_, household, err := st.UpsertUserWithHousehold(context.Background(), "sub-owner", "owner@example.test", "Owner", "")
	if err != nil {
		t.Fatalf("UpsertUserWithHousehold() error = %v", err)
	}
	if household.Role != "owner" {
		t.Fatalf("household role = %q, want owner", household.Role)
	}
}

// The server runs every migration on each start, so each must be safe to run
// again on a schema that is already up to date.
func TestMigrationsCanRunAgain(t *testing.T) {
	t.Parallel()
	st := storetest.Open(t)

	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
}

func TestOpenIsolatesEachStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	first := storetest.Open(t)
	second := storetest.Open(t)

	_, household, err := first.UpsertUserWithHousehold(ctx, "sub-owner", "owner@example.test", "Owner", "")
	if err != nil {
		t.Fatalf("UpsertUserWithHousehold() error = %v", err)
	}
	if _, err := first.AddMember(ctx, household.ID, store.MemberInput{Email: "member@example.test", Name: "Member", Role: "member"}); err != nil {
		t.Fatalf("AddMember() error = %v", err)
	}

	members, err := second.ListMembers(ctx, household.ID)
	if err != nil {
		t.Fatalf("ListMembers() error = %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("second store sees %d members of the first store's household, want 0", len(members))
	}
}

func TestOpenSkipsWhenDatabaseUnreachable(t *testing.T) {
	t.Setenv("TEST_DATABASE_URL", "postgres://nobody:nothing@127.0.0.1:1/homebase?sslmode=disable&connect_timeout=2")

	var skipped bool
	t.Run("open", func(t *testing.T) {
		defer func() { skipped = t.Skipped() }()
		storetest.Open(t)
	})
	if !skipped {
		t.Fatal("Open() did not skip when the test database is unreachable")
	}
}
