# 08: List a Household's members

**What to build:** `GET /households/{household_id}/members` lists every member with their display name and when they joined. Spec: `../spec.md`, "Households: members", story 14.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] Migration: `household_members.joined_at`, backfilled from each member's `household.memberadded` event time. The fold store writes it from the event's time from now on
- [ ] Read the member IDs and `joined_at` in Household scope, then switch to instance scope (`inInstanceScope`) to read display names. Nothing is copied into Household scope
- [ ] Items are `{ user_id, display_name, joined_at }`, oldest first. A legacy User has no `display_name`
- [ ] A caller who isn't a member gets 404. Delete the stub
- [ ] HTTP tests:
  - the creator alone;
  - two members after a join (use ticket 06's helper when it has landed, otherwise seed through a test helper);
  - a rename reflected in the list;
  - no name for a legacy User;
  - another Household gets 404

## Comments

2026-11-24, implementation notes:

- **Migration `000022_household_member_joined_at`:** adds `household_members.joined_at`, backfills it from the latest `household.memberadded` event per member (wa_owner bypasses row-level security), then makes it `NOT NULL`. It is one self-contained file, so if ticket 03 or 04 also takes 000022, renumbering it on merge is just a rename. Its test (`migrations/joined_at_integration_test.go`) migrates to 21 through a new test-only `migrations.UpTo`, seeds rows and events the old way, then migrates the rest.
- **Fold:** `HouseholdState` gains `JoinedAt map[ID]time.Time`, set from the `memberadded` envelope's time. The fold store loads it and writes it with each new member row. The fold schema version is unchanged, because `Load` reads the backfilled column. **Ticket 09:** the `memberremoved` Evolve should delete the User from `JoinedAt` too, so a rejoin records the new time (`Save` keeps an existing row's time, with `ON CONFLICT DO NOTHING`).
- **Listing:** `HouseholdLookups.Members` reads `user_id, joined_at` in Household scope, ordered by `joined_at, user_id`, then reads `users.display_name` inside `inInstanceScope`. `Households.Members` runs it through `InHousehold`, so a non-member or an unknown Household gets 404. The `ListMembers` stub is gone from `pending.go`.
- **Test seams that stand in for tickets not landed yet:**
  - `apitest.SeedMember` writes the member row and the `user_households` row directly as wa_app (ticket 06 hasn't landed). Once 06 is in, the two-members test should join through `/memberships` instead.
  - `apitest.SeedDisplayName` writes `users.display_name` directly (`""` writes NULL, as for a legacy User). It stands in for the rename (ticket 03) and for a User registered before display names. Once 03 is in, the rename test should go through `PATCH /me`.
- **Fixture change:** the RLS test's `household_members` insert now passes `joined_at`.
- **Checks:** `make lint`, `make test` and `make test-integration` pass.
