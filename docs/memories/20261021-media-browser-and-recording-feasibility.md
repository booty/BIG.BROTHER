# Media browser and recording feasibility

Last verified: 2026-10-21

## Verified local result

MediaMTX v1.21.2 passes synthetic browser publishing, reading, recording, and replay on this host: macOS 15.7.9, Darwin ARM64. The probe used Go 1.27.1, Node 26.12.0, npm 11.20.0, OpenSSL 4.0.3, FFmpeg 8.1.2, agent-browser 0.39.1, and its Chrome for Testing 156.0.8078.47. The installed agent-browser engine exercised here is Chromium only.

The disposable recording probe used these listeners: WebRTC HTTP `127.0.0.1:8889`, ICE UDP `:8189`, Control API `127.0.0.1:9997`, playback `127.0.0.1:9996`, and RTSP `127.0.0.1:8554`. The browser clients published through WHIP and read through WHEP. A separate Go HTTPS reverse proxy at `https://localhost:18443` stripped `/media/` upstream and rewrote session `Location` values back below `/media/`.

For both WHIP and WHEP through that proxy, OPTIONS returned 204, POST returned 201 with `Content-Type: application/sdp`, PATCH returned 204 with `Content-Type: application/trickle-ice-sdpfrag`, and DELETE returned 200 when the source/reader teardown was ordered independently. The rewritten session URLs remained under `/media/`. The WHEP browser reached `readyState: 4`, decoded 640×360 video, and received live audio and video tracks. Publisher `getStats()` contained non-zero `video/H264` and `audio/opus` counters. MediaMTX `/v3/paths/list` reported H.264 Baseline (640×360, no B-frames) and Opus with non-zero byte counters.

The MediaMTX test config enabled fMP4 recording with five-second segments. A five-second recording from the browser publisher and one from the synthetic RTSP publisher both downloaded as `video/mp4`; `ffprobe` found H.264 video plus Opus audio in each. The recorded RTSP clip also played to completion in headless Chrome at 640×360. RTSP ingest used an explicitly synthetic FFmpeg source (`testsrc` plus `sine`), encoded with libx264 Baseline (`-bf 0`) and libopus. MediaMTX did not need to transcode this codec-compatible source, so the tested path had no conversion cost. This is not a physical camera or target-hardware CPU measurement.

With the checked-in [packaged config](../../media/mediamtx.yml), MediaMTX logged only these listeners: WebRTC HTTP `127.0.0.1:8889`, ICE UDP `:8189`, and Control API `127.0.0.1:9997`. The API request `GET http://127.0.0.1:9997/v3/paths/list` returned HTTP 200 with an empty path list when ready. The server's WebRTC answer included both loopback and a private LAN host candidate on UDP 8189, but no second physical device tested LAN reachability.

The packaged BIG.BROTHER UI also passed `scripts/test-browser` on this host with synthetic Chrome media: name persistence/editing, autoplay retry, sender and receiver controls, audio-only use without a camera, and denied-permission recovery. A successful microphone/camera selection matched `MediaTrackSettings.deviceId` to the selected ID and ended the old tracks; an unavailable camera retained the room and live media. The repeatable details are in [`tests/browser/intercom.md`](../../tests/browser/intercom.md). This verifies the browser replacement/error paths, not physical device removal.

## Pinned release assets

The official v1.21.2 release archives and checksums used by `scripts/build` are:

| Target | Release asset | SHA-256 |
| --- | --- | --- |
| `darwin/arm64` | `mediamtx_v1.21.2_darwin_arm64.tar.gz` | `7d0d0c5c11527cbd10bde1c3d728864eb0334a9b653566cd73c0259750cbf093` |
| `linux/arm64` | `mediamtx_v1.21.2_linux_arm64.tar.gz` | `180a235374a548d9133cdd5e20f5dbd8b01317be1c86e0d7e66df585dc42ed6f` |
| `linux/amd64` | `mediamtx_v1.21.2_linux_amd64.tar.gz` | `121be6e00397e473a95d83c75f4fcd3819a6af3b0e1a50889cba43ab4fab6a27` |

All three archives matched the official release checksum list. `file` reported Mach-O ARM64, ELF AArch64, and ELF x86-64 respectively. The Darwin binary ran and validated the packaged config; Linux execution was not available on this Mac.

## Reproduction notes

Run `scripts/build --target darwin/arm64`, copy its `media/mediamtx` into `tmp/intercom-probe/mediamtx/`, then use `tmp/intercom-probe/mediamtx.yml` and the disposable publisher/reader pages. Detailed browser steps and assertions are in [`tmp/intercom-probe/README.md`](../../tmp/intercom-probe/README.md) while that ignored directory exists. The direct MediaMTX pages are `http://localhost:8889/probe/publish` and `http://localhost:8889/probe`; use isolated named agent-browser sessions and `--use-fake-device-for-media-stream,--use-fake-ui-for-media-stream` for synthetic input.

For the camera-shaped representative, the command was:

```sh
ffmpeg -hide_banner -loglevel info -re \
  -f lavfi -i 'testsrc=size=640x360:rate=20' \
  -f lavfi -i 'sine=frequency=440:sample_rate=48000' -t 30 \
  -c:v libx264 -preset ultrafast -tune zerolatency -profile:v baseline -bf 0 \
  -pix_fmt yuv420p -b:v 700k -g 40 -c:a libopus -ar 48000 -ac 1 -b:a 32k \
  -f rtsp -rtsp_transport tcp rtsp://127.0.0.1:8554/rtsp-representative
```

Use `curl 'http://127.0.0.1:9996/list?path=rtsp-representative'` to find a recorded interval, then request `/get?path=rtsp-representative&start=<start>&duration=5` and inspect with `ffprobe`. The disposable TLS origin was generated with `scripts/setup-tls --config-dir tmp/intercom-probe/config --host localhost --ip 127.0.0.1`; server-certificate reissue with `--replace-server-cert` preserved the CA in a separate disposable test.

MediaMTX's browser helper compares the codec string case-sensitively after lowercasing SDP. Configure it as `h264` (lowercase), even though the built-in publisher UI displays `H264`; uppercase resulted in an audio-only WHIP session. With lowercase `h264`, browser stats showed both video and audio.

## Pending checks and limits

- No physical camera, Android device, or iOS device was available. Firefox and desktop Safari were not driven because the installed headless agent-browser exposed only Chromium for this workflow.
- The local CA was not installed into any OS/device trust store. Browser checks used `--ignore-https-errors`; that is not certificate trust validation. The macOS agent-browser CLI reports that its `--ca-cert` option is unsupported for local Chrome.
- Internet was not disabled while preserving LAN connectivity. No public STUN/TURN was configured, but LAN-only operation was not tested on a separate device.
- Fresh offline Firefox with and without provisioned OpenH264 remains untested. Physical browser permissions, audible playback, camera switching/removal, mobile resume behavior, and the 1/3/6 participant hardware measurements remain pending.
- The tested synthetic RTSP source already provided H.264 Baseline plus Opus. Conversion requirements and CPU cost for a physical camera with different codecs are unknown.

See [media setup and packaged ports](../../media/README.md), the [implementation plan's acceptance table](../features/20261021-initial-intercom-implementation.md), and the [design's current media contract](../features/20261021-initial-intercom-design.md).
