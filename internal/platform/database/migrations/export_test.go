package migrations

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// UpTo applies the migrations up to and including version through ownerURL,
// so a test can seed data the way an older schema held it before migrating
// the rest.
func UpTo(ownerURL string, version uint) error {
	cfg, err := pgx.ParseConfig(ownerURL)
	if err != nil {
		return fmt.Errorf("owner URL: %w", err)
	}

	db := stdlib.OpenDB(*cfg)
	defer db.Close()

	m, err := newMigrate(db)
	if err != nil {
		return err
	}

	return m.Migrate(version)
}
