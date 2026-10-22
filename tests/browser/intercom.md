# Intercom UI browser scenarios

Run `scripts/test-browser` against a packaged app with an empty room. It uses headless Chromium synthetic camera/microphone input and saves JSON evidence under `tmp/test-results/`. The automated checks cover a two-participant room, localStorage name persistence and editing, muted self-preview, blocked-autoplay retry, per-receiver mute/hide without changing the sender, outgoing microphone/camera state, successful device selection, failed camera-switch retention, denied permission retry, and an audio-only client with no camera.

```sh
scripts/build --target darwin/arm64
tmp/build/darwin-arm64/bigbrother --config-dir tmp/intercom-run/config --port 18445
BASE_URL=https://localhost:18445 scripts/test-browser
```

The 2026-10-21 run passed with agent-browser 0.39.1 and Chrome for Testing 156.0.8078.47. It checks that successful device selection uses the requested device ID and ends the replaced track, and that an unavailable camera leaves both participant tiles and their live tracks intact. The test uses fake media and `--ignore-https-errors`; it does not verify physical device removal, trusted CA installation, Firefox/Safari, Android/iOS behavior, mobile backgrounding, or a six-tile layout. Those checks remain labeled pending in the implementation plan.
