# Big Brother CLI Client

`bb-client` is a headless macOS/Linux CLI client that connects to the existing BIG.BROTHER server application. This is a requirements discussion, not an implementation checklist.

## Usage scenarios

- A Raspberry Pi or other SBC (single-board computer) running as a bedside intercom or surveillance camera facing out of a window.
- First hardware target: Raspberry Pi 3 running Raspberry Pi OS Lite 64-bit, a directly attached USB keyboard, a Razer or Logitech USB webcam, and PC speakers connected to the Pi's analog audio output jack. Exact webcam model, microphone availability, and OS release must be checked during the hardware spike.
- Initially launched via SSH; later optionally started automatically at boot. Keyboard controls come from the keyboard attached to the Pi, not the SSH terminal.

## Vocabulary

HTT or `htt`: "hold to talk" — transmit microphone audio only while the configured key is held. Release stops microphone transmission. Video is independent.

## BB-Client Usage

`bb-client <hostname[:port] or HTTPS URL>`

The implementation must define default-port behavior and a way to trust the server's existing local CA. Do not assume the server always uses port 443.

Args/switches

- `--friendly-name` (optional) a friendly display name for this client. default: client's hostname
- `--audio-transmit` (optional) values allowed:
  - `none` this client does not transmit audio
  - `htt` this client only transmits audio if the HTT key is down (default)
  - `always` this client always transmits audio
- `--htt-key` (optional) key to hold for microphone transmission (default: spacebar)
- `--audio-transmit-toggle-key` (optional) cycle audio transmit mode between none/htt/always (default: a)
- `--output-vol-up-key` (optional) key to use for output device volume up (default: up arrow)
- `--output-vol-down-key` (optional) key to use for output device volume down (default: down arrow)
- `--transmit-video-toggle-key` (optional) key to use for video transmit on/off (default: v)
- `--video-transmit` (optional) on/off; default on. Transmit continuously whenever a camera is available and this setting is on, independently of HTT.
- `--audio-receive` (optional) on/off; default on. Allows camera-only deployments without playback.
- `--audio-input-device` (optional) the name of the audio input device to use. default: system default device
- `--audio-output-device` (optional) the name of the audio output device to use. default: system default device
- `--video-input-device` (optional) the name of the video input device to use. default: system default device
- `--status-announce-key` (optional) key to announce system status info. (default: s)
- `--list-devices`: list input/output devices to make selection practical.

For `--audio-input-device`, `--audio-output-device`, and `--video-input-device` use case-insensitive substring matching. For example, `--audio-input-device=logitech` should match any audio device with "Logitech" in its name. Prefer a case-insensitive exact match, otherwise pick the first match. Log ambiguous candidates and the selected device. An explicit selection remains preferred across hotplug events; see below.

## Receiving audio and volume

- Play audio from all other participants, mixing simultaneous speakers and excluding this client's own stream. Do not subscribe to remote video.
- In HTT mode, suppress incoming speech locally while the HTT key is held. Resume live playback on release; do not replay buffered speech.
- User volume controls change the selected output device's system volume. Application playback gain remains at 100%; there is no separate application volume setting.
- HTT suppression is a temporary application playback gate, not a system-volume change. Do not alter or restore the system mixer on every key press.
- `always` mode remains available, but simultaneous speaker/microphone use needs physical echo testing. Initial HTT behavior does not establish echo cancellation for `always` mode.
- Proposed volume details: 0–100% in 5-percentage-point steps; volume-up unmutes the selected output. At 0%, announcements are also silent; volume-up restores audible feedback. Coalesce repeated announcements. Preserve the existing system volume on startup rather than resetting it.

## Keyboard Controls

`bb-client` should recognize keyboard hotkey inputs. All are remappable via command line args as described above.

Recommended Linux implementation: read the selected physical keyboard through evdev, independently of stdin, terminal focus, SSH, or a desktop session. Detect press/release events; ignore repeat events for HTT and mode toggles. Volume controls may repeat. Clear held-key state when the keyboard or connection is lost; require a fresh press before resuming HTT after recovery.

For the dedicated Pi, recommend exclusive access to the selected keyboard so control presses do not also reach a local login console. Discover devices rather than hardcoding `/dev/input/eventN`; arrange device permissions during installation. Keep the keyboard-specific implementation separate from shared client behavior. macOS requires its own input implementation and validation.

The primary intended use case is for headless clients with no monitor attached, so we must give audio feedback

- audio-transmit-toggle: cycles audio transmit mode between none/htt/always (default `a`)
  - audio feedback: announce disabled, hold-to-talk, or always-on mode using bundled recordings
- output-vol-up: increase output device volume (default: up arrow)
  - audio feedback: say "volume X%" so the user can hear the new output volume
- output-vol-down: decrease output device volume  (default: down arrow)
  - audio feedback: say "volume X%" so the user can hear the new output volume
- transmit-video-toggle: toggle video transmission on/off
  - audio feedback: say "Video transmission <on/off>"
- status-announce
  - Announce actual connection, microphone, camera, and transmission state, distinguishing configured mode from unavailable hardware or failed media. For example: "Connected. Microphone unavailable. Video transmitting."

Use bundled WAV recordings initially. The detailed implementation plan must include a complete recording script with exact filenames, including phrases/numbers for volume. Arbitrary device names can remain in logs; runtime TTS is not required. Feedback must stay local and avoid being retransmitted acoustically through the microphone. Recommended policy: defer routine prompts during HTT speech and gate microphone transmission during spoken feedback, including in `always` mode.

## Notes

- Must function with no video input devices connected
- Must function with no audio input devices connected
- Speaker-only, microphone-only, and camera-only operation are valid. If all devices are absent at startup or disappear later, remain running and wait for hardware. Invalid arguments should exit with an informative error.
- IMPORTANT: If the client is disconnected from the server, it must automatically retry connections
  - Retry every 10 seconds
  - Because the total number of possible clients is small (remember, bigbrother is a small household application) we do not need retry backoff behavior. total clients is not expected to exceed 10.
  - This should cover client network disconnects as well as server crashes/reboots
  - Attempt immediately at startup; retry after failures with bounded connection attempts and no overlapping retry loops.
  - Recover room membership, publication, and subscriptions. A connected room WebSocket does not prove that media is working.
  - Preserve configured modes and device preferences, but clear held HTT state.

## Existing Web UI

People using the existing Big Brother web UI will see these headless clients connected as regular "tiles", just like the other existing connected users. From a web UI user's perspective, they are equivalent to other web UI users.

Use intentional labels rather than perpetual "Waiting for this participant's media" for clients that do not publish video or media. Suggested labels: "Audio only" for an audio publisher, "Listening only" for a receive-only client, and "Waiting for devices" when hardware is absent. Preserve distinct reporting for actual media failures. Do not infer intentional receive-only operation solely from `streamReady=false`.

## Future Planned Features

We are not implementing these features in the current iteration. However, we should make sure that our current architecture choices are compatible with and support these future planned features.

Keep local capture separate from transmission so future detection can inspect input while publication is disabled. Do not build a recognition/plugin framework now.

- Motion activation mode
  - `bb-client` will only send audio/video when motion has been detected
- Facial/body recognition
  - `bb-client` will recognize faces and bodies
- Gesture recognition (not sure if feasible)
  - `bb-client` will recognize body gestures
- License plate reading
- Voice activation
- Automatic volume normalization for transmitted audio

## Device Connects/Disconnects

- In automatic selection mode, prefer newly connected devices independently for microphone, speaker, and camera.
- An explicit device selection takes precedence over unrelated arrivals. If it disappears, temporarily fall back to an available device; restore the preferred device when it returns.
- When no replacement is available, keep running and wait for hardware. Preserve configured transmission modes while reporting actual availability.
- Log switches and use generic recorded announcements when an output is available; arbitrary device-name speech is not required.
- Keyboard removal must release HTT immediately. Keyboard rediscovery/reconnection is part of appliance behavior.

## STDOUT output

Log all operations to std out

- Keyboard control presses
  - Except hold to talk
- Device switches
- Verbose startup information for diagnostics
- Server connects and reconnects
  - Except for automatic reconnection retries. Echoing every single one to stdout would be far to verbose. Just write "will attempt reconnect to <hostname/address> every <X> seconds"
  - Log eventual recovery and materially changed failure reasons.

## Architecture direction and phases

- Provisional stack: Python controlling GStreamer, subject to hardware and installation validation. No requirement to match the server's Go implementation.
- Reuse existing room membership and WHIP/WHEP media signaling, with the current H.264/Opus media contract. Follow the [centralized-media ADR](../adrs/20261021-go-first-backends-and-centralized-media.md).
- Raspberry Pi OS Lite 64-bit is the selected starting OS. Confirm its exact release, audio mixer, webcam capture formats, encoder availability, and Pi 3 CPU/latency during the spike. Begin with modest video settings; hardware acceleration and acceptable performance are not established.
- Recommend one foreground program that also runs under a systemd service. Start the service manually over SSH initially; enable boot startup later. A plain foreground SSH launch remains useful for diagnostics but is not the appliance lifecycle. No desktop login or terminal input should be required.
- Bring basic service packaging into the first usable phase so closing SSH need not stop the appliance. Automatic startup can remain disabled until wanted.

1. Hardware feasibility: real keyboard press/release, webcam/microphone capture, browser-compatible publication, speaker playback, system volume, CPU and latency on the Pi 3; validate the candidate stack on macOS too.
2. Usable client: room/media integration, HTT and transmit modes, explicit device selection, initial recorded feedback, status, network/server recovery, and manual service startup.
3. Appliance completion: hardware arrival/removal/fallback, keyboard recovery, optional boot startup, configurable controls, announcement refinements, and broader physical-device checks.

Synthetic checks do not establish audible behavior, physical device recovery, or Pi performance. The implementation plan belongs in `docs/features/` and must include an exact WAV recording manifest. No product implementation has been started by this discussion.

## Implementation references

- [Linux input event semantics](https://docs.kernel.org/input/event-codes.html)
- [python-evdev input handling and exclusive access](https://python-evdev.readthedocs.io/en/latest/tutorial.html)
- [Raspberry Pi audio overview](https://pip-assets.raspberrypi.com/categories/1259-audio-camera-and-display/documents/RP-008124-WP/Choosing-an-Audio-option)
- [systemd service start versus enable](https://www.freedesktop.org/software/systemd/man/latest/systemctl.html)
