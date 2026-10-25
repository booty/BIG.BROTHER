# Headless client feasibility spike plan

Status: planned; no spike scripts or hardware checks have been completed.

**Goal:** Determine whether Python/GStreamer can provide a responsive HTT intercom on the owner's Raspberry Pi 3, using the existing BIG.BROTHER server and browser UI.

**Spec:** [Headless client requirements](../spitballing/hold-to-talk-headless-client.md).
**Architecture:** Keep the existing room protocol and centralized media path. Use disposable client probes, with native keyboard input on Linux. See the [media architecture ADR](../adrs/20261021-go-first-backends-and-centralized-media.md).
**Candidate stack:** Python, GStreamer, Linux evdev, and the installed system audio mixer. These are candidates to validate, not a settled implementation.

Work through the checklist in order, checking steps only after their stated evidence exists. A future agent should read the spec and this plan, inspect completed results, and resume at the first unchecked step. No harness-specific workflow is required.

## What is blocked on hardware?

| Work | Can happen before the Pi is ready? |
| --- | --- |
| Plan, probe scripts, browser scenarios, and results template | Yes |
| Candidate stack installation and synthetic media exchange on the Mac | Yes, if required tools are available |
| Physical Pi keyboard, microphone, speaker, camera, system mixer | No |
| Pi CPU, memory, latency, USB stability, and SSH-independent operation | No |
| Final client stack decision and detailed product implementation plan | Wait for the Pi evidence |

Preparing scripts does not establish that they work on the Pi. Mark unexecuted or unsupported checks pending, not passed.

## Scope and evidence

- First target: Raspberry Pi 3, Raspberry Pi OS Lite 64-bit, USB keyboard, Razer or Logitech USB webcam, and powered PC speakers on the analog output jack. Keep the existing server on another machine; this does not test hosting server and client together on the Pi.
- Probe HTT audio, continuous video, audio-only receive/mixing, system volume, loss of keyboard/device/network, and manual service operation. Start with 640×360 video at 15 frames/s; use a supported capture mode and convert if necessary, recording the cost.
- Do not build the full CLI, automatic device-selection policy, permanent UI changes, recognition features, or a production packaging system. Limited recovery probes answer feasibility questions; complete behavior belongs in the product plan.
- Use ordinary scripts and a few focused assertions. No broad unit-test suite or abstractions are needed for throwaway probes.
- Keep reproducible scripts/scenarios in `tests/spikes/headless-client/`. Store raw logs, sample audio/video, and metrics under ignored `tmp/headless-client-spike/` on each machine. Preserve concise findings in `docs/features/20261024-headless-client-spike-results.md` when execution begins.
- Read current official documentation through Context7 before implementing library-specific calls. Record versions and exact successful installation commands. Do not assume particular GStreamer plugins, encoders, mixer controls, or device node numbers exist.
- Check required tools before beginning each experiment. For browser automation, check `agent-browser` availability and read its help; use it headlessly where supported. If a required tool is unavailable, report that limitation and keep the affected check pending rather than silently switching tools.

## 1. Prepare the physical target — owner, before Pi execution

- [ ] Find the Pi 3, suitable power supply, microSD card, USB keyboard, webcam, and powered speakers. Record the exact Pi and webcam model; do not assume every webcam includes a microphone.
- [ ] Install Raspberry Pi OS Lite 64-bit using Raspberry Pi Imager. Configure a hostname, login, SSH access, and network settings. Follow the [official headless setup instructions](https://www.raspberrypi.com/documentation/computers/getting-started.html). Record the selected OS image/release.
- [ ] Boot and confirm SSH access from the development machine. Connect the keyboard/webcam and speakers. Ethernet is a useful initial baseline if convenient; record the network actually used.
- [ ] Supply the hostname/IP and SSH username for the execution session; do not put passwords or private keys in project documents. No application dependency setup or voice recordings are required from the owner yet.

**Ready when:** An executor can log in and the owner can operate the attached keyboard, speak into the microphone, and listen to the speakers during physical tests.

## 2. Establish the reproducible probe environment — preparation can begin now

**Create during execution:** `tests/spikes/headless-client/README.md`, `requirements.txt`, `inventory.sh`, and the results document named above.

- [ ] Read `protocol/room.md`, `web/src/room.ts`, `web/src/media.ts`, and `tests/browser/media-integration.md`. The protocol document and current code define the integration contract.
- [ ] On the Mac, verify Python/GStreamer bindings and available H.264/Opus and WebRTC elements. Record exact versions and installation steps. On the Pi, repeat after Task 1; do not extrapolate Mac availability to Linux.
- [ ] Implement `inventory.sh` to collect OS/release, architecture, kernel, CPU cores, memory, Python/GStreamer versions, relevant plugin availability, USB device identities, capture formats, keyboard capabilities, audio capture/playback devices, and mixer controls. Use installed tools and report missing tools explicitly.
- [ ] Identify the Pi's analog output and webcam microphone separately. Document permissions required for keyboard, camera, audio, and later service access. Use discovered identities rather than assuming `/dev/input/event0`, `/dev/video0`, or audio card index 0.
- [ ] Run the existing server at a stable LAN hostname/IP and explicit port. Provision/trust its local CA using existing project instructions. Record server revision, URL, media version, and browser version. Verify both room and media availability; server liveness alone is insufficient.
- [ ] Save all install/run commands in the probe README, with shell-safe placeholders and instructions for reproducing the same environment. Keep secrets out of logs.

**Pass:** Candidate components import/load, usable device identities are known, and the test client can reach the server with certificate verification. Missing components produce a concrete blocker, not an assumption that another platform will work.

## 3. Prove physical keyboard input — Pi and owner required

**Create during execution:** `tests/spikes/headless-client/keyboard.py`.

- [ ] Read physical-keyboard events through evdev while starting the probe from SSH. Log monotonic timestamps and press/release/repeat values for this diagnostic probe only; the eventual client should not log routine HTT presses.
- [ ] Use space as HTT, `a` as a mode toggle, `v` as video toggle, and arrows for volume actions. Run 20 press/release cycles and one 10-second hold. Assert one start/stop per HTT cycle and no repeated mode/video toggles from holding a key.
- [ ] Acquire exclusive access to the selected keyboard. Confirm presses do not reach the local console, then confirm stopping the probe releases access. A second SSH connection remains available for control.
- [ ] Unplug the keyboard while HTT is held. Clear held state immediately when removal is detected; rediscover it after reconnecting and require a fresh press. If events are dropped, reset/resynchronize rather than leaving HTT active.

**Pass:** Correct events arrive independently of SSH stdin or a desktop session, all cycles balance, and removal cannot leave transmission requested.

## 4. Prove local audio, volume, and camera access — Pi and owner required

**Create during execution:** `tests/spikes/headless-client/local-media.py`.

- [ ] Record and replay a short spoken sample using the selected webcam microphone and analog speakers. Confirm the sound comes from the intended jack, not HDMI. Keep the sample under `tmp/`.
- [ ] Play two distinguishable local audio sources together through one output path. Confirm both remain intelligible and the mix does not produce obvious clipping; application playback gain remains at 100%.
- [ ] Read/change the selected output's system mixer at modest volume. Test increase, decrease, mute/zero, and volume-up recovery. Confirm audible results and mixer readback; record supported increments and any deviation from the proposed 5-percentage-point steps. Restore the initial level after testing.
- [ ] Exercise separate playback suppression and microphone gating while media pipelines remain active. Suppression must not change system volume; resuming must play live audio rather than buffered speech.
- [ ] Play a temporary feedback sample with microphone transmission gated. Do not request final voice assets for the spike.
- [ ] Capture actual webcam video and identify its native formats. Test a browser-compatible H.264 encoding path at the initial video settings. Record whether encoding is software, hardware, or already provided by the camera; prove availability rather than assuming acceleration.

**Pass:** Real local media and mixer control work without a desktop login. Record limitations before adding networking so device failures do not get misdiagnosed as signaling failures.

## 5. Prove end-to-end room/media integration — synthetic preparation, then physical Pi

**Create during execution:** `tests/spikes/headless-client/client.py` and `browser-scenario.md`. Reuse the earlier probe helpers where practical; do not introduce a generic client framework.

- [ ] Join `/ws` with a friendly name; process `welcome` and participant snapshots. Keep reading to handle protocol ping/pong. Use the assigned stream IDs and publish/read URLs; do not hardcode paths.
- [ ] Publish H.264 video and Opus audio through WHIP, preserving `X-Bigbrother-Session` ownership and returned session locations. Announce `mediaState` and `streamState` accurately. Clean up sessions on normal exit. Follow `web/src/media.ts` for the existing signaling contract.
- [ ] Subscribe through WHEP to other ready participants with audio-only negotiation. Exclude self. Verify no remote video track is negotiated/received, rather than merely discarding decoded video.
- [ ] Validate synthetic exchange on the Mac with repeatable browser assertions first if the Pi is unavailable. Save commands and outcomes; this does not satisfy physical Pi checks.
- [ ] Repeat using actual Pi camera/microphone/speakers. Confirm the browser shows its normal participant tile and changing video frames. In HTT mode, verify speech arrives only while held and incoming speech is suppressed locally during the hold. Video continues through HTT cycles.
- [ ] Toggle video off and on without losing room membership or incoming audio. Confirm an audio-only source and a receive-only probe can participate; record existing UI label limitations without fixing them during the spike.
- [ ] Add two remote audio publishers. Confirm the Pi plays both concurrently, does not subscribe to itself, and removes a departing publisher cleanly. Keep browser endpoints acoustically isolated to avoid confusing test-room feedback with client behavior.

**Pass:** The real Pi interoperates with the current server/browser in both directions with the required codecs and receive behavior. An HTTP success or connected WebSocket alone is not a pass.

## 6. Exercise appliance lifecycle and representative recovery — Pi and owner required

**Create during execution:** `tests/spikes/headless-client/bb-client-spike.service` and recovery steps in the README. Use a manually started service; leave boot startup disabled.

- [ ] Run the probe under a service account/context with working input/media permissions and explicit configuration paths. Disconnect SSH, operate HTT and volume physically, then reconnect to inspect logs. No local desktop/login session may be required.
- [ ] Restart the test server. Retry on the agreed 10-second schedule with bounded, non-overlapping attempts; rejoin, republish, and resubscribe using fresh session IDs. Preserve configured modes but clear held HTT state. Log actual recovery duration.
- [ ] Interrupt the Pi's network locally, then restore it; arrange physical recovery rather than relying on the SSH connection being tested. Verify media returns and no stale held key resumes microphone transmission.
- [ ] Remove/reconnect the webcam. Keep the room connection and incoming playback where possible; record how capture/encoding must be reconstructed. Full automatic device preference/fallback policy is deferred.
- [ ] Start with media devices absent. Verify the probe can remain alive and later acquire available hardware. If this needs further work, record the failure as a product implementation requirement; do not claim appliance completeness.

**Pass:** Launch does not depend on SSH lifetime, and representative recovery is demonstrated or documented with a concrete unresolved technical obstacle. A client cannot repair a server-side media process that remains down; distinguish that case from client recovery.

## 7. Measure the combined workload — physical Pi required

- [ ] Run for 15 minutes with continuous video, HTT cycles, and two incoming audio streams. Repeat on the intended network if the baseline used Ethernet but deployment will use Wi-Fi.
- [ ] Record CPU, resident memory, temperature/throttling/power warnings when available, negotiated codecs, resolution/frame rate, and media errors at regular intervals. State CPU normalization explicitly: percentage of total machine capacity, not ambiguous per-core percentages.
- [ ] Record local key-to-gate timing from monotonic timestamps. Measure end-to-end audio/video latency with a documented physical recording method if possible; otherwise report human observations as subjective, never as measured milliseconds. Do not subtract unsynchronized timestamps from different machines.
- [ ] Initial decision targets, proposed for this spike: audio one-way delay at most 500 ms, video delay at most 1 s, no recurring audible dropouts or stuck HTT, average CPU below 75% of total capacity, and no sustained growth in memory/backlog or new throttling during the run. Report actual values; these are feasibility criteria, not claimed performance guarantees.
- [ ] If video prevents acceptable audio, repeat audio-only and then with reduced frame rate/resolution. Record which variable changed and the outcome. Do not expand into open-ended encoder optimization.

**Pass:** Useful conversation and modest video coexist with measurable headroom. If latency cannot be measured, mark that metric pending and collect the owner's usability judgment separately.

## 8. Record the decision and hand off

- [ ] Complete the results table below in the results document. Link successful commands/scenarios and summarize all failures, including exact errors and unresolved physical checks.
- [ ] Choose: proceed with the candidate stack; proceed with explicitly reduced video settings; or pause that stack pending one focused alternative experiment. Obtain the owner's decision on any capability reduction or hardware replacement.
- [ ] Validate the candidate's installation and synthetic exchange on macOS before presenting it as a cross-platform choice. Pi evdev success does not validate macOS keyboard handling or volume control; list those separately if still pending.
- [ ] Preserve reusable findings in a short memory and update its index only if they would save future rediscovery. Do not mark existing pending platform/device claims verified without matching evidence.
- [ ] Write the final feature design and phased product implementation checklist after the findings are accepted. Include the full WAV recording script with exact filenames there. The spike requires no final recordings.

| Check | Status: pending/pass/fail | Machine/version | Evidence and limitations |
| --- | --- | --- | --- |
| Candidate installation/imports | pending | | |
| Physical keyboard and removal | pending | | |
| Local microphone, analog speakers, and mixer | pending | | |
| H.264/Opus publication to browser | pending | | |
| Audio-only receive and two-source mix | pending | | |
| HTT gates, independent video toggle | pending | | |
| SSH-independent service operation | pending | | |
| Server/network/device recovery | pending | | |
| Combined workload and latency | pending | | |
| macOS installation/synthetic exchange | pending | | |

**Stop condition:** A written stack recommendation supported by the above evidence and explicit remaining limits. Do not turn the spike into the full client while waiting for hardware or chasing a failed feasibility criterion.
