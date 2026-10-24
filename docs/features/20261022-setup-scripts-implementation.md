# Setup scripts implementation plan

**Goal:** Fast, repeatable setup and a single native launch command.
**Design:** [Setup scripts design](20261022-setup-scripts-design.md).

- [x] Add a repeatable shell integration check for initial setup, unchanged setup,
  source changes/deletions, dependency changes, missing outputs, failed build
  retries, and launch argument forwarding. Use real build tools and cached downloads.
- [x] Make `scripts/build` cache its dependency, frontend, Go, and media stages;
  never mark failed work successful and retain cross-compilation support.
- [x] Add native `bin/setup` and `bin/launch`, with prerequisite checks and help.
- [x] Generate localhost TLS certificates on setup; reuse existing certificates
  and preserve the CA on server-pair recovery. Default launch to the repo-local
  directory and verify explicit config/TLS overrides still take precedence.
- [x] Document the commands, caching/recovery behavior, prerequisites, and TLS use.
- [x] Run the script integration checks, existing TLS path check, and backend unit
  tests. Backend tests require socket access outside the restricted sandbox.
- [ ] Real launch/health/shutdown smoke check: blocked by an existing MediaMTX
  instance on the fixed Control API port 9997. The check stops before launching
  another instance. Repeatable check: `sh tmp/setup-launch-smoke/check.sh`
  while that ignored artifact exists, after stopping the existing instance.

Verified on macOS ARM64: initial native setup/build; unchanged setup in 0.35 s;
integration checks including source/dependency changes, deletion/missing outputs,
failed-stage recovery, paths with spaces, localhost certificate creation/reuse,
CA-preserving server-pair recovery, and explicit config/TLS launch overrides.
Unchanged setup with TLS provisioned took 0.34 s. Linux
execution and a fresh uncached internet download were not exercised.
