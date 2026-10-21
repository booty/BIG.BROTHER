# Initial intercom design

Status: implementation baseline; media-specific details remain subject to the explicit feasibility gate.

## Purpose and scope

Build a simple, single-room video intercom for the owner and housemates. This document is the current requirements source; the spitballing notes are historical and are not required reading. Architectural rationale lives in the [ADR](../adrs/20261021-go-first-backends-and-centralized-media.md).

Prioritize simplicity of use. There is no application authentication: anyone on the home Wi-Fi or Ethernet network can join. This is not a commercial or mission-critical service. Typical use is 1–3 clients, occasionally around 6; impose no artificial participant cap, but do not promise unlimited hardware capacity.

Target a Raspberry Pi 5 running 64-bit Linux or an AMD A9 Linux netbook; an available 7th-generation Intel i5 Linux laptop is a fallback if measurements require it. Support macOS development. Browser targets are current desktop Chrome, Firefox, and Safari; Chrome/Firefox on Android; and Safari on iOS. Record tested versions rather than claiming every historical version works.

Version 1 includes browser audio/video, participant tiles, friendly names, device selection, local playback controls, connection logging, offline operation, and a documented launch/setup procedure. Prove camera ingestion and recording in an early feasibility probe, but defer their product UI and motion-trigger rules to a follow-up feature. Elixir is a subsequent implementation, not a prerequisite for shipping Go.

## Components

- `web/`: one TypeScript frontend, built into locally served static assets.
- `backends/go/`: CLI, static serving, participant state, application events, and media-service integration.
- `backends/elixir/`: reserved for a later counterpart; do not scaffold an empty implementation.
- `protocol/`: language-neutral HTTP/event behavior and examples.
- `tests/contract/`: TypeScript tests against any running backend's base URL.
- `media/`: pinned media-server configuration and distribution/setup instructions.

Browsers send application messages to the Go backend and media to the shared media server. Go launches one owned MediaMTX child, waits for readiness, and stops/reaps it on shutdown. If media exits unexpectedly, retain the room UI with an unavailable status; restarting the application is the initial recovery path. Do not build a process supervisor/restart framework.

Bind MediaMTX's WebRTC HTTP negotiation and enabled control API listeners to loopback. Disable unused services. Go proxies browser WHIP/WHEP requests under `/media/` through its HTTPS origin, stripping that prefix upstream and rewriting response `Location` URLs back to the same origin/prefix. Preserve request bodies, content types, status codes, and session headers for POST, PATCH, and DELETE; include other methods required by the tested client. Test creation, ICE updates, and teardown, not just the initial POST.

Expose the WebRTC ICE UDP listener on the LAN and advertise reachable LAN candidates; no public STUN/TURN is required. UDP-only is the initial arrangement to validate, not a permanent restriction. A later TCP fallback or RTSP listener for headless publishers is allowed when needed. Camera pull connections can be outbound. The exact media ports and endpoint mapping are probe outputs; verify them before dependent product work.

### Selected tools and build targets

Use Go's `net/http` and `github.com/coder/websocket`; TypeScript with esbuild for browser bundles and `tsc --noEmit` for type checking; and `node:test` for backend-independent contract tests. Compile TypeScript contract tests with `tsc` before running Node's test runner. Use headless `agent-browser` for supported browser checks, following AGENTS.md. Do not add Playwright as a parallel default. Version numbers are pinned in module/package lockfiles when installed; tool selection is settled, while MediaMTX adoption still depends on the probe.

`scripts/build --target OS/ARCH` must support `linux/arm64`, `linux/amd64`, and `darwin/arm64`, packaging the matching Go executable, MediaMTX binary, config, and browser assets under `tmp/build/OS-ARCH/`. Add `darwin/amd64` only if Intel Mac support is requested. Build-time downloads are permitted; launch-time downloads are not. Cross-compilation/package success is not evidence of execution on that platform.

## Application interface

The room contract below is fixed for the first implementation. Media descriptors are finalized by the probe. A blank name becomes `Guest ` plus the first four session-ID characters; send `welcome` before subsequent snapshots on that connection:

- `GET /healthz`: returns HTTP 200 and JSON `{ "status": "ok" }` when the application HTTP service is available. This is liveness, not a promise that media is healthy.
- `GET /ws`: WebSocket connection for participant state. Messages use `{ "type": "...", "payload": { ... } }`.
- Client `join`: `{ "name": string }`; server allocates a session ID. Friendly names are labels, not unique identities.
- Server `welcome`: `{ "selfId": string, "participants": Participant[] }`.
- Client `rename`: `{ "name": string }`; client `mediaState`: `{ "audioEnabled": boolean, "videoEnabled": boolean }`.
- Server `participants`: `{ "participants": Participant[] }`, a full snapshot after changes. For this room size, snapshots keep reconnection simple.
- `Participant`: `{ "id": string, "name": string, "audioEnabled": boolean, "videoEnabled": boolean, "streamId": string }`. Media flags describe the sender's announced state; stream readiness is determined separately.
- Server `error`: `{ "message": string }` for malformed/unsupported application messages; a bad message must not stop other clients.

Use connection-scoped IDs, not IP addresses, so multiple tabs/devices behind one address remain distinct. A dropped session is removed when closed or when a heartbeat timeout expires. Use server-initiated WebSocket protocol ping/pong, with a 10-second ping interval and a 30-second pong deadline; tests should use configurable shorter timings. Do not add an application-level heartbeat message. Keep the connection reader active so control frames are processed. Reconnecting creates a new session and replaces the frontend's stale snapshot.

The application assigns opaque stream IDs and exposes publish/read information through a small media adapter. Record the exact media negotiation endpoints after the probe; do not invent a generic multi-provider plugin API. A failed media connection must be visible in its tile and must not masquerade as an active video feed.

## Browser behavior

- Show all joined participants, including a locally previewed, always-muted self tile; fill the available viewport with a responsive grid.
- Attempt joining/capture with existing permission, but provide a clear Join action when permissions or autoplay require interaction. A denial must leave useful status and a retry path.
- Persist a friendly name in browser local storage. Without one, show a session-derived label such as `Guest ab12`; no database is required for version 1.
- On the self tile: edit name, mute outgoing microphone, stop outgoing video, show active device labels, and select camera/microphone through a settings button. Use a pencil edit control next to the friendly name.
- On another tile: mute audio or hide video locally, without changing that sender or other receivers. Do not promise bandwidth savings for hiding a tile.
- Handle absent/disconnected devices and failed device changes without losing the room UI. Stop obsolete local tracks after successful replacement and on exit.
- Keep remote audio separate from the muted local preview to avoid self-echo.

Initial mobile support means foreground browser use. Test backgrounding, locking, and resuming; do not promise continuous background camera/microphone operation.

## Launch and offline operation

Normal launch after setup: `bigbrother` or `bigbrother --port 1234`. Optional overrides: `--config-dir PATH`, `--tls-cert PATH`, and `--tls-key PATH`; require the certificate/key overrides together.

Use `bigbrother` beneath the platform's user configuration directory (Go `os.UserConfigDir`) by default, with `tls/server.crt` and `tls/server.key` inside it. A documented one-time setup procedure provisions a local CA and a server certificate valid for the chosen LAN hostname/IP, then explains how to trust the CA on client devices. Normal startup never silently creates/replaces a CA or installs trust. Missing/invalid files produce an actionable setup error. The probe selects and documents the certificate provisioning tool; building our own certificate manager is out of scope. Explain reissuing the server certificate when its address changes without replacing the CA.

With no explicit port, try 443, then 8443, then 8444–8453, finally an OS-assigned port. An explicit port must bind or fail with a helpful error; never silently substitute another. Fall back on permission/address-in-use errors, not on unrelated configuration failures. Never elevate automatically or change OS network policy. Linux commonly restricts low ports, depending on capabilities/system configuration; handle actual bind results instead of assuming OS-wide behavior. An HTTP redirect listener is deferred.

Print usable complete LAN `https://` URLs with selected ports, not just `0.0.0.0`. Document additional media ports separately. Log WebSocket connect/disconnect events to stdout with client IP and local host IP/port for that connection, including connections that close before joining. Never log private keys or full configuration dumps.

Setup may use the internet, but normal operation must not. Bundle every browser asset and the selected host's media binary. Do not fetch missing runtime components on launch. Report missing binaries/configuration and media startup failures clearly. Firefox's WebRTC H.264 support may require downloading OpenH264 during browser setup; verify its availability before offline use and document provisioning limitations. Do not equate ordinary H.264 video-file playback with WebRTC encoding support.

## Near-term extension boundary

IP cameras and headless publishers become additional stream sources. A later source model must distinguish persistent cameras from ephemeral browser participants; do not force cameras to impersonate WebSocket sessions.

Motion detection emits events to application policy, which triggers recording or snapshots through the media component. Prefer compatible camera encodings and camera-supplied motion events where available. Otherwise evaluate low-rate frame analysis as a separate worker. The follow-up must implement: after no motion for B minutes, motion triggers C minutes of video and/or snapshots D seconds apart, with a web UI to view the result. Exact settings and storage cleanup policy belong to that feature's design.

Later features include an admin UI for friendly names/settings, server-persisted text chat, and freely movable/resizable tiles. Continuous 24/7 recording is out of scope. No persistent chat, admin UI, arbitrary tile placement, authentication, distributed services, or database is needed in version 1. No hard participant cap initially; measure the six-client case and document limits.

## Validation and unresolved feasibility

Before dependent media integration, prove: trusted HTTPS on target devices, browser publish/subscribe across desktop and mobile targets, LAN-only connections, one real or representative RTSP source, a playable recorded clip, and the cost of required codec conversion. Explicitly probe H.264 video plus Opus audio for browser publication, verify the actual negotiated codec using connection statistics, record it, and replay it. MediaMTX's documented recording formats do not include VP8; do not assume browser defaults produce recordable media. Codec preferences cannot make an unavailable codec exist. Test Firefox with a fresh offline profile and with OpenH264 provisioned, documenting the difference. Do not silently introduce transcoding: measure any need and bring a material architecture/cost change back to the owner.

Use headless `agent-browser` for supported repeatable browser scenarios. If unavailable, report that and leave those checks pending; do not silently substitute another browser automation tool. Keep non-browser unit/contract tests in their ordinary runners. Label physical-device and human-interaction checks and preserve pending status. The feasibility gate requires evidence for the media path and target browser/device checks; continue independent work when access is missing, but do not begin dependent media integration or claim platform support until the gate passes or the owner explicitly narrows scope.

Do not claim cross-browser or low-power performance support from API availability alone. Record device/browser/OS versions, codecs, resolutions, and observed failures. Lack of target hardware is a pending manual check, not a passing result.

## Technical references

- [Go configuration directories](https://pkg.go.dev/os#UserConfigDir)
- [coder/websocket](https://github.com/coder/websocket)
- [esbuild](https://esbuild.github.io/getting-started/)
- [Node test runner](https://nodejs.org/api/test.html)
- [MediaMTX recording formats](https://mediamtx.org/docs/features/record)
- [MediaMTX reverse-proxy paths](https://mediamtx.org/docs/features/expose-the-server-in-a-subfolder)
- [Firefox OpenH264 setup](https://support.mozilla.org/en-US/kb/how-stop-firefox-making-automatic-connections)
