# BIG.BROTHER

BIG.BROTHER is a small home intercom. The Go backend serves room state over HTTPS, launches the bundled MediaMTX v1.21.2 process, and proxies browser WHIP/WHEP signaling on the same HTTPS origin. Browser media travels directly over LAN ICE/UDP; video does not pass through the room WebSocket. The current page joins with a friendly name, shows participant video tiles, and uses a muted local preview.

## Environment Setup

To build BIG.BROTHER from source, install Go 1.22 or newer and Node.js 18 or newer (npm is included with Node.js). On macOS with [Homebrew](https://brew.sh/), install them with:

```sh
brew install go node
```

On Linux, install Go and Node.js using their official instructions, and install the build utilities with your distribution's package manager. For Debian or Ubuntu:

```sh
sudo apt install curl openssl tar coreutils gawk
```

The build also needs `curl`, `tar`, `awk`, and either `sha256sum` or `shasum` to download and verify MediaMTX. OpenSSL is used to create TLS certificates. macOS includes these command-line utilities; on Linux, install their equivalents if they are missing. Check the tool versions and availability with:

```sh
go version
node --version
npm --version
openssl version
```

Install Go from [go.dev](https://go.dev/doc/install) and Node.js from [nodejs.org](https://nodejs.org/en/download). These tools are needed to build from source; a built package includes the browser assets and MediaMTX and does not need Go, Node.js, npm, or OpenSSL at runtime.

## Build and launch

For a local browser test on an Apple Silicon Mac, run these commands from the repository root:

```sh
scripts/build --target darwin/arm64
scripts/setup-tls --host localhost --ip 127.0.0.1 --config-dir tmp/bigbrother-local
tmp/build/darwin-arm64/bigbrother --config-dir tmp/bigbrother-local --port 8443
```

Trust `tmp/bigbrother-local/tls/ca.crt` on the Mac, then open <https://localhost:8443>. The server prints its listening URLs when it starts. Press Ctrl-C in the terminal to stop it.

To make the server available to other home devices, use a hostname and LAN IP that those devices can resolve and reach, and let the app use its default configuration directory:

```sh
scripts/setup-tls --host intercom.local --ip 192.168.1.20
tmp/build/darwin-arm64/bigbrother
```

Replace the example hostname and IP with your LAN values. Install the generated `ca.crt` on each client device, then open one of the HTTPS URLs printed by the server. The package targets supported by `scripts/build` are `darwin/arm64`, `linux/arm64`, and `linux/amd64`; use the target matching the server machine.

## Development

Requirements for the current shell are Go 1.22 or newer, Node.js with npm, and OpenSSL. Build the browser assets with `cd web && npm ci && npm run build && npm run typecheck`. Run backend unit tests with `cd backends/go && go test ./...`. The backend-independent room contract suite is in `tests/contract`; its test command expects a running backend.

## TLS and networking

`setup-tls` uses OpenSSL to create a local CA and server certificate under the platform's `bigbrother/tls` configuration directory (or the directory passed with `--config-dir`). Install `ca.crt` on each client device separately. If the server's LAN name or address changes, reissue only the server certificate while preserving the CA:

```sh
scripts/setup-tls --host intercom.local --ip 192.168.1.21 --replace-server-cert
```

The application does not create certificates or install trust at startup. `scripts/build` packages the Go executable, browser assets, MediaMTX v1.21.2, and its config under `tmp/build/OS-ARCH/`. It downloads the matching MediaMTX release archive and verifies its SHA-256; the packaged runtime does not download anything. See [media setup and ports](media/README.md).

By default, the server tries port 443, then 8443 through 8453, and finally an OS-assigned port. An explicit port such as `--port 1234` fails if it cannot bind. Use `--config-dir PATH` to select another configuration directory; `--tls-cert PATH` and `--tls-key PATH` override the default certificate pair and must be supplied together. Linux may restrict binding port 443 for an unprivileged user. Keep the chosen TCP port available for HTTPS and allow LAN UDP 8189 for WebRTC media. Startup waits for the bundled media service's loopback Control API before serving the app; a media startup failure stops application startup with an error.

Stop the application with Ctrl-C; it asks MediaMTX to close outstanding sessions, sends a termination signal, and reaps the child. `/healthz` reports application liveness, while `GET /media/status` reports media availability. If MediaMTX exits unexpectedly, room state remains available and new media signaling requests return 503; restart the app to recover. Media listeners use loopback TCP 8889 for signaling and API 9997, plus LAN UDP 8189 for WebRTC ICE. Browser/device certificate trust, offline operation, remote-device ICE reachability, and non-Chromium/mobile behavior remain pending; see the implementation plan's acceptance evidence and the [browser scenario](tests/browser/media-integration.md).
