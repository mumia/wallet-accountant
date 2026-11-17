// Package databasetest gives integration tests a migrated PostgreSQL database
// of their own. One container is started per test binary through
// testcontainers-go; every call to New creates a fresh database in it, runs
// the migrations as wa_owner and returns the connection strings of the three
// roles (ADR-0014), as production uses them.
package databasetest

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/mumia/wallet-accountant/internal/platform/database/migrations"
)

// PostgresImage is the PostgreSQL image the tests run against.
const PostgresImage = "postgres:18.7-alpine"

// URLs are the connection strings of one test database, one per role.
type URLs struct {
	// Owner connects as wa_owner, which owns the schema.
	Owner string
	// App connects as wa_app, under row-level security.
	App string
	// Reader connects as wa_event_store_reader.
	Reader string
}

var (
	startOnce sync.Once
	adminURL  string
	errStart  error

	// Roles are cluster-wide: migrations creating them must not race.
	migrateMu sync.Mutex
	dbCounter atomic.Int64
)

// New creates and migrates a database for the test.
func New(t testing.TB) URLs {
	t.Helper()

	urls := NewEmpty(t)
	Migrate(t, func() error { return migrations.Up(context.Background(), urls.Owner, urls.App, urls.Reader) })

	return urls
}

// NewEmpty creates a database for the test, owned by wa_owner, without
// running the migrations. The roles wa_app and wa_event_store_reader may not
// exist yet; the migrations create them.
func NewEmpty(t testing.TB) URLs {
	t.Helper()

	startOnce.Do(func() { adminURL, errStart = start(context.Background()) })

	if errStart != nil {
		t.Fatalf("postgres container: %v", errStart)
	}

	ctx := context.Background()
	name := fmt.Sprintf("wa_test_%d", dbCounter.Add(1))

	if err := adminExec(ctx, "CREATE DATABASE "+name+" OWNER wa_owner"); err != nil {
		t.Fatalf("create database: %v", err)
	}

	return URLs{
		Owner:  roleURL(name, migrations.RoleOwner, "wa_owner"),
		App:    roleURL(name, migrations.RoleApp, "wa_app"),
		Reader: roleURL(name, migrations.RoleEventStoreReader, "wa_event_store_reader"),
	}
}

// Migrate runs the migrations of a database from NewEmpty through fn (for
// example the migrate run mode), serialised with every other migration of
// the test binary, because the roles they create are cluster-wide.
func Migrate(t testing.TB, fn func() error) {
	t.Helper()

	migrateMu.Lock()
	defer migrateMu.Unlock()

	if err := fn(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

// start runs the container and creates wa_owner, as the database bootstrap
// does in development (deploy/dev/postgres-init.sql).
func start(ctx context.Context) (string, error) {
	c, err := postgres.Run(ctx, PostgresImage,
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return "", fmt.Errorf("run: %w", err)
	}

	admin, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return "", fmt.Errorf("connection string: %w", err)
	}

	adminURL = admin

	if err := adminExec(ctx, "CREATE ROLE wa_owner LOGIN CREATEROLE BYPASSRLS PASSWORD 'wa_owner'"); err != nil {
		return "", err
	}

	return admin, nil
}

func adminExec(ctx context.Context, sql string) error {
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return fmt.Errorf("connect as superuser: %w", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("%s: %w", sql, err)
	}

	return nil
}

func roleURL(database, role, password string) string {
	u, err := url.Parse(adminURL)
	if err != nil {
		panic(err)
	}

	u.User = url.UserPassword(role, password)
	u.Path = "/" + database

	return u.String()
}
