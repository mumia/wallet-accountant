# 10: OpenTofu from bare Proxmox to a Flux-reconciling cluster

**What to build:** `infra/bootstrap` and `infra/homelab` in the env repo. `tofu apply` in each takes the existing Proxmox host to a Garage LXC on the second RAID array and a k3s VM on the tailnet, with `*.shire.migmar.eu` pointing at it and Flux bootstrapped from the env repo. Spec: `../spec.md` ("Production", user story 1).

**Blocked by:** 03

**Status:** claimed

- [ ] Runbook (written first), `infra/README.md`. Every manual step is listed in order, with the exact `op`/`garage` commands:
  - **1Password vault and service account**: create a **dedicated vault** (e.g. `homelab-infra`) that holds only the infra secrets. Create a **service account** with **read-only** access to **that vault only**; check that service accounts are available on the user's plan. OpenTofu's 1Password provider reads through it and never writes. Its token goes in the user's personal vault, not the dedicated one, and is passed as `OP_SERVICE_ACCOUNT_TOKEN` in the shell that runs `tofu`
  - **Credentials into the dedicated vault**, each by hand: the Proxmox API user and token with least privilege; the OVH API keys limited to the `migmar.eu` zone; the Tailscale API key; the SOPS age private key (plus the printed copy); the OpenTofu state encryption passphrases
  - **Bootstrap state**: after each `tofu apply` in `infra/bootstrap`, the user uploads the encrypted state file to the dedicated vault as a document (`op document edit`, with their own account, because the service account is read-only). Before every `plan`/`apply` there, they download it (`op document get`). The runbook says to never apply from a stale copy. The state file is gitignored
  - **Garage keys**: after the bootstrap apply, the user creates buckets and keys on the Garage LXC with the `garage` CLI: `tofu-state`, `cnpg-zitadel-db`, `cnpg-apps-db`, `pg-dumps`, one key per consumer with access to its bucket only. Each key ID and secret goes into the dedicated vault as its own item, and the homelab stack and the SOPS-encrypted prod Secrets are filled from there. The Garage admin token and RPC secret go in the vault too. The runbook also covers rotating a key: create the new one, update the vault and the SOPS Secret, delete the old one
- [ ] `infra/bootstrap`: the Garage LXC on the second array's storage pool, with Garage installed and configured (one zone for now, layout applied). Buckets and keys are **not** made by OpenTofu; they come from the runbook step above. Local state uses OpenTofu state encryption, and the runbook keeps it in 1Password
- [ ] `infra/homelab`: S3 backend on Garage with `use_lockfile`, and state encryption
- [ ] The k3s VM through `bpg/proxmox`: Debian cloud image downloaded by the provider, 4 cores, 16 GB, disks on the chosen pool, a node-count variable; cloud-init installs k3s (Traefik with Gateway API, pinned version) and Tailscale with a key from the Tailscale provider
- [ ] OVH provider: wildcard A record `*.shire.migmar.eu` to the VM's Tailscale IP
- [ ] Flux provider: `flux_bootstrap_git` against `clusters/prod` with a deploy key, and the SOPS age key Secret, read from 1Password
- [ ] Existing host settings (pools, bridges) and existing VMs are only referenced, never imported
- [ ] Destroy-and-recreate of the VM tested once: everything comes back through Flux

## Comments

- 2026-11-18 (orchestrator): the user asked for this ticket to be written as unapplied code in this PR. The agent writes and validates the code (`tofu validate`, `flux build`/`kustomize build`, dry runs) but applies nothing and uses no real credentials. Applying it, first runs and the destroy-and-recreate check stay with the user; after merge the ticket returns to `ready-for-human` for those steps.
