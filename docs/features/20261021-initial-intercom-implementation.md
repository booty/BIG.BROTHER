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

- [x] Check required tools and access first: Go 1.27.1, Node 26.12.0/npm 11.20.0, agent-browser 0.39.1 and its help, MediaMTX v1.21.2 macOS ARM64 binary, OpenSSL 4.0.3, FFmpeg 8.1.2, installed browser apps, and available target devices. The CLI drives Chrome only; no Android/iOS device or physical camera was available, so those checks remain pending. Fake media, named sessions, and isolated publisher/reader sessions were verified.
- [x] Read current MediaMTX documentation for browser publishing/reading, control API, recording, and codecs. Pin v1.21.2 and document the supported packaged targets and release checksums in the memory and build script.
- [x] Create disposable publisher/subscriber pages and recording-enabled probe config under ignored `tmp/intercom-probe/`; record repeatable commands and assertions in its README.
- [x] Choose OpenSSL and document setup/reissue commands for `tls/server.crt` and `tls/server.key` in the config directory, including LAN hostname/IP coverage; test CA preservation during server-certificate reissue.
- [ ] [MANUAL] Trust the local CA on desktop and Android/iOS targets. Verify camera permission and audible playback after a user gesture; record platform-specific trust steps.
- [x] Create a repeatable headless Chrome `agent-browser` publish/subscribe scenario with explicit codec, response, URL, and track assertions and isolated client sessions. Record Firefox, Safari, Android, and iOS coverage as unsupported/pending rather than treating Chromium as those browsers.
- [x] Prefer H.264 plus Opus on the publisher, verify `video/H264` and `audio/opus` in browser connection statistics, and record/replay both browser-originated and synthetic RTSP-originated streams as fMP4. The MediaMTX helper requires lowercase `h264`; the uppercase value produced audio-only output and was corrected.
- [ ] Test fresh offline desktop Firefox and a provisioned OpenH264 profile separately. Record what browser setup must occur before offline operation.
- [ ] [DEVICE] Publish and receive real audio/video across desktop Chrome, Firefox, Safari, Android Chrome/Firefox, and iOS Safari. Record versions, codecs, and required settings.
- [x] Use an explicitly labeled synthetic FFmpeg RTSP source, display it in Chrome, record and replay a five-second clip, and repeat recording/replay with a browser-originated stream. The tested H.264 Baseline plus Opus source required no MediaMTX transcoding; physical camera conversion and target CPU cost remain unknown.
- [ ] [MANUAL] Coordinate disabling internet connectivity while retaining LAN connectivity and repeat publish/subscribe. Confirm no public connection service or CDN is required.
- [x] Record results, limitations, versions, checksums, and commands in the memory and index; claims are limited to verified local findings and unavailable device checks remain explicitly pending.
- [x] Validate same-origin WHIP/WHEP proxying: OPTIONS/POST/PATCH/DELETE, request content types, statuses, rewritten `Location` values, loopback WebRTC HTTP/Control API listeners, and advertised LAN ICE candidates all passed locally.
- [ ] [DEVICE] Verify the advertised LAN ICE candidate is reachable from another physical LAN client. Candidate advertisement passed; remote reachability was not tested.
- [x] Update the design with endpoint mappings, proxy headers/methods, ports, Control API readiness request, OpenSSL certificate setup/reissue commands, and publish/read descriptor fields for Task 4.
- [ ] **Decision gate:** If MediaMTX cannot pass after routine debugging, or success requires material transcoding cost/architectural changes, stop dependent work, present evidence and alternatives, and ask the owner to choose before continuing that work. Do not select a replacement silently.

**Pass condition:** Reproducible local media exchange, browser/camera recording and playback, offline behavior, and the listed target-browser/device evidence. Missing checks remain unchecked; independent portions of Tasks 2–3 may proceed (packaging/setup steps still need their specific probe outputs), but Task 4 waits for this gate or explicit scope narrowing by the owner. Do not infer compatibility from desktop-only results.

## Task 2: Deliver a launchable Go shell with local assets

**Files:** Create `backends/go/go.mod`, `backends/go/cmd/bigbrother/main.go`, `backends/go/internal/server/{server.go,server_test.go}`, `backends/go/internal/launch/{launch.go,launch_test.go}`, `web/{package.json,tsconfig.json,index.html}`, `web/src/main.ts`, `scripts/build`, `scripts/setup-tls`, and `README.md`.

**Interfaces:** Produces `GET /healthz`, static frontend serving, the design's CLI flags, and a documented `scripts/build` entry point. Build output goes in ignored `tmp/build/OS-ARCH/`; frontend artifacts are bundled with the runnable distribution. `scripts/setup-tls` implements the documented provisioning-tool commands from Task 1; it never silently installs device trust or replaces an existing CA.

- [x] Add Go tests for health JSON, local assets, explicit-port failure, 443-to-8443 fallback on permission/address-in-use errors, missing/invalid TLS files, default config lookup, `--config-dir`, paired certificate overrides, and one-sided override failure. Candidate ports are injected in tests.
- [x] Implement the HTTPS server, CLI parsing, port selection, and startup URL reporting with application logic outside `main.go`.
- [x] Add esbuild bundling, a separate `tsc --noEmit` check, and an offline static page; pin dependencies and retain lockfiles.
- [x] Implement `scripts/build --target OS/ARCH` for all three targets. Each package contains its own pinned MediaMTX v1.21.2 asset, config, license, and local assets; release checksums are verified at build time and no component is downloaded at launch.
- [x] Implement `scripts/setup-tls` with OpenSSL, preserving the CA across repeat runs and requiring `--replace-server-cert` before replacing server certificates; document device trust as a separate manual step.
- [x] Run Go unit/race tests and all three target builds. Darwin ARM64 executed on this host; Linux ARM64 and AMD64 were cross-packaged and architecture-checked, not run.
- [x] After one-time setup in a disposable `--config-dir`, launch the packaged app on default 443 and explicit port 18445 without certificate override flags; verify the page and health response using the generated CA. The actual user profile was not modified. Document ports and distribution launch.

## Task 3: Define and implement the room contract

**Files:** Create `protocol/room.md`, `tests/contract/{package.json,room.test.ts}`, `backends/go/internal/room/{room.go,room_test.go}`, and `backends/go/internal/server/websocket.go`.

**Interfaces:** Consumes the HTTP shell; produces `/ws`, `join`, `rename`, `mediaState`, `welcome`, `participants`, and `error` behavior defined in the design. Contract tests target `BASE_URL`, with optional `CA_CERT` for the local CA, and never import Go code.

- [x] Write the protocol examples and specify name fallback, session identity, message errors, ping/pong deadlines, and snapshot ordering. Blank names default to `Guest ` plus four ID characters; room changes are serialized; welcome precedes later snapshots; IDs are connection-scoped.
- [x] Add contract cases for two-client snapshots, rename, media flags, close, fresh-ID reconnect, and malformed JSON without disrupting the other client.
- [x] Add short-interval Go tests for ping/pong disconnect expiry and duplicate-removal prevention without JSON heartbeat messages.
- [x] Implement room state and `github.com/coder/websocket`; keep the reader active for control frames and use one in-memory owner for room state.
- [x] Log connect/disconnect events, including pre-join closures, with client IP and local address/port; test the required fields.
- [x] Run `go test -race ./...` and the compiled Node contract suite against the packaged HTTPS server; both passed.

## Task 4: Connect room participants to centralized media

**Files:** Create `backends/go/internal/media/{media.go,media_test.go,process.go,proxy.go}`, `web/src/{media.ts,room.ts}`, and extend `protocol/room.md` with the validated publish/read contract.

**Dependency:** Task 1's gate must pass (or the owner must explicitly narrow scope) before starting this task. On 2026-10-21 the owner explicitly narrowed the Task 4 gate to the verified synthetic Chrome/RTSP and local proxy results; physical-device trust, browser-matrix, offline-LAN, and remote ICE checks remain pending.

**Interfaces:** Consumes participant IDs/snapshots and Task 1's tested media endpoints. Produces session-associated stream IDs and publish/read descriptors consumed by the UI. Keep product-specific media requests in these adapter files.

- [x] Add tests asserting distinct stream IDs for concurrent sessions, cleanup after departure, and an actionable failure when the media service is unavailable.
- [x] Implement child startup/readiness, graceful termination/reaping, and an unavailable status after unexpected exit. A failed media start fails application startup; a runtime exit leaves room state available until the owner restarts the app.
- [x] Add proxy tests for OPTIONS/POST/PATCH/DELETE, request bodies/content types, response status/session headers, rewritten Location URLs, and cleanup. Bind media HTTP/control listeners to loopback and disable unused services.
- [x] Implement stream association and `/media/` negotiation proxying with reachable LAN ICE candidates. Do not relay media payloads through the application's room WebSocket.
- [x] Implement one browser publication and subscriptions to other participants; use local media for the self preview. Track stream readiness independently of announced participant state.
- [x] Release subscriptions, publication sessions, and obsolete tracks on departure/reconnect. Show media errors without taking down the room connection.
- [x] Interrupt the packaged MediaMTX child; `/healthz` stayed 200, `/media/status` became unavailable, and new signaling returned 503. Restarting the app restored media readiness and the room contract. No automatic child restart was added.
- [ ] Repeat Task 1's publish/subscribe checks across browsers and devices through the real application. The repeatable synthetic Chrome two-client scenario passes; Firefox, Safari, Android, iOS, trusted device certificates, and remote LAN ICE reachability remain pending.
- [x] Run Go race tests and the contract suite. Document remaining device failures before treating this task as complete.

## Task 5: Implement the usable intercom UI

**Files:** Create `web/src/{tiles.ts,devices.ts,preferences.ts,styles.css}` and `tests/browser/intercom.md` and `scripts/test-browser`; extend `web/src/main.ts`.

**Interfaces:** Consumes room snapshots and media streams; produces the controls and permission/retry behavior specified by the design. Store the friendly name under `bigbrother.name` in local storage.

- [x] Specify reproducible browser scenarios and implement `scripts/test-browser` using headless `agent-browser` commands supported by the installed version. Assert two participant tiles, remembered/edited names, muted self-preview, and local mute/hide controls not altering the sender. Save evidence under `tmp/test-results/`; fail/report pending explicitly instead of treating skipped checks as passed.
- [x] Implement a responsive grid, labels, outgoing mic/video controls, active device labels, camera/microphone selection, and friendly-name editing.
- [x] Handle denied permissions and blocked autoplay with an explicit Join/retry action. Test these states and a client with no camera or microphone; the synthetic client without a camera successfully publishes audio.
- [x] Handle browser track-ended/device-change events and switching failure while retaining the room UI. Synthetic tests verify successful selection stops obsolete tracks and failed camera selection keeps existing room media; physical device removal remains pending below.
- [x] Verify local hiding/muting affects only that receiver and that turning off one's own video/mic changes outgoing media.
- [x] Run `scripts/test-browser` with synthetic media where supported; all automated assertions passed on the final packaged Darwin ARM64 build with agent-browser 0.39.1 / Chrome for Testing 156.0.8078.47. Firefox/Safari/mobile remain pending; no alternate automation tool was substituted.
- [ ] [DEVICE] Verify real audio, camera selection/removal, mobile orientation, background/resume, and six-tile layout on actual devices. Synthetic media does not satisfy these checks.

## Task 6: Validate offline delivery and measured limits

**Files:** Update `README.md`, `media/README.md`, `scripts/build`, and fill in the Acceptance evidence section of this implementation document. Store reusable discoveries in memories and index them.

**Interfaces:** Consumes the runnable distribution. Produces repeatable setup/start/stop instructions and a factual acceptance record.

- [x] Document certificate trust setup, Linux/macOS distribution requirements, launch flags, web/media ports, offline runtime dependencies, and how to stop the application and its owned media process cleanly.
- [x] Ensure startup reports missing media binaries/configuration and media startup failures clearly. SIGINT shutdown of the packaged app was verified to remove its owned MediaMTX child.
- [ ] [MANUAL] On a clean target environment with runtime dependencies already provisioned, coordinate disconnecting the internet and run the packaged app without source/build tools. Confirm all assets and media work locally.
- [ ] Verify explicit occupied-port errors and default-port fallback without elevated privileges. Unit tests cover injected occupied-port and fallback errors; a real unprivileged bind and LAN URL reachability from another device remain pending.
- [ ] [DEVICE] Test 1, 3, and 6 participants on available target hardware. Record CPU, memory, resolution/frame rate, network conditions, latency observations, and failures; do not invent universal capacity guarantees.
- [x] Run Go tests, contract tests, and automated browser checks once against the final build. Record commands and results alongside the manual device matrix.
- [x] Review version 1 against the self-contained design, explicitly listing deferred features and unverified devices. Update relevant memories/index entries with verified findings.

## Follow-up plans, not version 1 blockers

- [ ] Write a separate camera/recording feature design: persistent source IDs, camera setup, motion event or frame-analysis worker, no-motion interval, clip/snapshot duration, storage cleanup, and playback UI. Reuse the validated media path.
- [ ] Write an Elixir-backend plan once the first Go slice works and the protocol has settled. Reproduce health/join/rename/disconnect first, run the same contract suite, and add media integration afterward. Preserve idiomatic Elixir internals and shared external behavior.

## Acceptance evidence

Evidence collected on 2026-10-21. The machine was macOS 15.7.9, Darwin ARM64; Go 1.27.1, Node 26.12.0/npm 11.20.0, agent-browser 0.39.1 with Chrome for Testing 156.0.8078.47, FFmpeg 8.1.2, and MediaMTX v1.21.2. The durable local media findings and checksums are in the [feasibility memory](../memories/20261021-media-browser-and-recording-feasibility.md).

| Check | Status | Evidence |
| --- | --- | --- |
| Media feasibility gate | Narrowed by owner for Task 4; target checks pending | Synthetic Chrome 156 WHIP/WHEP through the packaged Go HTTPS proxy; two clients decoded H.264 Baseline + Opus, each had one reader, and departure removed publication/reader sessions. Earlier browser and synthetic RTSP recordings replayed as five-second fMP4. Physical device trust/browser matrix, remote LAN ICE reachability, and internet-disconnected LAN check remain pending. See memory and browser scenario. |
| Go unit/race tests | Pass | 2026-10-21: `GOCACHE=/Users/booty/code/BIG.BROTHER/tmp/go-build-cache GOMODCACHE=/Users/booty/code/BIG.BROTHER/tmp/go-mod-cache go test -race ./...` from `backends/go`; Go 1.27.1, Darwin ARM64. |
| Backend contract tests | Pass | 2026-10-21: `BASE_URL=https://localhost:18445 CA_CERT=<disposable-config>/tls/ca.crt npm test` against packaged Darwin ARM64 app before and after its MediaMTX restart; Node 26.12.0; 1/1 test passed each run. |
| Headless agent-browser checks | Pass for automated synthetic Chrome scenarios | 2026-10-21: `scripts/test-browser` passed name persistence/editing, muted self-preview, blocked-autoplay retry, two decoded participant tiles, receiver-only mute/hide, outgoing mic/video controls, selected-device replacement and obsolete-track cleanup, failed camera-switch retention, denied-permission retry, and audio-only operation without a camera. `scripts/test-media-browser` also passed H.264/Opus exchange and departure cleanup. Both ran against the packaged Darwin ARM64 app with agent-browser 0.39.1 / Chrome for Testing 156.0.8078.47 and `--ignore-https-errors`; physical devices, certificate trust, other engines, and mobile remain pending. JSON evidence is under ignored `tmp/test-results/`. |
| Packaged shutdown | Pass | 2026-10-21: SIGINT to the packaged Darwin ARM64 app stopped the application and its owned MediaMTX child; process inspection found neither exact PID afterward. |
| Port fallback and LAN reachability | Partial | Launch unit tests cover explicit occupied-port errors and fallback after injected bind errors. This host run used an explicit test port; real unprivileged fallback and another-device LAN URL reachability remain pending. |
| Physical browser/device matrix | Pending | No physical camera, Android, or iOS device was available; installed agent-browser supported Chrome only for this workflow. Desktop Firefox/Safari and mobile checks are not claimed. |
| Offline packaged runtime | Pending | Matching media binaries and browser assets are bundled and no launch-time download is used, but internet was not disconnected while retaining LAN access. |
| Target packaging and execution | Partial | After the final UI implementation, `scripts/build --target linux/arm64`, `linux/amd64`, and `darwin/arm64` passed with checksum-verified MediaMTX v1.21.2 assets. `file` verified all architectures; packaged Darwin ARM64 executed on this host. Linux packages were not run. |
| 1/3/6-participant measurements | Pending | Not run; no target hardware or physical participant devices were available. |
