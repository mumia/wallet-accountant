---
name: implement-spec
description: "Implement a spec from its local tickets, running implementer subagents in parallel across the ticket graph."
disable-model-invocation: true
---

You have been given a spec: a feature directory `.scratch/<feature-slug>/` containing `spec.md` and one ticket per file under `issues/`. The tracker conventions are in `docs/agents/issue-tracker.md`. If no feature was named, ask which one.

The goal is one PR that implements the entire spec on a single branch.

The tickets are not a list of steps. They form a **task graph**: each ticket's `Blocked by:` line names the tickets that gate it. A ticket is on the **frontier** when its `Status:` is `ready-for-agent` and every ticket it is blocked by is `resolved`. There is usually more than one frontier ticket, and they can be worked at the same time.

Keep messages to and from subagents short. Communicate through **context pointers**: the spec path, the ticket path, research notes and previous commits. Don't repeat what a pointer already gives.

Run **implementer subagents** in the background whenever possible, for **maximum concurrency**.

## Ticket status

Only you, the orchestrator, change `Status:` lines, and only on the PR branch. Subagents never edit them. This keeps merges free of status conflicts.

- `ready-for-agent` → `claimed` when you start an implementer on the ticket.
- `claimed` → `resolved` once its work is merged into the PR branch.
- A ticket that fails twice, or needs a decision only the user can make, goes to `needs-info`, with the reason appended under `## Comments` at the bottom of its file. Tell the user. Tickets blocked by it wait.

Commit each status change on the PR branch.

## Steps

1. Read the spec and every ticket. Build the task graph from the `Blocked by:` lines and check that it has no cycles and no references to missing tickets. Read `CONTEXT-MAP.md`, the relevant `CONTEXT.md` files and the ADRs the spec cites, so you know which pointers each implementer needs.

2. (optional) Use an **exploration subagent** for exploration the tickets need: relevant code, external docs, prior-art branches the spec names. It saves its markdown notes in a directory outside the repo (in your scratchpad directory) that every later subagent can read. Implementers can then focus on implementing.

3. Create the PR branch `<feature-slug>` from `main`, push it, and open a draft PR with `gh pr create --draft`. The PR body links the spec file and lists every ticket file. There are no tracker issues to close.

4. For each frontier ticket, mark it `claimed`, commit, and start an **implementer subagent** in the background with `isolation: "worktree"`, so each works in its own worktree on its own branch. Make sure your checkout is on the PR branch with the latest merges first, because the worktree starts from it. Give each implementer:
   - the spec path and its ticket path, plus pointers to the exploration notes, if any
   - the instruction to implement test-first with the `mattpocock-skills:tdd` skill. It tests at the seams the spec's Testing Decisions section already fixes, and treats them as agreed. It doesn't ask the user.
   - the instruction to meet every acceptance criterion, run `make lint`, `make test` and `make test-integration` (whichever exist yet), and commit on its branch only when they pass
   - the instruction never to edit `Status:` lines, and to record any note for the ticket under `## Comments` in that ticket's file only
   - the instruction to report back briefly: its branch name, the commit SHA, which criteria are met, and anything it couldn't do

5. When an implementer finishes, start a **merger subagent**. It merges the implementer's branch into the PR branch, resolves conflicts using the `mattpocock-skills:resolving-merge-conflicts` skill, reruns lint and tests on the merged result, and pushes. Run merges one at a time so each one starts from the previous merge. Then mark the ticket `resolved` and commit.

   If an implementer reports unmet criteria or failing checks, start a fresh implementer on the same ticket. Point it at the failed branch and the report.

6. After every merge, recompute the frontier. Start implementers for any ticket that has just unblocked, without waiting for unrelated work to finish.

7. Once every ticket is `resolved`, run `/code-review` on the PR branch against `main`. Fix everything it raises in a single **implementer subagent**, then merge as in step 5.

8. Mark the PR ready for review with `gh pr ready`.

9. Remove every implementer worktree and its local branch. Report to the user: the PR link, the tickets resolved, and any tickets left in `needs-info`.
