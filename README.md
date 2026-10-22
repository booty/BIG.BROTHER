# BIG.BROTHER

BIG.BROTHER is a small home intercom. The Go backend serves room state over HTTPS, launches the bundled MediaMTX v1.21.2 process, and proxies browser WHIP/WHEP signaling on the same HTTPS origin. Browser media travels directly over LAN ICE/UDP; video does not pass through the room WebSocket. The current page joins with a friendly name, shows participant video tiles, and uses a muted local preview.

## Environment Setup

Python is used by some development/agent utilities, not by the intercom runtime. For a Python script that requires PyYAML, use `uv` to provide the dependency in an isolated environment instead of modifying Homebrew's Python installation:

```sh
uv run --no-project --with pyyaml python /path/to/script.py
```

For example, validate the Claude review skill with:

```sh
uv run --no-project --with pyyaml python \
  ~/.codex/skills/.system/skill-creator/scripts/quick_validate.py \
  ~/code/jbootz-llm-skills/skills/jbootz-claude-from-codex-review
```

`uv` installs/caches PyYAML for that invocation's environment; plain `python3` does not gain the package. If a persistent environment is preferred:

```sh
python3 -m venv ~/.venvs/skill-tools
~/.venvs/skill-tools/bin/python -m pip install PyYAML
```

Run scripts with `~/.venvs/skill-tools/bin/python` afterward. See the [environment memory](docs/memories/20261021-isolated-pyyaml-for-python-agent-utility-validation.md) for the verified local setup and troubleshooting context.

## Development

Requirements for the current shell are Go 1.22 or newer, Node.js with npm, and OpenSSL. Build the browser assets with `cd web && npm ci && npm run build && npm run typecheck`. Run backend unit tests with `cd backends/go && go test ./...`. The backend-independent room contract suite is in `tests/contract`; its test command expects a running backend.

## TLS setup

Choose the LAN names and addresses clients will use, then run:

```sh
scripts/setup-tls --host intercom.local --ip 192.168.1.20
```

The script uses OpenSSL to create a local CA and a server certificate in the platform's `bigbrother/tls` configuration directory. Install `ca.crt` on each client device separately. If the server's LAN name or address changes, reissue only the server certificate with `scripts/setup-tls --host intercom.local --ip 192.168.1.21 --replace-server-cert`; the existing CA is preserved. The application never creates certificates or installs trust at startup.

After trust is configured, start a built package with `tmp/build/OS-ARCH/bigbrother`. It tries port 443, then 8443 through 8453, and then an OS-assigned port. An explicit port such as `bigbrother --port 1234` fails if it cannot bind. Use `--config-dir PATH` to select another configuration directory; `--tls-cert PATH` and `--tls-key PATH` override the default certificate pair and must be supplied together. Startup waits for the bundled media service's loopback Control API before serving the app. A media startup failure stops application startup with an error.

`scripts/build --target OS/ARCH` packages the Go executable, browser assets, MediaMTX v1.21.2, and its config under `tmp/build/OS-ARCH/`. Supported targets are `linux/arm64`, `linux/amd64`, and `darwin/arm64`. The build downloads the matching MediaMTX release archive and verifies its SHA-256; the packaged runtime does not download anything. See [media setup and ports](media/README.md).

Run a prebuilt package on its matching OS/architecture; Go, Node.js, npm, and OpenSSL are not runtime dependencies. OpenSSL is needed only when provisioning or reissuing TLS certificates. Keep TCP 443 or a selected fallback port available for HTTPS and allow LAN UDP 8189 for WebRTC media. Linux may restrict binding port 443 for an unprivileged user; the app tries 8443 through 8453 and then an OS-assigned port when the default bind fails. An explicitly requested port never falls back.

Stop the application with Ctrl-C; it asks MediaMTX to close outstanding sessions, sends a termination signal, and reaps the child. `/healthz` reports application liveness, while `GET /media/status` reports media availability. If MediaMTX exits unexpectedly, room state remains available and new media signaling requests return 503; restart the app to recover. Media listeners use loopback TCP 8889 for signaling and API 9997, plus LAN UDP 8189 for WebRTC ICE. Browser/device certificate trust, offline operation, remote-device ICE reachability, and non-Chromium/mobile behavior remain pending; see the implementation plan's acceptance evidence and the [browser scenario](tests/browser/media-integration.md).
