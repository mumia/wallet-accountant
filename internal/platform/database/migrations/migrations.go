// Package migrations holds the database schema as forward-only golang-migrate
// migrations, and runs them as wa_owner (ADR-0014). Every bounded context's
// tables are added here, in one ordered sequence.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed *.sql
var files embed.FS

// Role names (ADR-0014).
const (
	RoleOwner             = "wa_owner"
	RoleApp               = "wa_app"
	RoleEventStoreReader  = "wa_event_store_reader"
	loginRolesDescription = RoleApp + " or " + RoleEventStoreReader
)

// ErrNotALoginRole is returned for a login URL whose user is not a role the
// migrations create.
var ErrNotALoginRole = errors.New("login URL user must be " + loginRolesDescription)

// Up applies every pending migration through ownerURL, which must connect as
// wa_owner. Then, for each of loginURLs (the connection strings of wa_app and
// wa_event_store_reader), it sets that role's password to the one in the URL,
// so that each connection string is the only place its password is written.
func Up(ctx context.Context, ownerURL string, loginURLs ...string) error {
	logins, err := parseLogins(loginURLs)
	if err != nil {
		return err
	}

	cfg, err := pgx.ParseConfig(ownerURL)
	if err != nil {
		return fmt.Errorf("migrations: owner URL: %w", err)
	}

	db := stdlib.OpenDB(*cfg)
	defer db.Close()

	if err := migrateUp(db); err != nil {
		return err
	}

	for _, l := range logins {
		// ALTER ROLE takes no parameters; the password is quoted as a literal.
		q := fmt.Sprintf("ALTER ROLE %s PASSWORD %s", l.role, quoteLiteral(l.password))
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("migrations: set the password of %s: %w", l.role, err)
		}
	}

	return nil
}

func migrateUp(db *sql.DB) error {
	m, err := newMigrate(db)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrations: up: %w", err)
	}

	return nil
}

func newMigrate(db *sql.DB) (*migrate.Migrate, error) {
	src, err := iofs.New(files, ".")
	if err != nil {
		return nil, fmt.Errorf("migrations: source: %w", err)
	}

	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		return nil, fmt.Errorf("migrations: driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	return m, nil
}

type login struct{ role, password string }

func parseLogins(urls []string) ([]login, error) {
	var logins []login

	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("migrations: login URL: %w", err)
		}

		if u.User == nil {
			continue
		}

		role := u.User.Username()
		if !slices.Contains([]string{RoleApp, RoleEventStoreReader}, role) {
			return nil, fmt.Errorf("%w, not %q", ErrNotALoginRole, role)
		}

		if password, ok := u.User.Password(); ok {
			logins = append(logins, login{role: role, password: password})
		}
	}

	return logins, nil
}

func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
