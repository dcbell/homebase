// Package storetest gives tests a migrated, isolated store.
//
// TEST_DATABASE_URL locates the PostgreSQL server (default: the docker compose
// db on localhost:5432). Whatever database that URL names is only used to
// connect: tests always work in the dedicated DatabaseName database, which is
// created on first use, and each call to Open gets its own fresh schema that is
// dropped when the test ends. Dev data is never touched, and any number of test
// processes can run at once.
package storetest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"homebase/internal/store"
)

// DatabaseName is the database every test schema lives in.
const DatabaseName = "homebase_test"

const defaultServerURL = "postgres://homebase:homebase@localhost:5432/homebase?sslmode=disable"

// migrationLockID serializes migrations across concurrent test processes, so
// migrations that inspect the catalog never see another test's half-built schema.
const migrationLockID = 7_011_011

var ensureDatabase sync.Map // server URL -> *sync.Once-guarded result

type ensureResult struct {
	once sync.Once
	err  error
}

// Open returns a store backed by a fresh, fully migrated schema in the test
// database. It skips the test when the database server is unreachable.
func Open(t testing.TB) *store.Store {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serverURL := os.Getenv("TEST_DATABASE_URL")
	if serverURL == "" {
		serverURL = defaultServerURL
	}
	testURL, err := withDatabase(serverURL, DatabaseName)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}

	if err := createTestDatabase(ctx, serverURL); err != nil {
		t.Skipf("test database unreachable at %s: %v (start it with: docker compose up -d db)", redact(serverURL), err)
	}

	admin, err := sql.Open("pgx", testURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	schema := "test_" + randomHex(t, 8)
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		dropCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(dropCtx, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Errorf("drop test schema %s: %v", schema, err)
		}
	})

	schemaURL, err := withSearchPath(testURL, schema)
	if err != nil {
		t.Fatalf("build schema URL: %v", err)
	}
	st, err := store.Open(ctx, schemaURL)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if err := migrateLocked(ctx, admin, st); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}
	return st
}

func createTestDatabase(ctx context.Context, serverURL string) error {
	value, _ := ensureDatabase.LoadOrStore(serverURL, &ensureResult{})
	result := value.(*ensureResult)
	result.once.Do(func() {
		db, err := sql.Open("pgx", serverURL)
		if err != nil {
			result.err = err
			return
		}
		defer func() { _ = db.Close() }()

		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := db.PingContext(pingCtx); err != nil {
			result.err = err
			return
		}

		_, err = db.ExecContext(ctx, `CREATE DATABASE `+DatabaseName)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "42P04" || pgErr.Code == "23505") {
			// Already exists, or another test process created it concurrently.
			err = nil
		}
		result.err = err
	})
	return result.err
}

func migrateLocked(ctx context.Context, admin *sql.DB, st *store.Store) error {
	conn, err := admin.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationLockID)
	}()

	return st.Migrate(ctx)
}

func withDatabase(rawURL, database string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	u.Path = "/" + database
	return u.String(), nil
}

func withSearchPath(rawURL, schema string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func redact(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "TEST_DATABASE_URL"
	}
	return u.Redacted()
}

func randomHex(t testing.TB, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("random schema name: %v", err)
	}
	return hex.EncodeToString(b)
}
