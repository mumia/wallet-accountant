package httpapi_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

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
