package httpapi_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
)

// The server mounts the API under httpapi.PathPrefix; the contract's only
// server is that path, on the origin the API shares with the UI (ADR-0021).
func TestContractServerIsThePathPrefix(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Servers []struct {
			URL string `yaml:"url"`
		} `yaml:"servers"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}

	if len(doc.Servers) != 1 || doc.Servers[0].URL != "/api" {
		t.Errorf("servers = %+v, want only /api", doc.Servers)
	}

	if httpapi.PathPrefix != "/api" {
		t.Errorf("PathPrefix = %q, want /api", httpapi.PathPrefix)
	}
}

// The middleware honours Idempotency-Key on every POST whether or not the
// contract says so; this keeps the contract saying so for every new POST.
func TestEveryPostDeclaresIdempotencyKey(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `yaml:"operationId"`
			Parameters  []struct {
				Ref string `yaml:"$ref"`
			} `yaml:"parameters"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}

	posts := 0

	for path, ops := range doc.Paths {
		op, ok := ops["post"]
		if !ok {
			continue
		}

		posts++

		declared := false

		for _, p := range op.Parameters {
			if p.Ref == "#/components/parameters/IdempotencyKey" {
				declared = true
			}
		}

		if !declared {
			t.Errorf("POST %s (%s) doesn't declare the IdempotencyKey parameter", path, op.OperationID)
		}
	}

	if posts == 0 {
		t.Fatal("no POST operations found: is the contract's path right?")
	}
}
