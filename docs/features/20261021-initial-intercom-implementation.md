# Initial Intercom Implementation Plan

> Check off steps only when their evidence exists. Preserve incomplete checks and their reasons for the next implementer. `[DEVICE]` requires access to physical hardware; `[MANUAL]` requires human interaction or coordination. Continue independent work when those checks are unavailable, but do not bypass acceptance gates.

**Goal:** Deliver a simple offline home intercom with a Go application backend and shared centralized media.

**Architecture:** One browser frontend talks to one application backend for room state and to an existing media server for audio/video. Establish a public application contract so a later Elixir backend can reuse the frontend and tests.

**Tech Stack:** Go `net/http` with `github.com/coder/websocket`; TypeScript with esbuild and `tsc --noEmit`; `node:test` for contract tests; headless `agent-browser` for supported browser checks; WebRTC/HTTPS; provisionally MediaMTX. Pin installed versions in module/package lockfiles and record the evaluated media/browser-tool versions. Do not reconsider the selected tools absent concrete incompatibility.

**Spec:** [Initial intercom design](20261021-initial-intercom-design.md). Read it with the [accepted ADR](../adrs/20261021-go-first-backends-and-centralized-media.md).

## Global constraints

- Home network only; no application authentication or production-scale infrastructure.
- Runtime must work without internet access; serve every browser asset locally.
- Package `linux/arm64`, `linux/amd64`, and `darwin/arm64`; validate execution separately on available targets. Intel Mac packaging is deferred unless requested. Target Raspberry Pi 5 or AMD A9 Linux hardware, with a 7th-gen i5 laptop as fallback.
- Test current Chrome, Firefox, Safari, Android browsers, and iOS Safari on available target devices.
- Typical room size is 1–3; measure 6 without hardcoding a participant cap.
- Go first; a second backend and camera/recording product features are follow-ups.
- Use the protocol, optional certificate overrides, configuration defaults, and 443-first port policy in the design. All current requirements are in that document; historical spitballing notes are not normative.
- Browser testing follows AGENTS.md: headless `agent-browser` where supported; no automatic fallback to another tool. Unit and protocol tests remain in ordinary runners.

## Review focus

1. Permission denial and autoplay blocking: Task 5 must retain a usable UI and retry action.
2. Abrupt disconnect/reconnect: Task 3 must remove ghosts and avoid duplicate sessions.
3. Incompatible media codecs: Tasks 1 and 4 must surface failure instead of showing a misleading live tile.
4. Camera removal/device switching: Task 5 must avoid leaking tracks or silently retaining the wrong device.
5. Missing internet, occupied ports, or failed media process: Tasks 2 and 6 must remain predictable and explain failures.

## Task 1: Prove the media and HTTPS path before building the product

**Files:** Create `tmp/intercom-probe/` for disposable code and `docs/memories/20261021-media-browser-and-recording-feasibility.md` for results; update the memory index. Create `media/README.md` and a pinned `media/mediamtx.yml` only once validated.

**Interfaces:** Produces the exact browser publish/read endpoints, codec settings, HTTPS arrangement, port map, launch commands, and supported recording container used by Tasks 2 and 4.

- [ ] Check required tools and access first: Go, Node/npm, headless `agent-browser` (inspect its installed help), MediaMTX binary, certificate tooling, target browsers/devices, and an RTSP camera or explicitly labeled representative source. Report missing tools/access before starting their workflows; leave their checks pending and continue independent work. Verify supported browser engines, fake-media controls, and isolated sessions before designing automation around them.
- [ ] Read current MediaMTX documentation for browser publishing/reading, control API, recording, and codecs. Pin the evaluated version and document supported host binaries.
- [ ] Create a disposable publisher/subscriber page and local media configuration; record the exact repeatable launch commands in the probe README.
- [ ] Choose and document one certificate provisioning tool and exact setup commands that place `tls/server.crt` and `tls/server.key` under the design's config directory. Include LAN hostname/IP coverage and server-certificate reissue instructions without CA replacement.
- [ ] [MANUAL] Trust the local CA on desktop and Android/iOS targets. Verify camera permission and audible playback after a user gesture; record platform-specific trust steps.
- [ ] Create a repeatable headless `agent-browser` publish/subscribe scenario for supported engines with explicit assertions and isolated client sessions. Record unsupported checks instead of treating Chromium as Firefox/Safari coverage.
- [ ] Prefer H.264 plus Opus on the publisher, verify the negotiated codec in connection statistics, and exercise recording/replay. Assert or explicitly report unavailable codecs; do not accept a silently negotiated VP8 stream as proof of recording support.
- [ ] Test fresh offline desktop Firefox and a provisioned OpenH264 profile separately. Record what browser setup must occur before offline operation.
- [ ] [DEVICE] Publish and receive real audio/video across desktop Chrome, Firefox, Safari, Android Chrome/Firefox, and iOS Safari. Record versions, codecs, and required settings.
- [ ] [DEVICE] Ingest an accessible RTSP camera (or explicitly labeled representative source), display it in a browser, record a short clip, and play the clip back. Repeat recording with a browser-originated stream. Identify any required conversion and its CPU cost.
- [ ] [MANUAL] Coordinate disabling internet connectivity while retaining LAN connectivity and repeat publish/subscribe. Confirm no public connection service or CDN is required.
- [ ] Record results, limitations, versions, and commands in the memory; index it as current only for claims actually verified. Keep unavailable device checks explicitly pending.
- [ ] Validate Go-style same-origin proxying in the probe: POST creates a session, PATCH updates ICE, DELETE tears down, and returned Location URLs resolve through `/media/`. Verify loopback HTTP/control listeners and reachable LAN ICE candidates.
- [ ] Update the design with exact tested endpoint mappings, proxy headers, ports, media readiness check, certificate setup, and publish/read descriptor fields. These are required inputs to Task 4.
- [ ] **Decision gate:** If MediaMTX cannot pass after routine debugging, or success requires material transcoding cost/architectural changes, stop dependent work, present evidence and alternatives, and ask the owner to choose before continuing that work. Do not select a replacement silently.

**Pass condition:** Reproducible local media exchange, browser/camera recording and playback, offline behavior, and the listed target-browser/device evidence. Missing checks remain unchecked; independent portions of Tasks 2–3 may proceed (packaging/setup steps still need their specific probe outputs), but Task 4 waits for this gate or explicit scope narrowing by the owner. Do not infer compatibility from desktop-only results.

## Task 2: Deliver a launchable Go shell with local assets

**Files:** Create `backends/go/go.mod`, `backends/go/cmd/bigbrother/main.go`, `backends/go/internal/server/{server.go,server_test.go}`, `backends/go/internal/launch/{launch.go,launch_test.go}`, `web/{package.json,tsconfig.json,index.html}`, `web/src/main.ts`, `scripts/build`, `scripts/setup-tls`, and `README.md`.

**Interfaces:** Produces `GET /healthz`, static frontend serving, the design's CLI flags, and a documented `scripts/build` entry point. Build output goes in ignored `tmp/build/OS-ARCH/`; frontend artifacts are bundled with the runnable distribution. `scripts/setup-tls` implements the documented provisioning-tool commands from Task 1; it never silently installs device trust or replaces an existing CA.

- [ ] Add failing Go tests for health JSON, serving a local asset, explicit-port failure, 443-to-8443 fallback on permission/address-in-use errors, and missing/invalid TLS files. Test default config lookup, `--config-dir`, paired certificate overrides, and failure for only one override. Inject candidate ports in tests rather than assuming privileged port access.
- [ ] Implement the small HTTPS server, CLI parsing, port selection, and startup URL reporting. Keep application logic out of `main.go`.
- [ ] Add esbuild bundling, a separate `tsc --noEmit` check, and an offline static page; pin dependencies and retain their lockfile. No frontend framework is required for this shell.
- [ ] Implement `scripts/build --target OS/ARCH` for `linux/arm64`, `linux/amd64`, and `darwin/arm64`, cross-compiling Go and packaging the matching pinned MediaMTX release, config, and local assets. Never copy the host media binary into another target's package. Runtime downloads are forbidden.
- [ ] Implement `scripts/setup-tls` using the probe's documented tool commands. Ensure repeat runs preserve the CA and require explicit action before replacing certificates. Document client trust as a separate manual operation.
- [ ] Run `cd backends/go && go test ./...`; expect all tests to pass. Run `scripts/build --target linux/arm64`, `scripts/build --target linux/amd64`, and `scripts/build --target darwin/arm64`; expect the corresponding package under `tmp/build/OS-ARCH/`. Record packaging separately from runtime testing.
- [ ] After one-time setup, launch plain `bigbrother`, then with `--port`, and verify the page and health response over trusted HTTPS without repeated certificate flags. Document the exact distribution launch command and required media ports.

## Task 3: Define and implement the room contract

**Files:** Create `protocol/room.md`, `tests/contract/{package.json,room.test.ts}`, `backends/go/internal/room/{room.go,room_test.go}`, and `backends/go/internal/server/websocket.go`.

**Interfaces:** Consumes the HTTP shell; produces `/ws`, `join`, `rename`, `mediaState`, `welcome`, `participants`, and `error` behavior defined in the design. Contract tests target `BASE_URL`, with optional `CA_CERT` for the local CA, and never import Go code.

- [ ] Write the protocol examples and specify name fallback, session identity, message errors, WebSocket ping/pong deadlines, and snapshot ordering. Default blank names to `Guest ` plus the first four ID characters; serialize room changes and send the welcome snapshot before subsequent updates. Use connection-scoped IDs; duplicate friendly names are allowed.
- [ ] Add failing contract cases: join two clients and compare snapshots; rename one; update media flags; close a connection; reconnect with a fresh ID; send malformed JSON without disrupting the other client.
- [ ] Add Go tests for abrupt disconnect expiry using an injected clock or short configurable intervals, using protocol ping/pong rather than JSON heartbeat messages. Assert removal within the configured timeout and no duplicate removal notification.
- [ ] Implement the room state and `github.com/coder/websocket`, keeping the reader active to process control frames. Serialize state changes using one clearly owned room state; no database or distributed pub/sub.
- [ ] Log each WebSocket connect/disconnect (including pre-join closures) with client IP and local connection address/port. Add a test asserting the required address fields are present.
- [ ] Run `cd backends/go && go test -race ./...`; expect no failures or races. Define `npm test` in `tests/contract` to compile the TypeScript tests with `tsc` and run their generated JavaScript using `node --test`. Against the running server, run `cd tests/contract && npm test`; expect all cases to pass.

## Task 4: Connect room participants to centralized media

**Files:** Create `backends/go/internal/media/{media.go,media_test.go,process.go,proxy.go}`, `web/src/{media.ts,room.ts}`, and extend `protocol/room.md` with the validated publish/read contract.

**Dependency:** Task 1's gate must pass (or the owner must explicitly narrow scope) before starting this task.

**Interfaces:** Consumes participant IDs/snapshots and Task 1's tested media endpoints. Produces session-associated stream IDs and publish/read descriptors consumed by the UI. Keep product-specific media requests in these adapter files.

- [ ] Add tests asserting distinct stream IDs for concurrent sessions, cleanup after departure, and an actionable failure when the media service is unavailable.
- [ ] Implement child startup/readiness, graceful termination/reaping, and an unavailable status after unexpected exit. A failed media start fails application startup; a runtime exit leaves room state available until the owner restarts the app.
- [ ] Add proxy tests for POST/PATCH/DELETE, request bodies/content types, response status/session headers, rewritten Location URLs, and cleanup. Bind media HTTP/control listeners to loopback and disable unused services.
- [ ] Implement stream association and `/media/` negotiation proxying with reachable LAN ICE candidates. Do not relay media payloads through the application's room WebSocket.
- [ ] Implement one browser publication and subscriptions to other participants; use local media for the self preview. Track stream readiness independently of announced participant state.
- [ ] Release subscriptions, publication sessions, and obsolete tracks on departure/reconnect. Show media errors without taking down the room connection.
- [ ] Repeat Task 1's cross-browser publish/subscribe checks through the real application. Interrupt the media child; verify unavailable status, then restart the application and verify recovery. Do not add automatic restart machinery.
- [ ] Run Go tests and the contract suite; expect passes. Document remaining device failures before treating this task as complete.

## Task 5: Implement the usable intercom UI

**Files:** Create `web/src/{tiles.ts,devices.ts,preferences.ts,styles.css}` and `tests/browser/intercom.md` and `scripts/test-browser`; extend `web/src/main.ts`.

**Interfaces:** Consumes room snapshots and media streams; produces the controls and permission/retry behavior specified by the design. Store the friendly name under `bigbrother.name` in local storage.

- [ ] Specify reproducible browser scenarios and implement `scripts/test-browser` using headless `agent-browser` commands supported by the installed version. Assert two participant tiles, remembered/edited names, muted self-preview, and local mute/hide controls not altering the sender. Save evidence under `tmp/test-results/`; fail/report pending explicitly instead of treating skipped checks as passed.
- [ ] Implement a responsive grid, labels, outgoing mic/video controls, active device labels, camera/microphone selection, and friendly-name editing.
- [ ] Handle denied permissions and blocked autoplay with an explicit Join/retry action. Test these states and a client with no camera or microphone.
- [ ] Handle device removal and switching failure while retaining the room UI. Verify successful switches stop obsolete tracks and use the selected device.
- [ ] Verify local hiding/muting affects only that receiver and that turning off one's own video/mic changes outgoing media.
- [ ] Run `scripts/test-browser` with synthetic media where supported; expect all automated assertions to pass. Explain any unsupported capability and request a fallback decision rather than silently adopting another browser tool.
- [ ] [DEVICE] Verify real audio, camera selection/removal, mobile orientation, background/resume, and six-tile layout on actual devices. Synthetic media does not satisfy these checks.

## Task 6: Validate offline delivery and measured limits

**Files:** Update `README.md`, `media/README.md`, `scripts/build`, and fill in the Acceptance evidence section of this implementation document. Store reusable discoveries in memories and index them.

**Interfaces:** Consumes the runnable distribution. Produces repeatable setup/start/stop instructions and a factual acceptance record.

- [ ] Document certificate trust setup, Linux/macOS distribution requirements, launch flags, web/media ports, and how to stop the application and its owned media process cleanly.
- [ ] Ensure startup reports missing media binaries/configuration and media startup failures clearly. Test shutdown leaves no owned child process running.
- [ ] [MANUAL] On a clean target environment with runtime dependencies already provisioned, coordinate disconnecting the internet and run the packaged app without source/build tools. Confirm all assets and media work locally.
- [ ] Verify explicit occupied-port errors and default-port fallback without elevated privileges. Confirm printed LAN URLs are usable from another device.
- [ ] [DEVICE] Test 1, 3, and 6 participants on available target hardware. Record CPU, memory, resolution/frame rate, network conditions, latency observations, and failures; do not invent universal capacity guarantees.
- [ ] Run Go tests, contract tests, and automated browser checks once against the final build. Record commands and results alongside the manual device matrix.
- [ ] Review version 1 against the self-contained design, explicitly listing deferred features and unverified devices. Update relevant memories/index entries with verified findings.

## Follow-up plans, not version 1 blockers

- [ ] Write a separate camera/recording feature design: persistent source IDs, camera setup, motion event or frame-analysis worker, no-motion interval, clip/snapshot duration, storage cleanup, and playback UI. Reuse the validated media path.
- [ ] Write an Elixir-backend plan once the first Go slice works and the protocol has settled. Reproduce health/join/rename/disconnect first, run the same contract suite, and add media integration afterward. Preserve idiomatic Elixir internals and shared external behavior.

## Acceptance evidence

Not executed yet. For each run, record date, command/scenario, tool/browser/OS versions, host architecture, expected result, observed result, and pass/fail/pending status. Link transient logs/screenshots when useful; retain the findings here so cleanup of `tmp/` does not erase the result.

| Check | Status | Evidence |
| --- | --- | --- |
| Media feasibility gate | Pending | Not run |
| Go unit/race tests | Pending | Not run |
| Backend contract tests | Pending | Not run |
| Headless agent-browser checks | Pending | Tool unavailable on PATH during plan revision; recheck before execution |
| Physical browser/device matrix | Pending | Not run |
| Offline packaged runtime | Pending | Not run |
| Target packaging and execution | Pending | Record each target separately |
| 1/3/6-participant measurements | Pending | Not run |
