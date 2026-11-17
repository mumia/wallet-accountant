// Package oidc is the OpenID Connect adapter of the Authenticator port
// (ADR-0010). It verifies bearer JWTs against the provider's JWKS, found
// through OpenID Connect discovery, and returns the token's issuer and subject.
package oidc

import (
	"context"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/mumia/wallet-accountant/internal/platform/authn"
)

// Authenticator verifies tokens from one issuer for one audience. Discovery
// runs on first use and is retried until it succeeds, so the API can start
// before the provider is reachable.
type Authenticator struct {
	issuer   string
	audience string

	mu       sync.Mutex
	verifier *oidc.IDTokenVerifier
}

var _ authn.Authenticator = (*Authenticator)(nil)

// New returns an Authenticator for tokens issued by issuer for audience.
func New(issuer, audience string) *Authenticator {
	return &Authenticator{issuer: issuer, audience: audience}
}

// Authenticate implements authn.Authenticator. Any token that does not verify
// gives an error wrapping authn.ErrInvalidToken.
func (a *Authenticator) Authenticate(ctx context.Context, token string) (authn.Identity, error) {
	verifier, err := a.load(ctx)
	if err != nil {
		return authn.Identity{}, err
	}

	t, err := verifier.Verify(ctx, token)
	if err != nil {
		return authn.Identity{}, fmt.Errorf("%w: %w", authn.ErrInvalidToken, err)
	}

	return authn.Identity{Issuer: t.Issuer, Subject: t.Subject}, nil
}

func (a *Authenticator) load(ctx context.Context) (*oidc.IDTokenVerifier, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.verifier != nil {
		return a.verifier, nil
	}

	// The provider keeps the context for fetching keys later, so it must
	// outlive this request.
	provider, err := oidc.NewProvider(context.WithoutCancel(ctx), a.issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery for %s: %w", a.issuer, err)
	}

	a.verifier = provider.Verifier(&oidc.Config{ClientID: a.audience})

	return a.verifier, nil
}
