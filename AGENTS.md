BIG.BROTHER is a "toy" client/server web video conferencing application for my home.

Do not overengineer this application. It does not need to be "production grade." It will never be used outside my home. The app does not need to enforce security -- our home network itself is the security boundary.

## Directory Structure

- docs/spitballing
  - Free-form brainstorming/spitballing docs
- docs/memories
  - Findings and discoveries live here. 
  - The criteria for a memory is, "would it save time and/or tokens for a future LLM to retrieve a memory from here, as opposed to re-discovering it later?"
  - filename format: `docs/memories/YYYYMMDD-foo.md` where foo is a brief sensible sumnation of the memory (~50-80 chars)
    - a TSV or similar 
    - one line per memory, containing
      - filename
      - status (fresh/stale/superceded)
      - date last edited
      - date last useful
      - commit hash at time of creation
      - commit hash at time of last usefulness
- docs/adrs
  - Identical in structure to docs/memories, except for durable application decision records (ADRs)
- docs/features
  - When we implement a nontrivial feature, the design and implementation docs go here
  - filename format: `docs/features/YYYYMMDD-foo-design.md` and `docs/features/YYYYMMDD-foo-implementation.md` for a given feature `foo` where foo is a brief sensible sumnation of the feature
  - the implementation file should be a checklist of tasks, suitable for execution by a cheap agent like Luna or Haiku. the implementing ages should check each task off when it is complete so the task can be picked up by another agent
- tmp/
  - gitignored
  - use this for any transient crap


## Notes

- Developer(s) will be using Codex, Claude, and other agents/harnesses. Durable project metadata (ADRs, memories, etc) should be source-controlled and agent/harness-agnostic
