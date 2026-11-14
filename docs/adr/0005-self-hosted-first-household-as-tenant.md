# Self-hosted first, with the Household as the tenant boundary

Wallet Accountant first runs as a self-hosted instance, operated by a Household or someone they trust. It is built so it can later run as a hosted service that anyone can sign up to. Self-hosting keeps GDPR, security and bank-aggregator licensing obligations off the project for now. To keep the hosted option open without a rewrite, every piece of data and every query is scoped to a Household from day one (see ADR-0003), even when an instance only serves one Household.
