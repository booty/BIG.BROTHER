BIG.BROTHER is a "toy" client/server web video conferencing application for my home.

Do not overengineer this application. It does not need to be "production grade." It will never be used outside my home. The app does not need to enforce security -- our home network itself is the security boundary.

## Directory Structure

- docs/spitballing
  - Free-form brainstorming/spitballing docs
- docs/memories
  - Findings and discoveries live here: environment quirks, debugging findings, and behavior that is not obvious from the code.
  - The criterion for a memory is, "would it save time and/or tokens for a future LLM to retrieve a memory from here, as opposed to rediscovering it later?"
  - Filename format: `docs/memories/YYYYMMDD-foo.md` where foo is a brief descriptive summary of the memory (~50-80 chars).
  - Each memory records the finding, when it matters, supporting evidence (commands, files, or observations), and the last verified date with an optional commit hash.
  - `docs/memories/index.txt` is a TSV with a header and one line per memory: `filename`, `status`, `summary`.
  - Statuses: `current`, `stale`, `superseded`. Summaries should be one sentence with useful search terms.
- docs/adrs
  - Durable application decision records (ADRs) explain what we chose and why it fits this project.
  - Filename format: `docs/adrs/YYYYMMDD-foo.md` where foo is a brief descriptive summary of the decision.
  - Each ADR records the decision, context and constraints, alternatives considered and tradeoffs, and what would justify revisiting it.
  - `docs/adrs/index.txt` uses the same TSV columns as the memory index: `filename`, `status`, `summary`.
  - Statuses: `proposed`, `accepted`, `superseded`. An old ADR can still explain a past decision accurately; age alone does not invalidate it.
- docs/features
  - When we implement a nontrivial feature, the design and implementation docs go here
  - Filename format: `docs/features/YYYYMMDD-foo-design.md` and `docs/features/YYYYMMDD-foo-implementation.md` for a given feature `foo` where foo is a brief descriptive summary of the feature.
  - The implementation file should be a checklist of tasks, suitable for execution by a cheap agent like Luna or Haiku. The implementing agents should check each task off when it is complete so the task can be picked up by another agent.
  - Link to durable decisions in ADRs rather than duplicating them in feature documents.
- tmp/
  - gitignored
  - use this for any transient crap


## Reading and Maintaining Project Knowledge

- Before relevant work, scan the memory and ADR indexes if they exist, then read only the documents that match the task.
- Routine changes do not require a memory or ADR. Keep documents short and avoid duplicating information readily available in the code.
- When evidence contradicts a memory, correct it or mark it `stale` and explain what needs rechecking. Update its index entry to match.
- Update a memory's last verified date only when its claims have actually been checked. Reading or using a document does not verify it; do not track last-use dates or commits.
- When superseding a memory or ADR, mark its index entry `superseded` and link the old document to its replacement. Preserve the rationale of past decisions.
- Keep index entries in sync when creating or changing documents. Use filenames relative to the index directory.

## Notes

- Developer(s) will be using Codex, Claude, and other agents/harnesses. Durable project metadata (ADRs, memories, etc) should be source-controlled and agent/harness-agnostic
