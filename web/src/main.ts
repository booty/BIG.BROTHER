import {
  activeDeviceLabel,
  captureDevice,
  captureInitialMedia,
  enumerateDevices,
  populateDeviceSelect,
  type AvailableDevices,
  type DeviceKind,
} from "./devices";
import { publishMedia, subscribeMedia, type MediaSession } from "./media";
import { loadFriendlyName, saveFriendlyName } from "./preferences";
import { joinRoom, type Participant, type RoomConnection, type RoomMessage } from "./room";
import { createParticipantTile, updateTileParticipant, type TileAction, type TileView } from "./tiles";

interface ReceiverPreferences {
  audioMuted: boolean;
  videoHidden: boolean;
}

const statusElement = document.querySelector<HTMLElement>("#room-status");
const grid = document.querySelector<HTMLElement>("#participant-grid");
const joinForm = document.querySelector<HTMLFormElement>("#join-form");
const nameInput = document.querySelector<HTMLInputElement>("#friendly-name");
const joinButton = document.querySelector<HTMLButtonElement>("#join-button");
if (!statusElement || !grid || !joinForm || !nameInput || !joinButton) {
  throw new Error("The intercom page is missing its room or Join controls.");
}

const tiles = new Map<string, TileView>();
const remoteSessions = new Map<string, MediaSession>();
const openingRemote = new Set<string>();
const receiverPreferences = new Map<string, ReceiverPreferences>();
const latestParticipants = new Map<string, Participant>();
const blockedPlayback = new Set<string>();
let connection: RoomConnection | undefined;
let selfId = "";
let localStream: MediaStream | undefined;
let publication: MediaSession | undefined;
let deviceCatalog: AvailableDevices = { audioInputs: [], videoInputs: [] };
let selectedAudioDeviceId = "";
let selectedVideoDeviceId = "";
let localMediaError = "";
let localMediaNote = "";
let announcedStreamReady = false;
let publicationFailed = false;
let closing = false;
let releasePromise: Promise<void> | undefined;

nameInput.value = loadFriendlyName();
joinButton.textContent = "Join";

function setStatus(message: string, state: "ready" | "working" | "error" = "working"): void {
  statusElement!.textContent = message;
  statusElement!.dataset.state = state;
}

function tileFor(participant: Participant): TileView {
  const existing = tiles.get(participant.id);
  if (existing) {
    updateTileParticipant(existing, participant);
    return existing;
  }
  const self = participant.id === selfId;
  const tile = createParticipantTile(participant, self, {
    onAction: handleTileAction,
    onRename: renameParticipant,
    onDeviceChange: (kind, deviceId) => { void switchDevice(kind, deviceId); },
    onVideoPlay: (participantId) => { void retryPlayback(participantId); },
  });
  tiles.set(participant.id, tile);
  grid!.append(tile.root);
  return tile;
}

function handleRoomMessage(message: RoomMessage): void {
  if (message.type === "welcome" || message.type === "participants") {
    if (message.type === "welcome" && typeof message.payload.selfId === "string") selfId = message.payload.selfId;
    const participants = message.payload.participants;
    if (Array.isArray(participants)) updateParticipants(participants as Participant[]);
  } else if (message.type === "error") {
    setStatus(String(message.payload.message ?? "The room reported an error."), "error");
  }
}

function updateParticipants(participants: Participant[]): void {
  const next = new Map(participants.map((participant) => [participant.id, participant]));
  for (const [id, old] of latestParticipants) {
    const replacement = next.get(id);
    if (!replacement || replacement.streamId !== old.streamId || !replacement.streamReady) {
      blockedPlayback.delete(id);
      const session = remoteSessions.get(id);
      if (session) void session.close();
      remoteSessions.delete(id);
      if (id !== selfId && replacement) {
        const tile = tiles.get(id);
        if (tile) tile.video.srcObject = null;
      }
      if (!replacement) {
        receiverPreferences.delete(id);
        tiles.get(id)?.root.remove();
        tiles.delete(id);
      }
    }
  }
  latestParticipants.clear();
  for (const participant of participants) latestParticipants.set(participant.id, participant);

  for (const participant of participants) {
    const tile = tileFor(participant);
    if (participant.id === selfId) {
      updateSelfTile(tile, participant);
      continue;
    }
    updateRemoteTile(tile, participant);
  }
}

function updateSelfTile(tile: TileView, participant: Participant): void {
  tile.root.classList.add("self");
  tile.root.dataset.audioEnabled = String(participant.audioEnabled);
  tile.root.dataset.videoEnabled = String(participant.videoEnabled);
  tile.root.dataset.streamReady = String(participant.streamReady);
  tile.video.muted = true;
  tile.video.srcObject = localStream ?? null;
  tile.state.textContent = localMediaError || localMediaNote || (publication ? "You are connected" : "Waiting for your media");
  const audioTrack = localStream?.getAudioTracks().find((track) => track.readyState === "live");
  const videoTrack = localStream?.getVideoTracks().find((track) => track.readyState === "live");
  if (tile.audioButton) {
    tile.audioButton.disabled = !audioTrack;
    tile.audioButton.textContent = audioTrack?.enabled ? "Mute microphone" : "Unmute microphone";
    tile.audioButton.setAttribute("aria-pressed", String(audioTrack ? !audioTrack.enabled : true));
  }
  if (tile.videoButton) {
    tile.videoButton.disabled = !videoTrack;
    tile.videoButton.textContent = videoTrack?.enabled ? "Turn camera off" : "Turn camera on";
    tile.videoButton.setAttribute("aria-pressed", String(videoTrack ? !videoTrack.enabled : true));
  }
  if (tile.audioSelect) {
    populateDeviceSelect(tile.audioSelect, "audio", deviceCatalog.audioInputs, selectedAudioDeviceId);
    tile.audioSelect.disabled = !connection;
  }
  if (tile.videoSelect) {
    populateDeviceSelect(tile.videoSelect, "video", deviceCatalog.videoInputs, selectedVideoDeviceId);
    tile.videoSelect.disabled = !connection;
  }
  if (tile.deviceLabels) {
    tile.deviceLabels.textContent = `Camera: ${activeDeviceLabel(videoTrack, deviceCatalog, "video")} · Microphone: ${activeDeviceLabel(audioTrack, deviceCatalog, "audio")}`;
  }
  if (!participant.streamReady && publication) tile.state.textContent = "Publisher is reconnecting";
}

function updateRemoteTile(tile: TileView, participant: Participant): void {
  tile.root.classList.remove("self");
  tile.root.dataset.audioEnabled = String(participant.audioEnabled);
  tile.root.dataset.videoEnabled = String(participant.videoEnabled);
  tile.root.dataset.streamReady = String(participant.streamReady);
  let preferences = receiverPreferences.get(participant.id);
  if (!preferences) {
    preferences = { audioMuted: false, videoHidden: false };
    receiverPreferences.set(participant.id, preferences);
  }
  tile.root.classList.toggle("video-hidden", preferences.videoHidden);
  if (tile.localAudioButton) {
    tile.localAudioButton.textContent = preferences.audioMuted ? "Unmute on this device" : "Mute this participant";
    tile.localAudioButton.setAttribute("aria-pressed", String(preferences.audioMuted));
  }
  if (tile.localVideoButton) {
    tile.localVideoButton.textContent = preferences.videoHidden ? "Show video here" : "Hide video here";
    tile.localVideoButton.setAttribute("aria-pressed", String(preferences.videoHidden));
  }
  if (!participant.streamReady) {
    tile.video.srcObject = null;
    tile.state.textContent = "Waiting for this participant’s media";
    return;
  }
  if (remoteSessions.has(participant.id)) {
    tile.state.textContent = blockedPlayback.has(participant.id)
      ? "Playback blocked; use Join or tap the video to retry"
      : participant.audioEnabled || participant.videoEnabled ? "Connected" : "Media is paused";
    return;
  }
  tile.state.textContent = "Connecting to media…";
  openRemote(participant);
}

function openRemote(participant: Participant): void {
  if (openingRemote.has(participant.id) || remoteSessions.has(participant.id) || !participant.streamReady) return;
  const ownerStreamId = latestParticipants.get(selfId)?.streamId;
  if (!ownerStreamId) return;
  openingRemote.add(participant.id);
  void subscribeMedia(participant.readUrl, ownerStreamId, (stream) => {
    const preferences = receiverPreferences.get(participant.id) ?? { audioMuted: false, videoHidden: false };
    for (const track of stream.getAudioTracks()) track.enabled = !preferences.audioMuted;
    for (const track of stream.getVideoTracks()) track.enabled = !preferences.videoHidden;
    const tile = tiles.get(participant.id);
    if (tile) {
      tile.video.srcObject = stream;
      void tile.video.play().catch(() => {
        blockedPlayback.add(participant.id);
        tile.state.textContent = "Media is ready; use Join or tap the video to play";
      });
    }
  }, (state) => {
    const tile = tiles.get(participant.id);
    if (tile && state === "disconnected") tile.state.textContent = "Media connection interrupted…";
    if (state === "connected" && tile) tile.state.textContent = "Connected";
    if (state === "failed") {
      if (tile) tile.state.textContent = "Media connection failed";
      const session = remoteSessions.get(participant.id);
      if (session) {
        remoteSessions.delete(participant.id);
        void session.close();
      }
      scheduleRemoteRetry(participant.id);
    }
  }).then((session) => {
    if (closing || latestParticipants.get(participant.id)?.streamId !== participant.streamId) {
      void session.close();
      return;
    }
    remoteSessions.set(participant.id, session);
    const tile = tiles.get(participant.id);
    if (tile && !blockedPlayback.has(participant.id)) tile.state.textContent = "Connected";
  }).catch((error: unknown) => {
    const tile = tiles.get(participant.id);
    if (tile) tile.state.textContent = `Media unavailable: ${errorMessage(error)}`;
    scheduleRemoteRetry(participant.id);
  }).finally(() => {
    openingRemote.delete(participant.id);
  });
}

function scheduleRemoteRetry(participantId: string): void {
  window.setTimeout(() => {
    const current = latestParticipants.get(participantId);
    if (current?.streamReady && !remoteSessions.has(participantId)) openRemote(current);
  }, 2_000);
}

function handleTileAction(participantId: string, action: TileAction): void {
  if (participantId === selfId && (action === "toggle-audio" || action === "toggle-video")) {
    const kind = action === "toggle-audio" ? "audio" : "video";
    const track = localStream?.getTracks().find((candidate) => candidate.kind === kind && candidate.readyState === "live");
    if (!track) return;
    track.enabled = !track.enabled;
    announceMediaState();
    updateParticipants([...latestParticipants.values()]);
    return;
  }

  const preferences = receiverPreferences.get(participantId) ?? { audioMuted: false, videoHidden: false };
  if (action === "toggle-local-audio") preferences.audioMuted = !preferences.audioMuted;
  if (action === "toggle-local-video") preferences.videoHidden = !preferences.videoHidden;
  receiverPreferences.set(participantId, preferences);
  const session = remoteSessions.get(participantId);
  for (const track of session?.stream.getAudioTracks() ?? []) track.enabled = !preferences.audioMuted;
  for (const track of session?.stream.getVideoTracks() ?? []) track.enabled = !preferences.videoHidden;
  updateParticipants([...latestParticipants.values()]);
}

function renameParticipant(participantId: string, name: string): void {
  const normalized = saveFriendlyName(name);
  nameInput!.value = normalized;
  connection?.send("rename", { name: normalized });
}

function announceMediaState(): void {
  const audioTrack = localStream?.getAudioTracks().find((track) => track.readyState === "live");
  const videoTrack = localStream?.getVideoTracks().find((track) => track.readyState === "live");
  connection?.send("mediaState", {
    audioEnabled: Boolean(audioTrack?.enabled),
    videoEnabled: Boolean(videoTrack?.enabled),
  });
}

function announceStreamReady(ready: boolean): void {
  if (announcedStreamReady === ready) return;
  announcedStreamReady = ready;
  connection?.send("streamState", { ready });
}

async function joinOrRetry(): Promise<void> {
  joinButton!.disabled = true;
  setStatus(connection ? "Retrying media and playback…" : "Connecting to the room…");
  try {
    if (!connection) {
      await releasePromise;
      closing = false;
      announcedStreamReady = false;
      const friendlyName = saveFriendlyName(nameInput!.value);
      nameInput!.value = friendlyName;
      connection = await joinRoom(friendlyName, handleRoomMessage, handleRoomClose);
      joinButton!.textContent = "Retry media";
      nameInput!.disabled = true;
      const ownParticipant = latestParticipants.get(connection.selfId);
      if (!ownParticipant) throw new Error("The room joined without returning this participant’s media descriptor.");
      setStatus("Joined the room. Requesting available devices…", "ready");
    }
    if (publication && !publicationFailed) {
      await retryPlayback();
      setStatus("You are connected to the room.", "ready");
      return;
    }
    await startLocalMedia();
  } catch (error) {
    setStatus(errorMessage(error), "error");
  } finally {
    joinButton!.disabled = false;
  }
}

async function startLocalMedia(): Promise<void> {
  const ownParticipant = latestParticipants.get(selfId);
  if (!ownParticipant) throw new Error("The room did not return this participant’s media descriptor.");
  localMediaError = "";
  localMediaNote = "";
  try {
    if (!localStream || localStream.getTracks().every((track) => track.readyState !== "live")) {
      const capture = await captureInitialMedia(selectedAudioDeviceId, selectedVideoDeviceId);
      localStream = capture.stream;
      if (capture.errors.length > 0) localMediaNote = "One device is unavailable; using the available media.";
      watchLocalTracks(capture.stream);
    }
    const tracks = localStream.getTracks().filter((track) => track.readyState === "live");
    if (tracks.length === 0) throw new Error("No camera or microphone is available. Connect a device and retry.");
    const tile = tiles.get(selfId);
    if (tile) tile.video.srcObject = localStream;
    await refreshDeviceLists();
    announceMediaState();
    setStatus("Publishing available camera and microphone…", "working");
    publication = await createPublication(ownParticipant);
    announceStreamReady(true);
    announceMediaState();
    setStatus(localMediaError || localMediaNote || "You are connected to the room.", localMediaError || localMediaNote ? "error" : "ready");
    updateParticipants([...latestParticipants.values()]);
    await retryPlayback();
  } catch (error) {
    localMediaError = errorMessage(error);
    announceMediaState();
    announceStreamReady(false);
    setStatus(`Room is connected; media is unavailable: ${localMediaError}`, "error");
    updateParticipants([...latestParticipants.values()]);
  }
}

async function createPublication(participant: Participant): Promise<MediaSession> {
  const stream = localStream;
  if (!stream) throw new Error("No local media is available to publish.");
  publicationFailed = false;
  const session = await publishMedia(participant.publishUrl, participant.streamId, stream, (state) => {
    if (state === "failed") {
      publicationFailed = true;
      const failedPublication = publication;
      publication = undefined;
      if (failedPublication) void failedPublication.close();
      localMediaError = "The media connection failed. Retry media to reconnect.";
      announceStreamReady(false);
      setStatus(localMediaError, "error");
      updateParticipants([...latestParticipants.values()]);
    } else if (state === "connected") {
      publicationFailed = false;
      localMediaError = "";
      announceStreamReady(true);
      setStatus(localMediaNote || "You are connected to the room.", localMediaNote ? "error" : "ready");
      updateParticipants([...latestParticipants.values()]);
    } else if (state === "disconnected") {
      setStatus("Your media connection was interrupted…", "error");
    }
  });
  if (publicationFailed) {
    await session.close();
    throw new Error("The media connection failed during publication.");
  }
  return session;
}

function watchLocalTracks(stream: MediaStream): void {
  for (const track of stream.getTracks()) {
    track.addEventListener("ended", () => {
      if (!localStream?.getTracks().includes(track)) return;
      localMediaError = `The ${track.kind === "audio" ? "microphone" : "camera"} was disconnected.`;
      announceMediaState();
      if (stream.getTracks().every((candidate) => candidate.readyState !== "live")) {
        void publication?.close();
        publication = undefined;
        announceStreamReady(false);
      }
      setStatus(`${localMediaError} The room remains connected; select another device to continue.`, "error");
      void refreshDeviceLists();
      updateParticipants([...latestParticipants.values()]);
    }, { once: true });
  }
}

async function switchDevice(kind: DeviceKind, deviceId: string): Promise<void> {
  if (!connection) return;
  const oldStream = localStream ?? new MediaStream();
  const oldTracks = kind === "audio" ? oldStream.getAudioTracks() : oldStream.getVideoTracks();
  const oldTrack = oldTracks.find((track) => track.readyState === "live");
  let newTrack: MediaStreamTrack | undefined;
  try {
    newTrack = await captureDevice(kind, deviceId);
    if (oldTrack) newTrack.enabled = oldTrack.enabled;
    if (publication && oldTrack) {
      await publication.replaceTrack(kind, newTrack);
      oldStream.removeTrack(oldTrack);
      oldStream.addTrack(newTrack);
      oldTrack.stop();
      localStream = oldStream;
    } else {
      await republishWithDevice(oldStream, newTrack, kind);
    }
    if (kind === "audio") selectedAudioDeviceId = deviceId;
    else selectedVideoDeviceId = deviceId;
    localMediaError = "";
    localMediaNote = "";
    watchLocalTracks(new MediaStream([newTrack]));
    announceMediaState();
    await refreshDeviceLists();
    updateParticipants([...latestParticipants.values()]);
    setStatus("Device changed.", "ready");
  } catch (error) {
    if (newTrack && newTrack.readyState === "live" && !localStream?.getTracks().includes(newTrack)) newTrack.stop();
    localMediaError = `Could not switch ${kind === "audio" ? "microphone" : "camera"}: ${errorMessage(error)}`;
    await refreshDeviceLists();
    updateParticipants([...latestParticipants.values()]);
    setStatus(`${localMediaError} The current room and media remain available.`, "error");
  }
}

async function republishWithDevice(oldStream: MediaStream, newTrack: MediaStreamTrack, kind: DeviceKind): Promise<void> {
  const ownParticipant = latestParticipants.get(selfId);
  if (!ownParticipant) throw new Error("The room participant is no longer available.");
  const nextStream = new MediaStream([
    ...oldStream.getTracks().filter((track) => track.kind !== kind && track.readyState === "live"),
    newTrack,
  ]);
  const oldPublication = publication;
  if (oldPublication) {
    announceStreamReady(false);
    await oldPublication.close();
    publication = undefined;
  }
  try {
    localStream = nextStream;
    publication = await createPublication(ownParticipant);
    for (const track of oldStream.getTracks()) {
      if (track.kind === kind && track !== newTrack) track.stop();
    }
    announceStreamReady(true);
  } catch (error) {
    localStream = oldStream;
    newTrack.stop();
    if (oldPublication && oldStream.getTracks().some((track) => track.readyState === "live")) {
      try {
        publication = await createPublication(ownParticipant);
        announceStreamReady(true);
      } catch {
        publication = undefined;
        announceStreamReady(false);
      }
    }
    throw error;
  }
}

async function refreshDeviceLists(): Promise<void> {
  try {
    deviceCatalog = await enumerateDevices();
  } catch {
    deviceCatalog = { audioInputs: [], videoInputs: [] };
  }
  const self = latestParticipants.get(selfId);
  const tile = self ? tiles.get(self.id) : undefined;
  if (self && tile) updateSelfTile(tile, self);
}

async function retryPlayback(participantId?: string): Promise<void> {
  if (participantId) {
    const tile = tiles.get(participantId);
    if (tile) {
      try {
        await tile.video.play();
        blockedPlayback.delete(participantId);
        tile.state.textContent = "Connected";
      } catch {
        blockedPlayback.add(participantId);
        tile.state.textContent = "Playback is blocked; use Join or tap the video to retry.";
      }
    }
    return;
  }
  await Promise.all([...tiles.entries()].map(async ([id, tile]) => {
    try {
      await tile.video.play();
      blockedPlayback.delete(id);
      const participant = latestParticipants.get(id);
      tile.state.textContent = participant?.streamReady ? "Connected" : "Waiting for media";
    } catch {
      blockedPlayback.add(id);
      tile.state.textContent = "Playback is blocked; tap the video or use Join to retry.";
    }
  }));
}

function handleRoomClose(): void {
  if (closing) return;
  connection = undefined;
  nameInput!.disabled = false;
  joinButton!.disabled = false;
  joinButton!.textContent = "Reconnect";
  setStatus("Room connection closed. Reconnect to rejoin.", "error");
  latestParticipants.clear();
  tiles.clear();
  grid!.replaceChildren();
  releasePromise = releaseMedia();
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

async function releaseMedia(closeRoom = false): Promise<void> {
  if (closing) return;
  closing = true;
  const currentPublication = publication;
  publication = undefined;
  await currentPublication?.close();
  await Promise.all([...remoteSessions.values()].map((session) => session.close()));
  remoteSessions.clear();
  for (const track of localStream?.getTracks() ?? []) track.stop();
  localStream = undefined;
  if (closeRoom) {
    connection?.close();
    connection = undefined;
  }
}

joinForm.addEventListener("submit", (event) => {
  event.preventDefault();
  void joinOrRetry();
});

navigator.mediaDevices?.addEventListener?.("devicechange", () => { void refreshDeviceLists(); });
window.addEventListener("pagehide", () => { void releaseMedia(true); }, { once: true });
setStatus("Choose a name and join when you are ready.", "ready");
void refreshDeviceLists();
