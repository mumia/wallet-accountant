# PostgreSQL runs on CloudNativePG: a shared apps cluster, backed up to Garage

PostgreSQL (ADR-0006) runs in Kubernetes under the CloudNativePG operator, which the platform installs. There are two clusters:

- `zitadel-db` holds only Zitadel's database (ADR-0022).
- `apps-db` is shared by the homelab's apps. Wallet Accountant is its first **tenant**.

The platform declares each tenant: its database (a CNPG `Database`) and its **owner role** (`wa_owner` for Wallet Accountant, with `BYPASSRLS` as ADR-0014 requires), plus the owner's password Secret. Everything inside the database stays with the app: the first migration creates `wa_app` and `wa_event_store_reader`, and `migrate` sets their passwords, exactly as today. Adding an app to the homelab is therefore one platform change, and the app's roles and schema never leak into the platform. This replaces `deploy/selfhosted/postgres-init.sh`.

We chose a shared apps cluster over one cluster per app to save memory on a single 16 GB node. We kept Zitadel separate so that a bad app migration or restore can never touch sign-in.

## Backups

- Both clusters keep continuous WAL archiving and scheduled base backups, so either can be restored to any point in time. Retention is about 30 days.
- Backups go to **Garage**, an S3-compatible store that runs outside the cluster on its own RAID array. Remote Garage nodes add off-site copies later.
- Point-in-time recovery restores a **whole** cluster. A nightly per-database `pg_dump` CronJob to Garage therefore lets one app on `apps-db` be restored without rolling back the others.
- Garage replicates deletions too, so it is not versioned storage. CNPG's retention policy alone decides what is kept.
- A restore drill, of both point-in-time recovery and a single database, is part of setting this up and is repeated quarterly.

The chart's `database.mode: managed` (ADR-0020) creates a single-tenant CNPG cluster, for self-hosters without a shared one.
