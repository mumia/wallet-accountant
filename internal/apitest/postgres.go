package apitest

import (
	"context"
	"sync"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// PostgresImage is the PostgreSQL image the tests run against.
const PostgresImage = "postgres:18.7-alpine"

var (
	pgOnce sync.Once
	pgURL  string
	errPg  error
)

// databaseURL starts one PostgreSQL container per test binary and returns
// its connection string. The container is removed by the testcontainers
// reaper when the test binary exits.
func databaseURL(t testing.TB) string {
	t.Helper()

	pgOnce.Do(func() {
		ctx := context.Background()

		var c *postgres.PostgresContainer

		c, errPg = postgres.Run(ctx, PostgresImage,
			postgres.WithDatabase("wallet_accountant"),
			postgres.WithUsername("wa_test"),
			postgres.WithPassword("wa_test"),
			postgres.BasicWaitStrategies(),
		)
		if errPg != nil {
			return
		}

		pgURL, errPg = c.ConnectionString(ctx, "sslmode=disable")
	})

	if errPg != nil {
		t.Fatalf("postgres container: %v", errPg)
	}

	return pgURL
}
