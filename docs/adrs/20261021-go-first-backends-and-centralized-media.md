# Go-first application backends with centralized media

- Status: accepted
- Date: 2026-10-21

## Context

BIG.BROTHER is a home video intercom and learning project. Typical use is 1–3 browser clients, occasionally around 6, on desktop and mobile Chrome, Firefox, and Safari. It must work without internet access. The host should run on modest Linux hardware and on macOS for development.

The owner knows Python, Ruby, C, and C# and wants to learn Go, potentially followed by Elixir. IP cameras and motion-triggered recording are near-term follow-ups, not distant possibilities. Simple operation matters more than production-scale infrastructure. The home network is the security boundary; application authentication is out of scope.

## Decision

1. Implement the first application backend in Go, without a full-stack framework. Use focused HTTP/WebSocket libraries where useful; do not implement network protocols ourselves.
2. Use one shared browser frontend, written in TypeScript, and a documented application protocol. Plan for alternative backend implementations, with Elixir a likely second implementation. Only one application backend runs at a time.
3. Use backend-independent contract tests through the public interface. Keep language-specific internals idiomatic. Do not introduce a plugin framework or require simultaneous feature parity from day one.
4. Route media through an existing media server from version 1. Browsers publish their camera/microphone stream and subscribe to other streams. The frontend composes the tiles; the server does not mix a video mosaic.
5. Share media infrastructure across application backends. Application code owns participants, names, settings, and later recording policy; media infrastructure owns transport and recording mechanics.
6. Evaluate MediaMTX first. Its adoption is provisional until browser interoperability, camera ingestion, and recording have been demonstrated. The centralized-media architecture is accepted independently of that product choice.
7. Serve all runtime frontend assets locally. Do not depend on public signaling, STUN, TURN, CDNs, or other internet services for operation.
8. Use HTTPS with a certificate trusted by client devices because browsers require a secure context for camera/microphone capture. This adds no application login or authorization system. Provision certificates once and load them from a default configuration directory on normal launches; optional flags override those defaults. Local-CA trust instructions must be validated on target devices.
9. Preserve a simple launch command. The application backend owns the media child process and proxies browser media negotiation through one HTTPS origin. A single-file executable and a single network port are not requirements.

## Alternatives and tradeoffs

- **Browser mesh:** A small signaling server is attractive for 1–3 clients. Mesh does not preclude camera recording, but it would introduce a second media arrangement or require changing the browser media path soon. Centralized media better matches the near-term roadmap.
- **Phoenix:** Useful realtime abstractions, but obscures some of the learning the owner wants. Elixir can be explored later using its process model and focused networking libraries.
- **Only one backend forever:** Less maintenance. Multiple implementations are justified by learning, not performance or product necessity; build a working Go slice before duplicating it.
- **Custom WebRTC/media server:** Offers more protocol-level learning, but substantially expands scope. Reuse media software instead.
- **Rust first:** Viable, but adds ownership and async complexity without a demonstrated application need. Go is a smaller transition from the owner's existing experience.

Centralizing media adds a process and server bandwidth requirements. Camera codecs may need conversion for browser playback; forwarding and recording compressed streams are cheaper than transcoding. Motion analysis is separate work and may require decoded frames. Hardware capacity must be measured, not assumed.

## Revisit when

- MediaMTX cannot satisfy the actual browser/camera/recording tests: evaluate another media component while preserving the application boundary.
- Target hardware cannot support measured forwarding, conversion, or motion-analysis costs: adjust camera formats, quality, or hardware before rewriting application backends.
- Maintaining alternative backends stops providing learning value: retain one supported backend and label others experimental.
- Requirements expand beyond a trusted home network: reconsider security and deployment assumptions explicitly.

## References

- [Initial intercom design](../features/20261021-initial-intercom-design.md)
- [Browser camera access requirements](https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getUserMedia)
- [MediaMTX browser publishing](https://mediamtx.org/docs/publish/web-browsers)
- [MediaMTX codec compatibility](https://mediamtx.org/docs/features/webrtc-specific-features)
- [MediaMTX recording](https://mediamtx.org/docs/features/record)
