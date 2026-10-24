# Convenient setup and launch

`bin/setup` prepares a native build using the existing `scripts/build` pipeline.
Go and Node/npm remain explicit machine prerequisites; setup installs application
dependencies and downloads the pinned MediaMTX release. `bin/launch` runs setup,
then replaces itself with the native executable and forwards server arguments.
Cross-compilation remains available through `scripts/build --target OS/ARCH`.

On first setup, create localhost certificates (localhost, 127.0.0.1, and ::1)
under `tmp/bigbrother-local/tls` using `scripts/setup-tls`. Reuse existing server
certificate/key pairs; incomplete pairs fail rather than silently replacing a
key. If both server files are missing, issue them using the existing CA when
available. Launch uses this repo-local configuration directory by default;
explicit server arguments override it. LAN certificates use an explicit
configuration directory and hostname/IP through `scripts/setup-tls`.

Make dependency installation, frontend compilation, backend compilation, and
media packaging independently incremental. Store content fingerprints alongside
their outputs, publish fingerprints only after success, and require expected
outputs before skipping a stage. Include tool versions in the relevant inputs.
Keep the existing checksum verification for newly packaged MediaMTX releases.

Follow the [existing TLS and media decisions](../adrs/20261021-go-first-backends-and-centralized-media.md).
Certificate trust remains a manual device operation.
