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

For this machine, use:

```sh
bin/setup
bin/launch
```

`bin/setup` installs npm dependencies, downloads Go modules and the pinned
MediaMTX release as needed, and builds the application for the host platform.
It also creates localhost TLS certificates under `tmp/bigbrother-local/tls`
on first setup and reuses them on later runs. Go, Node.js/npm, and OpenSSL must
already be installed (see Environment Setup above).
`bin/launch` runs setup first and then starts the server in the foreground;
Ctrl-C stops the server and its media process. It uses `tmp/bigbrother-local`
as its configuration directory by default. Both commands work from any working
directory. Explicit server options override the defaults, for example:

```sh
bin/launch --config-dir tmp/bigbrother-local --port 8443
```

Setup checks content fingerprints and expected output files, skipping dependency
installation and each build stage when current. An unchanged run needs no network
access. Source edits, deletions, dependency manifest changes, tool version changes,
and missing outputs trigger the relevant work. State is recorded only after a
stage succeeds, so failed work is retried. To force all stages to run again,
remove `web/node_modules` and `tmp/build` and run `bin/setup`.

Trust the generated `tmp/bigbrother-local/tls/ca.crt` on the Mac/browser as a
separate, one-time step. For a local browser test on port 8443, run:

```sh
bin/setup
bin/launch --port 8443
```

Open <https://localhost:8443>. The server prints its listening URLs when it starts. Press Ctrl-C in the terminal to stop it.

To make the server available to other home devices, provision a certificate for
your LAN hostname/IP in a separate configuration directory:

```sh
scripts/setup-tls --host intercom.local --ip 192.168.1.20 --config-dir tmp/bigbrother-lan
bin/launch --config-dir tmp/bigbrother-lan
```

Replace the example hostname and IP with your LAN values. Install the generated `ca.crt` on each client device, then open one of the HTTPS URLs printed by the server. Supported hosts are `darwin/arm64`, `linux/arm64`, and `linux/amd64`. To build for a different machine, use `scripts/build --target linux/arm64` (or another supported target); the package is written to `tmp/build/OS-ARCH/`.

## Development

Requirements for the current shell are Go 1.22 or newer, Node.js with npm, and OpenSSL. Build the browser assets with `cd web && npm ci && npm run build && npm run typecheck`. Run backend unit tests with `cd backends/go && go test ./...`. The backend-independent room contract suite is in `tests/contract`; its test command expects a running backend.

Run `sh tests/scripts/setup-launch.sh` after `bin/setup` to exercise setup caching,
change detection, failure recovery, and launch argument forwarding with real
build tools in a disposable copy, including localhost certificate creation,
reuse, and explicit configuration/TLS overrides. It reuses the downloaded Go/MediaMTX caches;
npm may need network access if its package cache has been cleared.

## TLS and networking

`setup-tls` uses OpenSSL to create a local CA and server certificate under the platform's `bigbrother/tls` configuration directory (or the directory passed with `--config-dir`). Install `ca.crt` on each client device separately. If the server's LAN name or address changes, reissue only the server certificate while preserving the CA:

```sh
scripts/setup-tls --host intercom.local --ip 192.168.1.21 --config-dir tmp/bigbrother-lan --replace-server-cert
```

The packaged Go executable loads pre-provisioned certificates; the source-checkout
`bin/launch` wrapper invokes setup to create localhost certificates when needed.
CA trust is installed manually. `scripts/build` packages the Go executable, browser assets, MediaMTX v1.21.2, and its config under `tmp/build/OS-ARCH/`. It downloads the matching MediaMTX release archive and verifies its SHA-256; the packaged runtime does not download anything. See [media setup and ports](media/README.md).

By default, the server tries port 443, then 8443 through 8453, and finally an OS-assigned port. An explicit port such as `--port 1234` fails if it cannot bind. Use `--config-dir PATH` to select another configuration directory; `--tls-cert PATH` and `--tls-key PATH` override the default certificate pair and must be supplied together. Linux may restrict binding port 443 for an unprivileged user. Keep the chosen TCP port available for HTTPS and allow LAN UDP 8189 for WebRTC media. Startup waits for the bundled media service's loopback Control API before serving the app; a media startup failure stops application startup with an error.

Stop the application with Ctrl-C; it asks MediaMTX to close outstanding sessions, sends a termination signal, and reaps the child. `/healthz` reports application liveness, while `GET /media/status` reports media availability. If MediaMTX exits unexpectedly, room state remains available and new media signaling requests return 503; restart the app to recover. Media listeners use loopback TCP 8889 for signaling and API 9997, plus LAN UDP 8189 for WebRTC ICE. Browser/device certificate trust, offline operation, remote-device ICE reachability, and non-Chromium/mobile behavior remain pending; see the implementation plan's acceptance evidence and the [browser scenario](tests/browser/media-integration.md).
