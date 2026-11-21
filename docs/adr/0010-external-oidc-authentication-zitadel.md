# Authentication through an external OIDC provider; Zitadel as the reference IdP

The backend never stores passwords. The API accepts OAuth2 bearer tokens (JWTs) issued by an external OpenID Connect provider and checks them in an HTTP adapter, as the Zalando guidelines expect. The domain only ever sees a User ID. Zitadel is the reference provider: it ships in the Docker Compose setup for development and self-hosting. Any compliant OIDC provider can replace it through configuration, which also covers a hosted provider if the app becomes a service (ADR-0005). We rejected built-in authentication because it would mean owning password storage, resets and MFA. Authentication proves who someone is but does not admit them to the instance. Only redeeming an Invitation creates a User (ADR-0017).

## Status

Accepted. Amended by ADR-0019 and ADR-0022: Zitadel is a platform service, not shipped by Compose.
