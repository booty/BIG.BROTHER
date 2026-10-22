# Media integration browser scenario

`scripts/test-media-browser` exercises the packaged Go application in headless Chromium with synthetic camera and microphone input. Run it with an empty room and no other browser clients connected. It checks that both clients decode live audio/video, self-preview remains muted, MediaMTX sees H.264 plus Opus, each publisher has one reader, and closing a participant removes its publication and the other participant's reader.

Build and start the packaged application with a disposable TLS configuration before running it:

```sh
scripts/build --target darwin/arm64
scripts/setup-tls --config-dir tmp/intercom-run/config --host localhost --ip 127.0.0.1
tmp/build/darwin-arm64/bigbrother --config-dir tmp/intercom-run/config --port 18445
BASE_URL=https://localhost:18445 scripts/test-media-browser
```

The scenario uses Chrome's fake media flags and ignores the disposable certificate warning. It verifies media exchange and teardown; it does not verify trusted certificates, physical camera/microphone behavior, another browser engine, a second LAN device, or internet-disconnected operation. The 2026-10-21 run passed in Chrome for Testing 156.0.8078.47 via agent-browser 0.39.1. The wider desktop/mobile browser matrix remains pending.
