# 04: CloudNativePG and Zitadel in the platform

**What to build:** the platform's PostgreSQL and identity provider, running locally. The CNPG operator runs `zitadel-db` and `apps-db`; `apps-db` gets a tenants list. Zitadel runs at `https://auth.localhost` on `zitadel-db`, with an admin login and a machine user whose token app setup Jobs use. Spec: `../spec.md` ("Platform"); ADR-0022, ADR-0023.

**Blocked by:** 03

**Status:** ready-for-agent

- [ ] CNPG operator HelmRelease; `zitadel-db` and `apps-db` `Cluster`s, single instance each, with sizes from cluster values
- [ ] `apps-db` tenants list: per tenant, a `Database`, an owner role (managed role) and its password Secret. Wallet Accountant's owner `wa_owner` has `BYPASSRLS` and `CREATEROLE` (ADR-0014)
- [ ] The tenant's connection Secrets are made available in the app's namespace (encrypted copies in prod, plain in local, as decided)
- [ ] Zitadel HelmRelease on `zitadel-db`, external domain `auth.localhost`, TLS ended at the Gateway, self-registration off, the v1 login UI as today
- [ ] The first-instance human admin, plus a machine user with a personal access token stored in a Secret for setup Jobs
- [ ] Zitadel's issuer URL works both from the browser and from a pod (through the ticket 03 CoreDNS rewrite)
- [ ] The Zitadel masterkey comes from a Secret (throwaway locally); add it to the spec's 1Password list for prod
