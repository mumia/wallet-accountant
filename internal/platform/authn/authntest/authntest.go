// Package authntest is the test implementation of the Authenticator port. It
// hands out opaque tokens, each mapped to an OIDC identity, so tests can act
// as any identity without an OpenID Connect provider.
package authntest

import (
	"context"
	"crypto/rand"
	"sync"

	"github.com/mumia/wallet-accountant/internal/platform/authn"
)

// Authenticator maps tokens it issued to identities. It is safe for
// concurrent use.
type Authenticator struct {
	mu     sync.Mutex
	tokens map[string]authn.Identity
}

// New returns an Authenticator that knows no tokens yet.
func New() *Authenticator {
	return &Authenticator{tokens: map[string]authn.Identity{}}
}

// Token returns a new bearer token for the identity.
func (a *Authenticator) Token(id authn.Identity) string {
	token := "test-" + rand.Text()

	a.mu.Lock()
	defer a.mu.Unlock()
	a.tokens[token] = id

	return token
}

// Authenticate implements authn.Authenticator.
func (a *Authenticator) Authenticate(_ context.Context, token string) (authn.Identity, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	id, ok := a.tokens[token]
	if !ok {
		return authn.Identity{}, authn.ErrInvalidToken
	}

	return id, nil
}
