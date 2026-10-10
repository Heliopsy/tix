---
name: openspec-archive-change
description: Archive a completed change in the experimental workflow. Use when the user wants to finalize and archive a change after implementation is complete.
license: MIT
compatibility: Requires openspec CLI.
metadata:
  author: openspec
  version: "1.0"
  generatedBy: "1.2.0"
---

Archive a completed change in the experimental workflow.

**Input**: Optionally specify a change name. If omitted, check if it can be inferred from conversation context. If vague or ambiguous you MUST prompt for available changes.

**Steps**

1. **If no change name provided, prompt for selection**

   Run `openspec list --json` to get available changes. Use the **AskUserQuestion tool** to let the user select.

   Show only active changes (not already archived).
   Include the schema used for each change if available.

   **IMPORTANT**: Do NOT guess or auto-select a change. Always let the user choose.

2. **Check artifact completion status**

   Run `openspec status --change "<name>" --json` to check artifact completion.

   Parse the JSON to understand:
   - `schemaName`: The workflow being used
   - `artifacts`: List of artifacts with their status (`done` or other)

   **If any artifacts are not `done`:**
   - Display warning listing incomplete artifacts
   - Use **AskUserQuestion tool** to confirm user wants to proceed
   - Proceed if user confirms

3. **Check task completion status**

   Read the tasks file (typically `tasks.md`) to check for incomplete tasks.

   Count tasks marked with `- [ ]` (incomplete) vs `- [x]` (complete).

   **If incomplete tasks found:**
   - Display warning showing count of incomplete tasks
   - Use **AskUserQuestion tool** to confirm user wants to proceed
   - Proceed if user confirms

   **If no tasks file exists:** Proceed without task-related warning.

4. **Assess delta spec sync state**

   Check for delta specs at `openspec/changes/<name>/specs/`. If none exist, proceed without sync prompt.

   **If delta specs exist:**
   - Compare each delta spec with its corresponding main spec at `openspec/specs/<capability>/spec.md`
   - Determine what changes would be applied (adds, modifications, removals, renames)
   - Show a combined summary before prompting

   There is no "archive without syncing" option. A delta that is not folded
   into `openspec/specs/` is a delta that was written and then discarded, and
   the next `MODIFIED` against that capability has nothing to modify. This
   repository ran that way for fifteen archives and ended up with no baseline
   at all, so every `MODIFIED` delta reported "target spec does not exist" and
   agents began rewriting deltas as `ADDED` to silence it.

5. **Perform the archive**

   One command. It moves the change under `openspec/changes/archive/` with
   today's date *and* folds its deltas into `openspec/specs/`, in that one
   step:

   ```bash
   OPENSPEC_TELEMETRY=0 openspec archive <name> -y
   ```

   Never pass `--skip-specs`, and never do the move by hand with `mv`. Both
   skip the fold. `--skip-specs` is for a change that genuinely has no delta
   specs, and such a change has nothing to skip anyway.

   If the fold is refused, read the refusal rather than working around it. A
   `MODIFIED` block replaces its whole requirement, so archive refuses to drop
   a scenario the current spec still has: carry it forward, or say explicitly
   that the requirement is superseded. Converting the delta to `ADDED` to make
   the message go away loses the amendment.

   **Then write the Purpose.** A capability created by this archive gets
   `TBD - created by archiving change ...`, which fails both markdownlint
   MD022 and `openspec validate --specs --strict`. Replace it with a sentence
   saying what the capability is for, editing `openspec/specs/<capability>/spec.md`
   directly: a `## Purpose` in a delta is read only when the capability is
   created.

6. **Display summary**

   Show archive completion summary including:
   - Change name
   - Schema that was used
   - Archive location
   - Whether specs were synced (if applicable)
   - Note about any warnings (incomplete artifacts/tasks)

**Output On Success**

```
## Archive Complete

**Change:** <change-name>
**Schema:** <schema-name>
**Archived to:** openspec/changes/archive/YYYY-MM-DD-<name>/
**Specs:** ✓ Synced to main specs (or "No delta specs" or "Sync skipped")

All artifacts complete. All tasks complete.
```

**Guardrails**
- Always prompt for change selection if not provided
- Use artifact graph (openspec status --json) for completion checking
- Don't block archive on warnings - just inform and confirm
- Preserve .openspec.yaml when moving to archive (it moves with the directory)
- Show clear summary of what happened
- If sync is requested, use openspec-sync-specs approach (agent-driven)
- If delta specs exist, always run the sync assessment and show the combined summary before prompting
