# 12: Backups to Garage and a restore drill

**What to build:** both CNPG clusters archive WAL and take scheduled base backups to Garage, and a nightly CronJob dumps each `apps-db` database. A written, timed restore drill proves both. Spec: `../spec.md` ("Testing Decisions", user story 7); ADR-0023.

**Blocked by:** 10, 11

**Status:** ready-for-human (the drill is the user's to run and sign off)

- [ ] CNPG barman-cloud plugin configured for Garage (S3 endpoint, path-style, region) for `zitadel-db` and `apps-db`, each under its own prefix; retention 30 days
- [ ] A nightly `pg_dump -Fc` CronJob per `apps-db` database to Garage, keeping 30 days
- [ ] Alert when a backup or dump fails (through Prometheus to ntfy, or Flux events)
- [ ] Drill 1: point-in-time recovery of `apps-db` into a scratch cluster; check the app's data
- [ ] Drill 2: restore only `wallet_accountant` from its dump into a scratch database
- [ ] Runbook: both procedures with their timings, the quarterly reminder, and the caveat that Garage replicates deletions
