export interface MediaSession {
  stream: MediaStream;
  replaceTrack(kind: "audio" | "video", track: MediaStreamTrack): Promise<void>;
  close(): Promise<void>;
}

const ICE_GATHERING_TIMEOUT_MS = 10_000;

export async function publishMedia(
  publishUrl: string,
  ownerStreamId: string,
  localStream: MediaStream,
  onConnectionState?: (state: RTCPeerConnectionState) => void,
): Promise<MediaSession> {
  const peer = new RTCPeerConnection({ iceServers: [] });
  if (onConnectionState) peer.addEventListener("connectionstatechange", () => onConnectionState(peer.connectionState));
  for (const track of localStream.getTracks()) peer.addTrack(track, localStream);
  preferValidatedCodecs(peer);
  const remote = new MediaStream();
  peer.addEventListener("track", (event) => {
    for (const track of event.streams[0]?.getTracks() ?? [event.track]) {
      if (!remote.getTracks().some((existing) => existing.id === track.id)) remote.addTrack(track);
    }
  });

  try {
    const locationUrl = await negotiate(peer, publishUrl, ownerStreamId);
    return createSession(peer, remote, locationUrl, ownerStreamId);
  } catch (error) {
    peer.close();
    throw error;
  }
}

export async function subscribeMedia(
  readUrl: string,
  ownerStreamId: string,
  onTrack: (stream: MediaStream) => void,
  onConnectionState?: (state: RTCPeerConnectionState) => void,
): Promise<MediaSession> {
  const peer = new RTCPeerConnection({ iceServers: [] });
  if (onConnectionState) peer.addEventListener("connectionstatechange", () => onConnectionState(peer.connectionState));
  const remote = new MediaStream();
  peer.addTransceiver("audio", { direction: "recvonly" });
  peer.addTransceiver("video", { direction: "recvonly" });
  peer.addEventListener("track", (event) => {
    for (const track of event.streams[0]?.getTracks() ?? [event.track]) {
      if (!remote.getTracks().some((existing) => existing.id === track.id)) remote.addTrack(track);
    }
    onTrack(remote);
  });

  try {
    const locationUrl = await negotiate(peer, readUrl, ownerStreamId);
    return createSession(peer, remote, locationUrl, ownerStreamId);
  } catch (error) {
    peer.close();
    throw error;
  }
}

async function negotiate(peer: RTCPeerConnection, endpoint: string, ownerStreamId: string): Promise<string> {
  const offer = await peer.createOffer();
  await peer.setLocalDescription(offer);
  await waitForIceGathering(peer);
  const response = await fetch(endpoint, {
    method: "POST",
    headers: {
      Accept: "application/sdp",
      "Content-Type": "application/sdp",
      "X-Bigbrother-Session": ownerStreamId,
    },
    body: peer.localDescription?.sdp,
  });
  if (!response.ok) {
    const detail = (await response.text()).trim();
    throw new Error(`Media negotiation failed (${response.status})${detail ? `: ${detail}` : ""}`);
  }
  const locationHeader = response.headers.get("Location");
  if (!locationHeader) throw new Error("Media server did not return a session location.");
  const answer = await response.text();
  await peer.setRemoteDescription({ type: "answer", sdp: answer });
  return new URL(locationHeader, endpoint).toString();
}

function waitForIceGathering(peer: RTCPeerConnection): Promise<void> {
  if (peer.iceGatheringState === "complete") return Promise.resolve();
  return new Promise((resolve, reject) => {
    const timeout = window.setTimeout(() => {
      peer.removeEventListener("icegatheringstatechange", onStateChange);
      reject(new Error("Timed out gathering local network candidates."));
    }, ICE_GATHERING_TIMEOUT_MS);
    const onStateChange = () => {
      if (peer.iceGatheringState === "complete") {
        window.clearTimeout(timeout);
        peer.removeEventListener("icegatheringstatechange", onStateChange);
        resolve();
      }
    };
    peer.addEventListener("icegatheringstatechange", onStateChange);
  });
}

function preferValidatedCodecs(peer: RTCPeerConnection): void {
  for (const transceiver of peer.getTransceivers()) {
    const kind = transceiver.sender.track?.kind;
    if (!kind) continue;
    const capabilities = RTCRtpSender.getCapabilities(kind);
    const mimeType = kind === "video" ? "video/h264" : "audio/opus";
    const preferred = capabilities?.codecs.filter((codec) => codec.mimeType.toLowerCase() === mimeType) ?? [];
    if (preferred.length === 0) throw new Error(`This browser cannot publish the required ${mimeType} codec.`);
    if (typeof transceiver.setCodecPreferences !== "function") {
      throw new Error("This browser cannot select the required media codecs.");
    }
    transceiver.setCodecPreferences(preferred);
  }
}

function createSession(
  peer: RTCPeerConnection,
  stream: MediaStream,
  locationUrl: string,
  ownerStreamId: string,
): MediaSession {
  let closed = false;
  return {
    stream,
    async replaceTrack(kind, track) {
      const sender = peer.getSenders().find((candidate) => candidate.track?.kind === kind);
      if (!sender) throw new Error(`This media session does not publish a ${kind} track.`);
      await sender.replaceTrack(track);
    },
    async close() {
      if (closed) return;
      closed = true;
      peer.close();
      try {
        await fetch(locationUrl, {
          method: "DELETE",
          headers: { "X-Bigbrother-Session": ownerStreamId },
          keepalive: true,
        });
      } catch {
        // The backend also removes outstanding sessions when its owner leaves.
      }
    },
  };
}
