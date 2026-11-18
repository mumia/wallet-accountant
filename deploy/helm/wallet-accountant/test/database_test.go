//go:build chart

package charttest

import (
	"encoding/base64"
	"net/url"
	"strconv"
	"testing"
)

func hookOf(o Object) (hook string, weight int) {
	a := o.Map("metadata.annotations")
	hook, _ = a["helm.sh/hook"].(string)
	w, _ := a["helm.sh/hook-weight"].(string)
	weight, _ = strconv.Atoi(w)
	return hook, weight
}

func secretValue(t *testing.T, s Object, key string) string {
	t.Helper()
	if v := s.Str("stringData." + key); v != "" {
		return v
	}
	raw, err := base64.StdEncoding.DecodeString(s.Str("data." + key))
	if err != nil || len(raw) == 0 {
		t.Fatalf("Secret %s has no %s", s.Str("metadata.name"), key)
	}
	return string(raw)
}

func TestExternalDatabaseCreatesNoDatabase(t *testing.T) {
	t.Parallel()
	ms := render(t, values("base", "database-external", "oidc-from-values"))
	if len(ms.OfKind("Cluster")) != 0 || len(ms.OfKind("Secret")) != 0 {
		t.Errorf("external mode rendered a Cluster or a Secret: %v", ms.names())
	}
}

func TestManagedDatabaseIsACloudNativePGClusterWithTheOwnerRole(t *testing.T) {
	t.Parallel()
	ms := render(t, values("base", "database-managed", "oidc-from-values"))

	cluster := ms.Get(t, "Cluster", "wa-wallet-accountant-db")
	if got := cluster.Str("apiVersion"); got != "postgresql.cnpg.io/v1" {
		t.Errorf("apiVersion = %q", got)
	}
	if got := cluster.At("spec.instances"); got != 1 {
		t.Errorf("instances = %v, want 1", got)
	}
	if got := cluster.Str("spec.bootstrap.initdb.database"); got != "wallet_accountant" {
		t.Errorf("initdb database = %q", got)
	}
	if got := cluster.Str("spec.bootstrap.initdb.owner"); got != "wa_owner" {
		t.Errorf("initdb owner = %q", got)
	}

	// The owner role postgres-init.sh created: LOGIN CREATEROLE BYPASSRLS (ADR-0014).
	var owner Object
	for _, r := range cluster.List("spec.managed.roles") {
		if asObject(r).Str("name") == "wa_owner" {
			owner = asObject(r)
		}
	}
	if owner == nil {
		t.Fatalf("no managed role wa_owner in %v", cluster.List("spec.managed.roles"))
	}
	for _, attr := range []string{"login", "createrole", "bypassrls"} {
		if owner.At(attr) != true {
			t.Errorf("wa_owner %s = %v, want true", attr, owner.At(attr))
		}
	}
	ownerSecret := ms.Get(t, "Secret", owner.Str("passwordSecret.name"))
	if got := ownerSecret.Str("type"); got != "kubernetes.io/basic-auth" {
		t.Errorf("owner Secret type = %q", got)
	}
	if got := secretValue(t, ownerSecret, "username"); got != "wa_owner" {
		t.Errorf("owner Secret username = %q", got)
	}

	// The URLs every workload reads, pointing at the cluster's read-write Service.
	urls := ms.Get(t, "Secret", "wa-wallet-accountant-db-urls")
	for key, user := range map[string]string{"owner-url": "wa_owner", "app-url": "wa_app", "reader-url": "wa_event_store_reader"} {
		u, err := url.Parse(secretValue(t, urls, key))
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if u.User.Username() != user || u.Host != "wa-wallet-accountant-db-rw.wallet.svc:5432" || u.Path != "/wallet_accountant" {
			t.Errorf("%s = %s, want %s@wa-wallet-accountant-db-rw.wallet.svc:5432/wallet_accountant", key, u.Redacted(), user)
		}
		if pw, _ := u.User.Password(); len(pw) < 24 {
			t.Errorf("%s has a password of %d characters, want at least 24", key, len(pw))
		}
	}
	ownerURL, _ := url.Parse(secretValue(t, urls, "owner-url"))
	if pw, _ := ownerURL.User.Password(); pw != secretValue(t, ownerSecret, "password") {
		t.Error("the owner URL's password differs from the owner role's")
	}

	// They exist before migrate runs on the first install.
	migrate := ms.Get(t, "Job", "wa-wallet-accountant-migrate")
	_, migrateWeight := hookOf(migrate)
	for _, o := range []Object{ownerSecret, urls, cluster} {
		hook, weight := hookOf(o)
		if hook != "pre-install" || weight >= migrateWeight {
			t.Errorf("%s %s: hook %q weight %d, want a pre-install hook before migrate (%d)",
				o.Str("kind"), o.Str("metadata.name"), hook, weight, migrateWeight)
		}
	}
	e := env(containers(migrate)[0])
	if got := e["WA_OWNER_DATABASE_URL"].Str("valueFrom.secretKeyRef.name"); got != "wa-wallet-accountant-db-urls" {
		t.Errorf("migrate reads the owner URL from %q", got)
	}
}
