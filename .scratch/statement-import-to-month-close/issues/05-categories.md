# 05: Categories

**What to build:** a User builds their Household's Income and Expense Category tree (for example Food › Eating out › Coffee), sees it listed as a tree with the Internal Categories, and can rename or move a Category. Spec: `../spec.md` stories 9–14; ADR-0004.

**Blocked by:** 04 (Households, membership, Internal Categories and Idempotency-Key)

**Status:** ready-for-agent

- [ ] `POST/GET /households/{household_id}/categories` and `PATCH …/categories/{category_id}` (rename or move)
- [ ] Kind is Income, Expense or Internal; a child inherits its parent's kind, so an Expense tree never contains an Income Category
- [ ] Names are unique among siblings, on create, rename and move
- [ ] Users can't create Internal Categories, or nest under or move Internal ones
- [ ] The list returns the tree including the three Internal Categories
- [ ] Decider tests: kind inheritance, sibling uniqueness, Internal Category refusals
- [ ] HTTP tests: create, nest, list, rename, move, each refusal; another Household gets 404 for every Category route
- [ ] Row-level security test for every table this ticket adds; Category fold registered for rebuild
- [ ] Consistency map row for Create Category matches the code
