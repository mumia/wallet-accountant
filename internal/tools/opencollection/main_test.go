package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Every operation of the contract becomes a request that signs in through
// the collection: a request without `auth: inherit` would be sent with no
// token at all.
func TestEveryOperationIsARequestThatInheritsSignIn(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	doc, err := parse(raw)
	if err != nil {
		t.Fatal(err)
	}

	files, err := collection(doc)
	if err != nil {
		t.Fatal(err)
	}

	requests := 0

	for name, content := range files {
		if name == "opencollection.yml" || strings.HasPrefix(name, "environments/") {
			continue
		}

		if strings.HasSuffix(name, "/folder.yml") {
			var f folder
			if err := yaml.Unmarshal(content, &f); err != nil {
				t.Fatalf("%s: %v", name, err)
			}

			if f.Request.Auth != "inherit" {
				t.Errorf("%s: auth is %q, not inherit", name, f.Request.Auth)
			}

			continue
		}

		requests++

		var r struct {
			HTTP struct {
				Method  string     `yaml:"method"`
				Auth    string     `yaml:"auth"`
				Headers []keyValue `yaml:"headers"`
			} `yaml:"http"`
		}
		if err := yaml.Unmarshal(content, &r); err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if r.HTTP.Auth != "inherit" {
			t.Errorf("%s: auth is %q, not inherit", name, r.HTTP.Auth)
		}

		if r.HTTP.Method == "POST" && !hasHeader(r.HTTP.Headers, "Idempotency-Key") {
			t.Errorf("%s: a POST without an Idempotency-Key", name)
		}
	}

	if requests != len(doc.operations) {
		t.Errorf("%d requests for %d operations", requests, len(doc.operations))
	}
}

// The contract's server is a path on the origin the API shares with the UI
// (ADR-0021); the collection points at that path on the development app.
func TestBaseURLIsTheContractServerOnTheDevelopmentApp(t *testing.T) {
	t.Parallel()

	doc, err := parse([]byte("openapi: 3.0.3\nservers:\n  - url: /api\npaths: {}\n"))
	if err != nil {
		t.Fatal(err)
	}

	for _, v := range variables(doc) {
		if v.Name == "baseUrl" {
			if v.Value != "http://localhost:8080/api" {
				t.Errorf("baseUrl = %q, want http://localhost:8080/api", v.Value)
			}

			return
		}
	}

	t.Error("no baseUrl variable")
}

// The development app (make run, debug mode) verifies tokens from the local
// cluster's Zitadel, so the collection signs in there.
func TestAuthURLIsTheLocalClustersZitadel(t *testing.T) {
	t.Parallel()

	for _, v := range variables(&document{}) {
		if v.Name == "authUrl" {
			if v.Value != "https://auth.localhost:18443" {
				t.Errorf("authUrl = %q, want https://auth.localhost:18443", v.Value)
			}

			return
		}
	}

	t.Error("no authUrl variable")
}

func hasHeader(headers []keyValue, name string) bool {
	for _, h := range headers {
		if h.Name == name && h.Value != "" {
			return true
		}
	}

	return false
}

// An object part of a multipart body (the mapping test's csv_mapping) is
// written as the JSON text a client sends, the same on every run: never a
// Go value's printed form, which holds pointers.
func TestAnObjectMultipartPartIsJSON(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	doc, err := parse(raw)
	if err != nil {
		t.Fatal(err)
	}

	files, err := collection(doc)
	if err != nil {
		t.Fatal(err)
	}

	var r struct {
		HTTP struct {
			Body multipartBody `yaml:"body"`
		} `yaml:"http"`
	}
	if err := yaml.Unmarshal(files["connections/test-csv-mapping.yml"], &r); err != nil {
		t.Fatal(err)
	}

	for _, p := range r.HTTP.Body.Data {
		if p.Name != "csv_mapping" {
			continue
		}

		var m map[string]any
		if err := json.Unmarshal([]byte(fmt.Sprint(p.Value)), &m); err != nil || m["delimiter"] == nil {
			t.Fatalf("csv_mapping = %v, want a CSV mapping as JSON text (%v)", p.Value, err)
		}

		return
	}

	t.Fatal("no csv_mapping part in connections/test-csv-mapping.yml")
}
