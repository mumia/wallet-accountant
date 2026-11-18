# 07: Grafana stack and single sign-on for the admin tools

**What to build:** the Grafana stack as a platform component, opt-in locally through `make local-up-observability`, with the app's telemetry visible. Grafana and Headlamp sign in through Zitadel, and the break-glass logins still work. Spec: `../spec.md` ("Platform", user stories 5 and 16); ADR-0016, ADR-0022.

**Blocked by:** 04

**Status:** ready-for-agent

- [ ] Prometheus (with its OTLP receiver), Loki, Tempo and Grafana HelmReleases in `platform/observability`, a Flux Kustomization that `clusters/local` includes only when the observability flag is set; `clusters/prod` always includes it
- [ ] Data sources provisioned like `deploy/selfhosted/grafana/provisioning` today, including trace-to-logs links
- [ ] The app chart's collector exports to the stack when it's present and discards otherwise
- [ ] A platform setup Job (same mechanism as the app's) creates Zitadel apps for Grafana and Headlamp
- [ ] Grafana: generic OAuth through Zitadel, plus the local admin kept, its password in a Secret
- [ ] Headlamp: OIDC through Zitadel, plus token login kept
- [ ] A runbook page in the env repo: "Zitadel is down", using kubectl, the Grafana admin and a Headlamp token
