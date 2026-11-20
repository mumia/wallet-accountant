# The stand-in UI (spec "Testing Decisions"): a test fixture that meets the UI
# contract (docs/architecture/ui-contract.md) with stock nginx and one page,
# so the chart's UI slot, the Zitadel `web` app and same-origin routing are
# tested before the real UI exists. `make local-deploy LOCAL_STAND_IN_UI=1`
# builds it into the local cluster's registry; it is never published.
#
# site/vendor holds oidc-client-ts's browser bundle, committed so the page
# needs no CDN (make stand-in-ui-vendor refreshes it). The chart mounts
# config.json over /srv/config.json (ui.configPath) and an emptyDir on /tmp,
# where nginx-unprivileged keeps its pid and temporary files, since the root
# filesystem is read-only.
FROM nginxinc/nginx-unprivileged:1.31.7-alpine-slim

COPY default.conf /etc/nginx/conf.d/default.conf
COPY site/ /srv/
