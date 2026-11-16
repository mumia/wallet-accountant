// Package httpapi holds the HTTP plumbing every bounded context shares: the
// ogen server generated from api/openapi.yaml (package oas), problem+json
// errors, authentication of bearer tokens and request telemetry.
package httpapi

//go:generate go tool ogen --config ../../../api/ogen.yml --target oas --package oas --clean ../../../api/openapi.yaml
