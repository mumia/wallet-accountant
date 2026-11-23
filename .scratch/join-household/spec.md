# Spec: Joining a Household

Status: ready-for-agent

Today the only way into an instance is the bootstrap Invitation (ADR-0017). It admits the operator's first User, and nobody else can follow. A Household therefore always has exactly one member, the User who created it. This spec lets a member invite someone else into their Household, whether that person has never used the instance or is already a User with a Household of their own. It also covers seeing who the members are, revoking an Invitation and leaving or removing a member. ADR-0017 deferred all of this as "comes later". It was decided with the user on 2026-11-23.

This spec covers only the backend. The UI gets its own spec in `../wallet-accountant-ui` once ticket 01's contract lands.

## Problem Statement

Wallet Accountant exists so that a Household can keep one shared book of record. Yet a Household can't have a second member. Someone who signs in can't get an Invitation, because the app can't issue one. Two people who each set up their own Household can't merge into one. Nothing records who a User is in human terms, so even a member list would be a list of UUIDs. Nothing tells members who can see their money, and nobody can take that access away.

## Solution

Any member of a Household can issue an **Invitation** to that Household. Its code works once, lasts 24 hours and is shown only to the member who issued it. Whoever redeems it becomes a **Member**:

- someone **Unregistered** redeems it at `POST /registrations`, which registers them as a User and adds them to the Household in one step;
- an existing User redeems it at the new `POST /memberships`.

Any member can list and revoke the Household's open Invitations, list its members, remove another member or leave. The last member can't leave. Every User now has a **display name**: they choose it when registering and can change it later. Joining, leaving and removal each create a Notification every member sees.

Any member can now admit new people to the instance through an Invitation to their Household. That delegates part of the Instance administrator's control over admission to the members of each Household. A new ADR records this and amends ADR-0017.

## User Stories

### Inviting

1. As a member, I want to issue an Invitation to my Household, so that the person I share money with can join it.
2. As a member, I want to see an Invitation's code only once, when I issue it, so that nobody can read it back later from the list.
3. As a member, I want an Invitation to expire 24 hours after I issue it, so that a code I sent and forgot stops working.
4. As a member, I want to list my Household's open Invitations, each with who issued it and when it expires, so that I know which codes could still let someone in.
5. As a member, I want to revoke any open Invitation, including one another member issued, so that a code sent to the wrong person can't be used.

### Joining

6. As someone Unregistered, I want to redeem an Invitation to a Household and become both a User and a member of that Household, so that one code is all I need.
7. As a User who already has a Household, I want to redeem an Invitation to another Household and become its member, so that I can share books with someone who set up first.
8. As a User, I want redeeming a code for a Household I already belong to refused, with the code left unused, so that clicking my own link by mistake doesn't waste it.
9. As anyone redeeming a code, I want an unknown, expired, revoked or already redeemed code to get the same error, so that codes can't be probed.
10. As a new member, I want to see all of the Household's Accounts, Categories, Transactions and history at once, so that I'm a full member from the start (ADR-0003).

### Display names

11. As someone Unregistered, I want to give a display name when I redeem an Invitation, so that other members know who I am.
12. As a User, I want to change my display name, so that a name chosen in a hurry isn't permanent.
13. As a User who registered before display names existed, I want to be able to set one, so that I don't stay nameless.

### Members

14. As a member, I want to list my Household's members with their display names and when they joined, so that I know who can see our money.
15. As a member, I want to leave a Household, so that I stop seeing books that are no longer mine.
16. As a member, I want to remove another member, so that someone who shouldn't see our books any more can't.
17. As the last member, I want leaving refused, so that a Household is never left with nobody who can reach it.
18. As a User whose last Household I left or was removed from, I want to stay a User and be able to create a new Household, so that losing a Household doesn't lock me out of the instance.
19. As a former member, I want to be able to rejoin through a new Invitation, so that a mistaken removal can be undone.

### Telling the Household

20. As a member, I want a Notification when someone joins, so that I know who can now see our books.
21. As a member, I want a Notification when someone leaves or is removed, naming who removed whom, so that removal never happens silently.

## Implementation Decisions

### Contract

Ticket 01 writes these into `api/openapi.yaml`. Every other ticket implements part of them and must not change the contract without a comment on this spec.

- **New operations:**
  - `POST /memberships` (`createMembership`): body `{ invitation_code }`. Answers 201 with the `Household` joined. It takes an Idempotency-Key.
  - `GET /households/{household_id}/members` (`listMembers`): items of `{ user_id, display_name, joined_at }`, oldest first.
  - `DELETE /households/{household_id}/members/{user_id}` (`removeMember`): your own ID means leaving. Answers 204.
  - `POST /households/{household_id}/invitations` (`createInvitation`): no body. Answers 201 with `{ invitation_id, code, expires_at }`. The code appears in this response and never again. It takes an Idempotency-Key. A replay returns the stored response, code included, the same way Idempotency-Key replays already work.
  - `GET /households/{household_id}/invitations` (`listInvitations`): open Invitations only, each `{ invitation_id, issued_by, issued_at, expires_at }`, oldest first.
  - `DELETE /households/{household_id}/invitations/{invitation_id}` (`revokeInvitation`): answers 204.
  - `PATCH /me` (`changeMe`): body `{ display_name }`. Answers with `Me`.
- **Changed schemas:**
  - `Registration` gets a required `display_name`. **This breaks the UI's `/register` form,** which must add the field. The bootstrap Invitation is redeemed through the same endpoint, so the first User names themselves too.
  - `Me` gets a nullable `display_name`.
  - `Notification.kind` gets `MEMBER_JOINED`, `MEMBER_LEFT` and `MEMBER_REMOVED`, with nullable `member` and `removed_by` fields, each `{ user_id, display_name }`. Names are included because a removed member is no longer in the members list.
- **Display name:** trimmed, 1–64 characters and not unique. A blank value gets 400 through the existing validation problem.
- **New problem types:**
  - `/problems/already-a-member`: 409 from `createMembership` when the User is already a member of the Household. The code stays unused.
  - `/problems/last-member`: 409 from `removeMember` when the member is the only one.
- **Unchanged problems:**
  - An invalid code still gets `/problems/invalid-invitation` (422), whatever the reason: unknown, expired, revoked, redeemed or superseded. That now also covers `createMembership`.
  - A User calling `createRegistration` still gets `/problems/already-registered` (409).
  - A caller who isn't a member gets 404 on every new Household-scoped route. So does an `invitation_id` that isn't an open Invitation of this Household, and a `user_id` that isn't a member.
- **Unregistered callers on `createMembership`:** they get 403 `registration-required`, like `GET /me`, so the UI sends them to `/register` with the code.
- `Household` is unchanged.

### Households: Invitation in Household scope

- An Invitation to a Household lives in **Household scope** (ADR-0014). Its events carry the `householdid`, and its fold table has `household_id` with the usual row-level security policy. Issue, list and revoke are ordinary Household commands.
- The bootstrap Invitation stays in instance scope, unchanged.
- **The lookup row:** only the row from code hash to Invitation stays in instance scope, so that someone who doesn't know the Household yet can redeem the code. It gains a nullable `household_id`, which is null for the bootstrap Invitation.
- **The Decider:** the existing Invitation Decider gains a Household and a revoke.
  - `invitation.issued` records the issuing User as `actorid` and has no `issuedby: operator`. The Household comes from the envelope's `householdid`, and the role is always `user`.
  - Expiry is 24 hours (`CodeLifetime`). An Invitation to a Household is never reissued: each issue is a new stream.
  - `invitation.revoked` (no data) is refused once the Invitation is redeemed or already revoked. Revoking an Invitation that has expired but was never redeemed is allowed, since it does no harm.
  - Redeeming is refused for a revoked Invitation, using the same error path as a wrong code.
- **Listing open Invitations:** "open" means not redeemed, not revoked, and `expires_at` later than the request's clock. Expired rows are filtered when read, so no expiry event is needed (ADR-0017).
- **Issuers who leave:** an Invitation issued by a member who later leaves or is removed stays valid until it expires or is revoked.

### Households: redeeming into a Household

Both redeem endpoints run the same command, Join Household, in one database transaction. The order follows ADR-0017's rule for commands that span both scopes:

1. In instance scope, resolve the caller's User, if they have one. Look up the code hash to find the Invitation and its Household. If the lookup finds nothing, or finds the bootstrap Invitation when called through `createMembership`, refuse with `invalid-invitation`.
2. **Unregistered caller (`createRegistration`) only:** run the User Decider, so the `user.registered` event now carries `displayname` and role `user`, and save the identity lookup row. The User's ID is new, as today.
3. Set `app.household_id` to the Invitation's Household.
4. Run the Household Decider with `AddMember { UserID, InvitationID }`. It refuses with `already-a-member` if the User already belongs. It records `household.memberadded` with a new optional `invitationid`. This runs before the Invitation is marked redeemed, so a refusal rolls back the whole transaction and the code stays unused.
5. Run the Invitation Decider's Redeem. It checks the code, the expiry against the command's timestamp, revocation and redemption, and records `invitation.redeemed`.
6. Save the Household fold, which also writes the `user_households` index in instance scope, as today.

Notes on this command:

- **Concurrency:** if two people redeem one code at the same time, the Invitation's version check makes one of them retry, and the retry finds the code redeemed. If two codes for the same Household are redeemed by one User at the same time, the Household's version check catches the second, and its retry gets `already-a-member`.
- **Unregistered callers:** `createRegistration` with the bootstrap code behaves as today, with a display name added. `createRegistration` with a Household code registers the person and makes them a member.
- **Event order:** steps 2 and 4–5 write instance-level events first, then Household events, in one transaction, as ADR-0017 foresaw.

### Households: User display name

- `UserRegistered` gains `displayname`. It is omitted on events recorded before this change, and the fold reads it as null.
- `RenameUser` records `user.renamed { displayname }`. The same name again records nothing. This is an instance-scope command on the caller's own User only.
- **Personal data:** the display name joins the issuer and subject as personal data in the permanent event store. That's acceptable while instances are self-hosted (ADR-0005, ADR-0017's erasure caveat).

### Households: members

- `household.memberadded` gains optional `invitationid`. It is absent for the Household's creator.
- `household_members` gains `joined_at`, taken from the `memberadded` event's time. A migration backfills it for existing rows from the event store.
- `RemoveMember { UserID, RemovedBy }` records `household.memberremoved { userid }`, with `actorid` set to whoever removed them. When `actorid` equals `userid`, the member left.
  - It is refused with not found if the User isn't a member, which the handler answers with 404.
  - It is refused with `last-member` if they are the only member.
  - The Household fold's `Members` drops the User, and saving the fold removes both index rows, as `Save` already does.
- **Losing access:** the next request is refused immediately, because every Household route checks `household_members` in its own transaction.
- **What stays:** a removed User keeps their User, their earlier events keep naming them as `actorid`, and their Notification read state is left in place.
- **Members listing:** read the member IDs and `joined_at` in Household scope, then switch to instance scope to read display names, using the existing `inInstanceScope` pattern from `HouseholdFolds.Save`. No display name is copied into Household scope, so a rename needs no projection.

### Households: Notifications

- **New kinds:** `member_joined`, `member_left` and `member_removed`. They are raised by deferred Households handlers on `household.memberadded` and `household.memberremoved`.
  - `member_joined` is raised only for a `memberadded` event that has an `invitationid`, so a Household's creator doesn't notify an empty room.
  - `member_left` or `member_removed` is chosen by whether `actorid` equals `userid`.
- **Contents:** the Notification stores the member's User ID and, for removal, the remover's. The list reads their display names the same way the members listing does.
- **Read state:** it stays per User. A new member sees Notifications raised before they joined as unread, which follows from ADR-0003: they see everything.
- **Deduplication:** through the processed-events table, like `suspected_duplicates_found`.

### Consistency

Add or update these rows in `docs/architecture/consistency.md`:

| Command | Contexts | Immediate | Deferred |
|---|---|---|---|
| Issue Invitation to a Household | Households | Invitation issued in Household scope; its code lookup row (instance scope) saved with the Household | — |
| Revoke Invitation | Households | Invitation revoked (refused once redeemed or already revoked) | — |
| Join Household (`createRegistration` with a Household code, or `createMembership`) | Households | Code looked up in instance scope; for an Unregistered caller, User registered with display name and identity lookup row; Household set; member added (refused if already a member, which leaves the code unused); Invitation redeemed; membership index rows saved | `member_joined` Notification |
| Remove member / leave | Households | Member removed (refused for the last member or a non-member); both membership index rows deleted | `member_left` or `member_removed` Notification |
| Rename User | Households | `user.renamed` recorded (nothing for the same name) | — |
| Redeem Invitation (bootstrap) | Households | Unchanged, except that the registered User now carries a display name | — |

### Documentation

- **New ADR, "Invitations to a Household"**, amending ADR-0017. It records:
  - any member may issue one, and it also admits new people to the instance;
  - Household scope with an instance-scope code lookup;
  - the 24-hour lifetime;
  - bearer codes, not identity-bound ones, for the reasons ADR-0017 gave;
  - revocation;
  - equal members removing each other and the last-member rule;
  - display names as personal data.

  ADR-0017's "Invitations" section gets a pointer to the new ADR.
- **`internal/households/CONTEXT.md`:**
  - add **Member** ("A User who belongs to a Household. Every Member is equal and sees everything in it."; avoid: participant, owner);
  - update **Invitation** to say it admits one person to the instance, a Household, or both;
  - add **display name** to **User**;
  - say in **Household** that it always has at least one Member.

### Delivery

- Ticket 01 (the contract) merges first, and its merge commit is tagged **`v0.4.0`**. The UI pins its `openapi.yaml` to that tag. The release answers the new operations with 501 until their tickets land, as with `v0.2.0`.
- Tickets are in `issues/`. 02 (docs) can go any time. After 01, the order is 03 and 08 in parallel; then 04, followed by 05 and 06 in parallel; then 07 and 09; and 10 last.

## Testing Decisions

- **What a good test checks:** behaviour through the public interface. Decider tests go through `Decide`/`Evolve` (`decidertest`), and application behaviour through HTTP integration tests (`apitest`, testcontainers PostgreSQL). Tests never check internal structure. ogen validates every response against the contract.
- **Decider tests** (`internal/households/domain`):
  - **Invitation:** issue to a Household; revoke, and revoke again refused; redeem a revoked one refused; revoke a redeemed one refused; expiry at exactly 24 hours.
  - **Household:** add a member; add an existing member refused; remove a member; leave; remove the last member refused; remove a non-member refused; rejoin after removal.
  - **User:** register with a display name; rename; rename to the same name records nothing; a blank name refused.
- **HTTP tests:**
  - An Unregistered person redeems a Household code through `/registrations` and sees the Household's existing Accounts.
  - A User with their own Household joins a second Household through `/memberships`, and `GET /households` lists both.
  - Redeeming a Household you already belong to gets 409, and the same code then works for someone else.
  - Every invalid-code case gets the same 422 on both endpoints: unknown, expired (clock moved past 24 hours), revoked, redeemed, and the bootstrap code on `/memberships`.
  - An Unregistered caller on `/memberships` gets 403.
  - Two concurrent redemptions of one code succeed exactly once.
  - Listing shows only open Invitations, and the code never appears in a list.
  - A non-member gets 404 on every new Household route.
  - Remove, leave and the last-member refusal; a removed member gets 404 on their next request and lands with zero Households.
  - Rename through `PATCH /me`; the members list reflects the rename; a legacy User shows a null display name.
  - Idempotency-Key replay on `createInvitation` returns the same code.
  - Notifications for joined, left and removed appear after events are drained, and none is raised for creating a Household.
- **Row-level security:** an adapter-level test shows that a Household Invitation's fold rows are invisible from another Household and from instance scope, and that the code lookup row is invisible from Household scope.
- **Clock:** tests set the clock through the existing injected time source and never sleep.
- **Prior art:** the registration and bootstrap tests from `.scratch/statement-import-to-month-close/issues/03-bootstrap-and-registration.md`, the Household membership tests from ticket 04 of that slice, and the Notification handler tests from ticket 13.

## Out of Scope

- **The UI,** which gets its own spec in `wallet-accountant-ui`: the Invite and members screens, the `/join?invitation_code=` route for existing Users, and the display name field on `/register`.
- **Admission without a Household:** Instance administrators issuing in-app Invitations that admit someone with no Household.
- **Identity-bound Invitations:** codes tied to an email address or an OIDC identity.
- **Invitation lifetime:** a lifetime the issuer chooses, or reissuing an Invitation to a Household.
- **Roles inside a Household,** such as owner or viewer, and any per-Account sharing (ADR-0003).
- **Household lifecycle:** deleting a Household, or transferring it.
- **Erasing a User's personal data,** by crypto-shredding or otherwise.
- **Delivery outside the app:** email or other out-of-app delivery of Invitations or Notifications.
- **Rate limits** on redeeming codes. 256-bit codes make guessing pointless.

## Further Notes

- **Decided in the grilling on 2026-11-23:**
  - one Invitation concept that can optionally name a Household (Q2);
  - any member issues, and that admits new people (Q3);
  - bearer codes (Q4);
  - 24-hour expiry (Q5);
  - list, revoke and leave or remove are in scope (Q6);
  - users type their display name (Q7);
  - Household scope for the Invitation (Q14).
- **A trust choice:** any member can both admit a stranger to the instance and remove another member. That's acceptable for a self-hosted family instance. A future hosted service (ADR-0005) should revisit both.
- **A breaking change:** the required `display_name` on `Registration` breaks the current UI's `/register` form until the UI's spec catches up. Ticket 01 should note this in the contract's description, and the UI's spec should land soon after.
- **The UI's smoke test** (`wallet-accountant-ui` ticket 21) currently needs a fresh cluster because the API can't issue Invitations. Once this lands, it can issue a Household Invitation instead.
