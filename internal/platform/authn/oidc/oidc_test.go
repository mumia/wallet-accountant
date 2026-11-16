package oidc_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/mumia/wallet-accountant/internal/platform/authn"
	"github.com/mumia/wallet-accountant/internal/platform/authn/oidc"
)

const audience = "wallet-accountant"

// issuer is a minimal OpenID Connect provider: discovery and a JWKS.
type issuer struct {
	url string
	key *ecdsa.PrivateKey
}

func newIssuer(t *testing.T) *issuer {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	iss := &issuer{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                iss.url,
			"jwks_uri":                              iss.url + "/keys",
			"authorization_endpoint":                iss.url + "/authorize",
			"token_endpoint":                        iss.url + "/token",
			"id_token_signing_alg_values_supported": []string{"ES256"},
		})
	})
	mux.HandleFunc("GET /keys", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, KeyID: "k1", Algorithm: "ES256", Use: "sig"},
		}})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	iss.url = srv.URL

	return iss
}

func (i *issuer) token(t *testing.T, claims jwt.Claims) string {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: jose.JSONWebKey{Key: i.key, KeyID: "k1"}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatal(err)
	}

	return raw
}

func (i *issuer) validClaims() jwt.Claims {
	now := time.Now()

	return jwt.Claims{
		Issuer:   i.url,
		Subject:  "user-123",
		Audience: jwt.Audience{audience},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(time.Hour)),
	}
}

func TestAuthenticateReturnsIssuerAndSubjectOfAValidToken(t *testing.T) {
	t.Parallel()
	iss := newIssuer(t)
	auth := oidc.New(iss.url, audience)

	id, err := auth.Authenticate(t.Context(), iss.token(t, iss.validClaims()))
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	want := authn.Identity{Issuer: iss.url, Subject: "user-123"}
	if id != want {
		t.Fatalf("identity = %+v, want %+v", id, want)
	}
}

func TestAuthenticateRejectsInvalidTokens(t *testing.T) {
	t.Parallel()
	iss := newIssuer(t)
	other := newIssuer(t)

	expired := iss.validClaims()
	expired.Expiry = jwt.NewNumericDate(time.Now().Add(-time.Minute))

	wrongAudience := iss.validClaims()
	wrongAudience.Audience = jwt.Audience{"someone-else"}

	signedByAnotherKey := iss.validClaims()

	cases := map[string]string{
		"expired":               iss.token(t, expired),
		"wrong audience":        iss.token(t, wrongAudience),
		"signed by another key": other.token(t, signedByAnotherKey),
		"not a JWT":             "garbage",
	}

	auth := oidc.New(iss.url, audience)

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := auth.Authenticate(t.Context(), token)
			if !errors.Is(err, authn.ErrInvalidToken) {
				t.Fatalf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}
