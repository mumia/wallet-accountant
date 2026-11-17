// Package authn is the Authenticator port (ADR-0010): it turns a bearer token
// into the verified identity of whoever holds it. Authentication proves who
// someone is; whether that identity is a User is decided by Households
// (ADR-0017).
package authn

import (
	"context"
	"errors"
)

// ErrInvalidToken is returned for a token that is malformed, expired, signed
// by an unknown key or issued for someone else.
var ErrInvalidToken = errors.New("invalid bearer token")

// Identity is a verified OpenID Connect identity.
type Identity struct {
	Issuer  string
	Subject string
}

// Authenticator verifies a bearer token and returns its identity.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (Identity, error)
}

type identityKey struct{}

// WithIdentity returns a context carrying the caller's verified identity.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// IdentityFrom returns the caller's verified identity, if the request was
// authenticated.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)

	return id, ok
}
