# MediaMTX

The distribution pins MediaMTX v1.21.2. `scripts/build` downloads only the matching release archive, verifies its published SHA-256, and includes the binary, license, and [configuration](mediamtx.yml) in `tmp/build/OS-ARCH/`. The launched application must not download media binaries or browser assets.

The packaged configuration is deliberately narrow:

- MediaMTX WebRTC HTTP signaling listens on `127.0.0.1:8889`; the Go backend proxies browser WHIP and WHEP negotiation under its HTTPS `/media/` path. It must preserve request bodies, content types, status/session headers, and rewrite session `Location` URLs under `/media/`.
- The WebRTC ICE listener is UDP `:8189` and advertises host-interface addresses. Permit this port on the home LAN. No public STUN or TURN service is configured. The probe observed both loopback and a private LAN candidate; it did not verify connectivity from another physical device.
- The Control API listens on loopback at `127.0.0.1:9997` for local readiness and diagnostics.
- The Go app starts MediaMTX as an owned child, waits for `GET /v3/paths/list` to return 200, and only then begins serving HTTPS. Missing binaries/configuration and readiness failures are startup errors. Ctrl-C closes tracked WHIP/WHEP sessions, sends MediaMTX SIGTERM, waits for the child, and kills it only if the graceful shutdown deadline expires. A browser or server DELETE that races with an already-closed WebRTC session can receive 404; that response is preserved and treated as completed cleanup.
- `GET /healthz` is application liveness. `GET /media/status` independently reports MediaMTX state. An unexpected child exit leaves room WebSockets available, changes media status to unavailable, and makes new `/media/` signaling requests return 503. Restart the application to recover.
- Recording and playback are disabled in the shipped config. The probe used fMP4 recording and replay successfully; continuous recording remains out of scope.
- RTSP, RTMP, HLS, SRT, and Media-over-QUIC listeners are disabled because version 1 publishes from browsers. Camera ingestion remains a follow-up.

The probe used MediaMTX's built-in browser publisher and reader pages as well as the same-origin proxy path. The measured browser path was H.264 Baseline (no B-frames) plus Opus. The app's browser publisher prefers H.264 and subscribes with WHEP after the room announces `streamReady`; media packets use ICE/UDP directly. `webrtcEncryption` stays off because the app provides HTTPS for signaling, while WebRTC media itself remains encrypted.

## TLS setup

Choose the LAN name and address clients will use, then run from the repository root:

```sh
scripts/setup-tls --host intercom.local --ip 192.168.1.20
```

Install the generated `ca.crt` on each client device separately. If the LAN address changes, reissue only the server certificate while preserving the CA:

```sh
scripts/setup-tls --host intercom.local --ip 192.168.1.21 --replace-server-cert
```

Normal launch reads `tls/server.crt` and `tls/server.key` under the platform's `bigbrother` configuration directory. Trust was not installed on physical desktop, Android, or iOS devices during this probe. The macOS `agent-browser` CLI does not support its `--ca-cert` option for local Chrome, so browser smoke checks used `--ignore-https-errors`; that does not count as certificate-trust validation.

## Probe limits

The repeatable local synthetic-media commands and recorded results are in [`docs/memories/20261021-media-browser-and-recording-feasibility.md`](../docs/memories/20261021-media-browser-and-recording-feasibility.md). The checked-in `mediamtx.yml` is validated against v1.21.2; the probe's recording-enabled config and publisher pages are disposable under ignored `tmp/intercom-probe/`.

The repeatable packaged-app browser scenario is [`scripts/test-media-browser`](../scripts/test-media-browser), with its prerequisites and limits in [`tests/browser/media-integration.md`](../tests/browser/media-integration.md). It passed with synthetic media in headless Chrome on this host. Real camera/microphone behavior, trusted CA installation, remote LAN ICE reachability, offline operation, Firefox/Safari, Android, and iOS checks remain pending.

The default configuration directory follows Go's `os.UserConfigDir`: macOS uses
`~/Library/Application Support/bigbrother` even when `XDG_CONFIG_HOME` is set;
Linux uses `$XDG_CONFIG_HOME/bigbrother`, falling back to `~/.config/bigbrother`.
Use `--config-dir` on both setup and launch to choose a different directory.
Run `sh tests/scripts/setup-tls-paths.sh` to check the setup script's platform defaults.
