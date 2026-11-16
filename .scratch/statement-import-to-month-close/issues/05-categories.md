# 05: Categories

**What to build:** a User builds their Household's Income and Expense Category tree (for example Food › Eating out › Coffee), sees it listed as a tree with the Internal Categories, and can rename or move a Category. Spec: `../spec.md` stories 9–14; ADR-0004.

**Blocked by:** 04 (Households, membership, Internal Categories and Idempotency-Key)

**Status:** claimed

- [ ] `POST/GET /households/{household_id}/categories` and `PATCH …/categories/{category_id}` (rename or move)
- [ ] Kind is Income, Expense or Internal; a child inherits its parent's kind, so an Expense tree never contains an Income Category
- [ ] Names are unique among siblings, on create, rename and move
- [ ] Users can't create Internal Categories, or nest under or move Internal ones
- [ ] The list returns the tree including the three Internal Categories
- [ ] Decider tests: kind inheritance, sibling uniqueness, Internal Category refusals
- [ ] HTTP tests: create, nest, list, rename, move, each refusal; another Household gets 404 for every Category route
- [ ] Row-level security test for every table this ticket adds; Category fold registered for rebuild
- [ ] Consistency map row for Create Category matches the code

## Comments

Implementation notes (implementer agent):

- **Shape.** Each Category stays its own aggregate (stream `ledger.category`, as ticket 04 seeded them). New commands `CreateCategory` and `ChangeCategory` (rename when `Name` is set, move when `Move` is set, both at once allowed); new events `io.walletaccountant.ledger.category.renamed.v1` and `…moved.v1`. Cross-Category rules are decided from facts the app reads and passes in the command: the `Parent` (ID, kind, path from the root) and the sibling names in the place the Category ends up. The fold now carries its own `ID` (from the envelope's stream ID), so the Decider refuses a move under itself or a descendant. No migration: the `categories` table already had every column.
- **Concurrency.** `ledgerapp.Categories` takes a transaction-level advisory lock on the Household's Category tree (`CategoryReader.LockTree`) before reading parents and siblings, so two creates of one name, or two moves that would make a cycle, serialize. A unique index was rejected on purpose: names compare case-insensitively after trimming, cycles span rows, and rebuild saves each Category's fold in its own transaction, so a name swap between siblings would trip such an index mid-rebuild. The HTTP concurrency test (16 parallel creates of one name: one 201, the rest 409) passes, but it does **not** catch removal of the lock: requests arrive spread over ~20 ms while the read-to-commit window is ~1 ms. A deterministic proof would need a test at the adapter level, outside the agreed seams.
- **Rules.** Root Categories need `INCOME` or `EXPENSE`; a sub-Category takes its parent's kind (`kind` optional, a different one refused). Names are trimmed, unique among siblings ignoring case (Internal roots count, so a root "Uncategorised" is refused), and a rename that changes only the case is allowed. Internal Categories can't be created, renamed, moved, or nested under. An empty PATCH, or one that changes nothing, records nothing and returns 200.
- **API.** `POST …/categories` (201), `PATCH …/categories/{category_id}` (200; `parent_id: null` moves to the root, absent leaves it), `GET …/categories` now returns `CategoryTree`: root `items`, each a `CategoryNode` with `parent_id` and `children` (absent on a leaf), siblings sorted by name. Problems: 409 `/problems/category-name-taken`; 422 `/problems/internal-category`; 422 `/problems/invalid-category` (blank name, missing/unknown/mismatched kind, unknown parent, cycle). An unknown Category, or another Household's (even through the caller's own Household path), is 404. ticket 04's households tests still pass, since the roots are the three Internal Categories.
- **Contract lint.** vacuum warns on any circular reference, so `make lint-api` now passes `--ignore-array-circle-ref` (a tree recursing through an array always ends). `children` is optional because vacuum treats a required recursive property as infinite.
- **Consistency map.** Create Category row updated (tree locked, parent and siblings checked); a Rename or move Category row added at the end of the table.
- **Flaky test, not from this ticket.** `internal/platform/forwarding` `TestAHandlerActsOnlyWithinTheEventsHousehold` failed 2 of about 5 runs: the handler's observation for an event is missing after `drain`, so `Drain` seems able to return before the handler commits. It passed in the final `make test-integration` run.
