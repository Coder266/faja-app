---
name: implement-epic
description: Orchestrates the implementer (Sonnet) and reviewer (Opus) subagents to implement stories from docs/stories.md in an implement→review→fix loop until the reviewer approves, committing each approved story. Usage - /implement-epic [1|2|3|S<n>|all], default all.
---

You are orchestrating the implementation of faja-app stories using two project subagents: `implementer` (writes code, never commits) and `reviewer` (read-only review with a strict verdict protocol). You do not implement or review code yourself — your job is scoping, sequencing, relaying context between the agents, committing approved work, and reporting.

## 1. Determine scope

Parse the skill argument:

- `all` or no argument → every unimplemented story, S1 → S10 in order.
- `1`, `2`, `3` → that epic's stories (Epic 1 = S1–S5, Epic 2 = S6–S8, Epic 3 = S9–S10).
- `S<n>` → that single story.

Read `docs/stories.md` and `git log --oneline -20`. A story whose commit (`S<n>: …`) already exists is done — skip it and note that in your report. Stories run strictly in ID order: S5 depends on S1–S4, S7 on S5–S6, S9 on both apps existing.

Create one task per in-scope story with TaskCreate and keep statuses current as you go.

## 2. Per-story loop

Work on exactly one story at a time. The working tree must be clean (relative to the stories in scope) when a story starts, because the reviewer reviews *all* uncommitted changes.

1. **Implement.** Spawn a fresh `implementer` subagent (`run_in_background: false`). The prompt must be self-contained — subagents share no context with you. Include: the full story text from `docs/stories.md` (title, user story, every acceptance criterion), the data-sources table from the top of that file if the story touches an API, and the reminder that it must not commit.
2. **Review.** Spawn a fresh `reviewer` subagent (`run_in_background: false`). Include the same full story text and tell it to review the uncommitted working tree against those criteria. If the implementer flagged pushback, stubs, or omissions, relay those verbatim so the reviewer can judge them.
3. **Check the verdict.** The reviewer's final message starts with `VERDICT: APPROVED` or `VERDICT: CHANGES_REQUESTED`.
   - **CHANGES_REQUESTED** → send the complete numbered findings list to the *same* implementer agent via SendMessage (it keeps its context). Then send the implementer's fix report back to the *same* reviewer via SendMessage for re-review. Repeat.
   - A message that follows neither format → ask that agent once to restate in the required format; if it still doesn't, treat as CHANGES_REQUESTED and continue.
4. **Loop cap.** Maximum **3 fix rounds per story**. If the reviewer still requests changes after round 3, stop the whole run: report the story ID, the unresolved findings verbatim, and both agents' positions on any disputed finding. Do not commit the story and do not start the next one — the user decides.
5. **Commit on approval.** Stage only the files belonging to this story (`git add` them explicitly, never `git add -A`) and commit as:

   ```
   S<n>: <short story title>

   Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>
   ```

   Mark the story's task completed, then move to the next story.

## 3. Final report

After the last story (or an early stop), run the full verification once yourself — `cd backend && go test ./...` and, if the frontend exists, `cd frontend && npm run build` — and report: stories completed (with commit hashes), stories skipped as already done, review rounds needed per story, and anything left unresolved.

## Ground rules

- Never edit code yourself, even for a one-line reviewer nit — that's what fix rounds are for.
- Never let the implementer and reviewer verdicts pass through you paraphrased when they disagree — quote them.
- Do not push to the remote unless the user asked for that separately.
- If a subagent errors out or hangs, retry it once with the same prompt before reporting failure.
