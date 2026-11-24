# 02: ADR "Invitations to a Household" and the Households glossary

**What to build:** record this spec's decisions where later work will look for them. Spec: `../spec.md`, "Documentation" and "Further Notes".

**Blocked by:** —

**Status:** claimed

- [ ] New ADR `docs/adr/0025-invitations-to-a-household.md`, amending ADR-0017. It records:
  - any member may issue an Invitation to their Household, and it also admits a new person to the instance, which delegates admission to the Household's members;
  - Household scope for the Invitation, with only the code lookup in instance scope, and redeeming as a command that spans both scopes;
  - the 24-hour lifetime and no reissue;
  - bearer codes rather than identity-bound ones (ADR-0017's reasons);
  - revocation, and that Invitations stay valid when their issuer leaves;
  - equal members removing each other, and the last-member rule;
  - display names as personal data in the event store;
  - the rejected alternatives: two codes, issuing by Instance administrators only, and a separate Household invitation concept
- [ ] ADR-0017's "Invitations" section points to ADR-0025
- [ ] `internal/households/CONTEXT.md`:
  - **Member** added;
  - **Invitation** now admits to the instance, a Household, or both;
  - **User** has a display name;
  - **Household** always has at least one Member
- [ ] `CONTEXT-MAP.md` checked, and its Households line updated if needed
