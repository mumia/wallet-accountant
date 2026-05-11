# INV-001: Secrets never appear in tracked files

**Date:** 2026-05-01
**Status:** Active

## Statement

Secrets do not appear as values in any file tracked by version control.

## Rationale

Once a secret reaches a tracked file, every clone of the repository — past, present, future — holds a copy. Removing it "later" doesn't remove it from history. Detection-after-commit means rotation, not remediation. The only safe state is that the secret was never written to disk in tracked form. This invariant exists because git history is permanent, mirrors and forks multiply the exposure, and "we'll fix it tomorrow" doesn't work for credentials.

## Consequences of violation

- Permanent exposure window: every clone, fork, and mirror retains the secret in history forever — the leak cannot be undone by any subsequent commit.
- Mandatory rotation across every environment that uses the leaked credential, plus every dependent system that authenticates with it.
- Compliance breach (SOC 2, ISO 27001, GDPR, PCI DSS, depending on the credential type) — auditable findings, possible breach-notification obligations.
- Supply-chain risk if the repository is mirrored, made public, or scraped by automated credential harvesters.
- Erosion of trust in the project's hygiene — every future credential decision becomes a "did we already leak this?" investigation.

## Implementation

Secrets enter the running application via environment variables (resolved through Spring's property placeholder syntax `${VAR_NAME}`) or via a dedicated secrets backend. Tracked YAML files contain placeholders only. Local development reads from an untracked `.env` (excluded by `.gitignore`) or a developer-local secrets manager. CI environments and deployments inject secrets via the platform's secret-store integration. The spring-boot guideline rule on secrets captures the framework-specific mechanism.

## Anti-patterns

- A hardcoded JWT signing key inside `application.yml` "for now" — the comment never gets removed and the value ships to production.
- Real-looking dummy credentials in test fixtures (e.g., `test_password: "P@ssw0rd123!"`) — secret scanners cannot distinguish them from real credentials, and worse, they may *be* real credentials someone reused.
- A `.env.example` that contains real values rather than `CHANGE_ME` placeholders.
- Secrets stored in `application-dev.yml` "because dev is internal" — the *repository* is the leak surface, not the runtime environment.
- Committing `kubeconfig`, `aws-credentials`, `service-account.json`, or `*.pem` files "to share with the team" — version control is not a credential distribution mechanism.

## Enforcement

- **Automated (pre-commit)**: a hook runs `gitleaks` / `trufflehog` / `detect-secrets` against the staged diff and blocks the commit on any detection.
- **Automated (CI)**: the pipeline runs the same scanner on every PR plus a periodic full-history scan; failing scans block merge and trigger an alert.
- **Automated (gitignore audit)**: a structural check verifies that `.env`, `*.pem`, `*-key.json`, and other obvious secret-bearing patterns are excluded by `.gitignore` at the repository root.
- **Automated (runtime)**: services FAIL fast on startup when a required secret-bearing property still holds its placeholder default — surfacing missed configuration before deploy rather than after.
- **Manual**: PR reviewers reject any diff that adds a non-placeholder value to `application*.yml`, `bootstrap*.yml`, property files, fixtures, or any other tracked file that could plausibly hold a credential.

<!-- Directives for edikt governance. Populated by /edikt:invariant:compile. -->
[edikt:directives:start]: #
source_hash: 06ffc1344e403d5bfe066b69883e512792a0f5f107a383f0e9973bcb0a592fb4
directives_hash: c9357d2293bf7bf1c091d38ff5f40e55532f418b71701aa21947d438363c6a82
compiler_version: "0.4.3"
paths:
  - "**/*"
scope:
  - planning
  - design
  - review
  - implementation
directives:
  - "Secrets — passwords, API keys, OAuth client secrets, JWT signing keys, TLS private keys, tokens — MUST NEVER appear as values in any tracked file. NEVER commit a real secret value to `application.yml`, profile-specific YAML, source code, fixtures, `.env.example`, configuration scripts, or any other file under version control. Inject secrets through environment variables resolved at runtime or through a dedicated secrets backend. (ref: INV-001)"
reminders:
  - "Before committing → scan the diff for any value that looks like a credential (password, key, token); if in doubt, replace it with a `${VAR_NAME}` placeholder and inject at runtime (ref: INV-001)"
verification:
  - "[ ] No real-looking secret values in `application*.yml`, `bootstrap*.yml`, fixtures, or `.env.example` — only placeholders or env-var references (ref: INV-001)"
  - "[ ] Pre-commit and CI secret scanners (`gitleaks` / `trufflehog` / `detect-secrets`) configured, running, and passing (ref: INV-001)"
  - "[ ] `.gitignore` excludes `.env`, `*.pem`, `*-key.json`, and other obvious secret-bearing patterns (ref: INV-001)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
