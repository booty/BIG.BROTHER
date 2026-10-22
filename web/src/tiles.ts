import type { DeviceKind } from "./devices";
import type { Participant } from "./room";

export type TileAction = "toggle-audio" | "toggle-video" | "toggle-local-audio" | "toggle-local-video";

export interface TileCallbacks {
  onAction(participantId: string, action: TileAction): void;
  onRename(participantId: string, name: string): void;
  onDeviceChange(kind: DeviceKind, deviceId: string): void;
  onVideoPlay(participantId: string): void;
}

export interface TileView {
  root: HTMLElement;
  video: HTMLVideoElement;
  name: HTMLElement;
  state: HTMLElement;
  deviceLabels?: HTMLElement;
  audioButton?: HTMLButtonElement;
  videoButton?: HTMLButtonElement;
  localAudioButton?: HTMLButtonElement;
  localVideoButton?: HTMLButtonElement;
  audioSelect?: HTMLSelectElement;
  videoSelect?: HTMLSelectElement;
  nameInput?: HTMLInputElement;
  editButton?: HTMLButtonElement;
}

export function createParticipantTile(
  participant: Participant,
  self: boolean,
  callbacks: TileCallbacks,
): TileView {
  const root = document.createElement("article");
  root.className = self ? "tile self" : "tile";
  root.dataset.participantId = participant.id;

  const video = document.createElement("video");
  video.autoplay = true;
  video.playsInline = true;
  video.muted = self;
  video.setAttribute("aria-label", `${participant.name} video`);
  video.addEventListener("click", () => callbacks.onVideoPlay(participant.id));

  const details = document.createElement("div");
  details.className = "tile-details";
  const nameRow = document.createElement("div");
  nameRow.className = "name-row";
  const name = document.createElement("strong");
  name.textContent = participant.name;
  nameRow.append(name);
  let nameInput: HTMLInputElement | undefined;
  let editButton: HTMLButtonElement | undefined;
  let saveNameButton: HTMLButtonElement | undefined;
  if (self) {
    editButton = makeButton("Edit name", "edit-name");
    editButton.textContent = "✎";
    editButton.addEventListener("click", () => {
      if (!nameInput || !saveNameButton) return;
      nameInput.value = name.textContent ?? "";
      nameInput.hidden = false;
      editButton!.hidden = true;
      saveNameButton.hidden = false;
      nameInput.focus();
      nameInput.select();
    });
    nameInput = document.createElement("input");
    nameInput.className = "name-editor";
    nameInput.setAttribute("aria-label", "Friendly name");
    nameInput.maxLength = 48;
    nameInput.hidden = true;
    saveNameButton = makeButton("Save name", "save-name");
    saveNameButton.textContent = "Save";
    saveNameButton.hidden = true;
    saveNameButton.addEventListener("click", () => {
      callbacks.onRename(participant.id, nameInput!.value);
      nameInput!.hidden = true;
      saveNameButton!.hidden = true;
      editButton!.hidden = false;
    });
    nameInput.addEventListener("keydown", (event) => {
      if (event.key === "Enter") saveNameButton!.click();
      if (event.key === "Escape") {
        nameInput!.hidden = true;
        saveNameButton!.hidden = true;
        editButton!.hidden = false;
      }
    });
    nameRow.append(editButton, nameInput, saveNameButton);
  }

  const state = document.createElement("span");
  state.className = "tile-state";
  const deviceLabels = self ? document.createElement("span") : undefined;
  if (deviceLabels) deviceLabels.className = "device-labels";
  details.append(nameRow, state);
  if (deviceLabels) details.append(deviceLabels);

  const controls = document.createElement("div");
  controls.className = "tile-controls";
  let audioButton: HTMLButtonElement | undefined;
  let videoButton: HTMLButtonElement | undefined;
  let localAudioButton: HTMLButtonElement | undefined;
  let localVideoButton: HTMLButtonElement | undefined;
  let audioSelect: HTMLSelectElement | undefined;
  let videoSelect: HTMLSelectElement | undefined;
  if (self) {
    audioButton = makeButton("Mute microphone", "toggle-audio");
    videoButton = makeButton("Turn camera off", "toggle-video");
    audioButton.addEventListener("click", () => callbacks.onAction(participant.id, "toggle-audio"));
    videoButton.addEventListener("click", () => callbacks.onAction(participant.id, "toggle-video"));
    const audioDevice = makeDeviceSelect("Microphone", "audio-device", "audio");
    const videoDevice = makeDeviceSelect("Camera", "video-device", "video");
    audioSelect = audioDevice.select;
    videoSelect = videoDevice.select;
    audioSelect.addEventListener("change", () => callbacks.onDeviceChange("audio", audioSelect!.value));
    videoSelect.addEventListener("change", () => callbacks.onDeviceChange("video", videoSelect!.value));
    controls.append(audioButton, videoButton, audioDevice.wrapper, videoDevice.wrapper);
  } else {
    localAudioButton = makeButton("Mute this participant", "toggle-local-audio");
    localVideoButton = makeButton("Hide this participant’s video", "toggle-local-video");
    localAudioButton.addEventListener("click", () => callbacks.onAction(participant.id, "toggle-local-audio"));
    localVideoButton.addEventListener("click", () => callbacks.onAction(participant.id, "toggle-local-video"));
    controls.append(localAudioButton, localVideoButton);
  }

  root.append(video, details, controls);
  return {
    root,
    video,
    name,
    state,
    deviceLabels,
    audioButton,
    videoButton,
    localAudioButton,
    localVideoButton,
    audioSelect,
    videoSelect,
    nameInput,
    editButton,
  };
}

export function updateTileParticipant(tile: TileView, participant: Participant): void {
  tile.name.textContent = participant.name;
  tile.video.setAttribute("aria-label", `${participant.name} video`);
  tile.root.dataset.participantId = participant.id;
}

function makeButton(label: string, action: string): HTMLButtonElement {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "control-button";
  button.dataset.action = action;
  button.setAttribute("aria-label", label);
  button.textContent = label;
  return button;
}

function makeDeviceSelect(label: string, id: string, kind: DeviceKind): { wrapper: HTMLLabelElement; select: HTMLSelectElement } {
  const wrapper = document.createElement("label");
  wrapper.className = "device-select";
  wrapper.textContent = label;
  const select = document.createElement("select");
  select.id = id;
  select.dataset.deviceKind = kind;
  wrapper.append(select);
  return { wrapper, select };
}
