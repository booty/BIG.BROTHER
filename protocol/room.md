# Room WebSocket contract

The application serves the room WebSocket at `GET /ws` on the same HTTPS origin as the frontend. Clients send JSON text messages shaped as `{ "type": "...", "payload": { ... } }`. Server messages use the same envelope. Friendly names are labels and are not unique identities.

## Client messages

- `join`: `{ "name": string }`. A connection joins once. The server trims surrounding whitespace and uses `Guest ` plus the first four characters of the generated session ID when the name is blank.
- `rename`: `{ "name": string }`. The same trimming and blank-name fallback apply.
- `mediaState`: `{ "audioEnabled": boolean, "videoEnabled": boolean }`. These flags announce the sender's outgoing state; they do not say whether a stream is ready.
- `streamState`: `{ "ready": boolean }`. The publisher sends `ready: true` only after WHIP negotiation succeeds, and sends `false` when publication fails or is released. This is independent of the sender's audio/video enabled flags.

## Server messages

- `welcome`: `{ "selfId": string, "participants": Participant[] }`, sent once after a successful join and before that connection receives any later snapshot.
- `participants`: `{ "participants": Participant[] }`, a full room snapshot after each join, rename, media-state change, stream-state change, and departure.
- `error`: `{ "message": string }` for malformed JSON, missing or invalid fields, unsupported types, or actions before joining. A malformed message does not disconnect the client or disturb other participants.

`Participant` has the shape `{ "id": string, "name": string, "audioEnabled": boolean, "videoEnabled": boolean, "streamId": string, "publishUrl": string, "readUrl": string, "streamReady": boolean }`. The backend assigns a random 24-character lowercase hex `streamId` per WebSocket connection. `publishUrl` and `readUrl` are absolute same-origin URLs ending in `/media/{streamId}/whip` and `/media/{streamId}/whep`; the server derives them from each recipient's HTTPS host so clients can use the same certificate-valid origin. `streamReady` begins false and becomes true only after the publisher has a successful WHIP session. IDs are generated per WebSocket connection, so multiple connections from one device or address remain separate. Participant snapshots are sorted by ID.

## Disconnects and ordering

The server sends WebSocket protocol pings every 10 seconds and drops a connection if its pong does not arrive within 30 seconds. The reader stays active to process pongs. A closed or expired connection is removed once; reconnecting creates a new connection ID. Room changes are serialized, and the welcome snapshot is written before subsequent updates on the joining connection. Tests can configure shorter ping and pong intervals.

The application does not send JSON heartbeat messages. WebSocket upgrade is the only operation that creates a connection; `join` adds it to the participant snapshot. `/healthz` remains application liveness. `GET /media/status` separately reports whether the owned MediaMTX process is available; an unexpected media exit leaves room state available and makes new `/media/` requests return 503 with an actionable status.

## Browser media signaling

The browser sends WHIP/WHEP SDP directly to the returned URLs. The Go HTTPS origin proxies only `OPTIONS`, `POST`, `PATCH`, and `DELETE` to MediaMTX on `127.0.0.1:8889`, removing the `/media/` prefix. It preserves request bodies, content types, status codes, and response/session headers, and rewrites MediaMTX `Location` values back under `/media/`. `X-Bigbrother-Session` identifies the participant that owns each WHIP/WHEP resource; departure deletes those resources through MediaMTX so stale readers and publications do not remain attached. Media payloads use the direct browser-to-MediaMTX ICE/UDP path and never pass through `/ws`.

## Delivery under slow connections

Room updates are queued in mutation order for each connection; one writer per
connection performs network writes outside the room mutex. Welcome precedes that
connection's snapshots. Each client can queue up to 32 messages, and a network
write has a five-second deadline. A stalled client whose queue fills is closed
rather than blocking other clients' joins, departures, or updates.
