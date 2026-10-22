export interface AvailableDevices {
  audioInputs: MediaDeviceInfo[];
  videoInputs: MediaDeviceInfo[];
}

export type DeviceKind = "audio" | "video";

export async function enumerateDevices(): Promise<AvailableDevices> {
  if (!navigator.mediaDevices?.enumerateDevices) return { audioInputs: [], videoInputs: [] };
  const devices = await navigator.mediaDevices.enumerateDevices();
  return {
    audioInputs: devices.filter((device) => device.kind === "audioinput" && device.deviceId !== "default"),
    videoInputs: devices.filter((device) => device.kind === "videoinput" && device.deviceId !== "default"),
  };
}

export async function captureInitialMedia(audioDeviceId = "", videoDeviceId = "", signal?: AbortSignal): Promise<{ stream: MediaStream; errors: string[] }> {
  if (!navigator.mediaDevices?.getUserMedia) throw new Error("This browser does not provide camera or microphone access.");
  const constraints: MediaStreamConstraints = {
    audio: deviceConstraint(audioDeviceId),
    video: deviceConstraint(videoDeviceId),
  };
  const capturedStreams: MediaStream[] = [];
  const stopCaptured = () => {
    for (const stream of capturedStreams) for (const track of stream.getTracks()) track.stop();
  };
  const capture = async (requested: MediaStreamConstraints): Promise<MediaStream> => {
    signal?.throwIfAborted();
    const stream = await navigator.mediaDevices.getUserMedia(requested);
    capturedStreams.push(stream);
    if (signal?.aborted) {
      stopCaptured();
      signal.throwIfAborted();
    }
    return stream;
  };
  signal?.addEventListener("abort", stopCaptured, { once: true });
  try {
    try {
      return { stream: await capture(constraints), errors: [] };
    } catch (bothError) {
      signal?.throwIfAborted();
      const [audioResult, videoResult] = await Promise.allSettled([
        capture({ audio: deviceConstraint(audioDeviceId), video: false }),
        capture({ audio: false, video: deviceConstraint(videoDeviceId) }),
      ]);
      const stream = new MediaStream();
      if (audioResult.status === "fulfilled") {
        for (const track of audioResult.value.getAudioTracks()) stream.addTrack(track);
      }
      if (videoResult.status === "fulfilled") {
        for (const track of videoResult.value.getVideoTracks()) stream.addTrack(track);
      }
      if (stream.getTracks().length === 0) {
        throw new Error(`No camera or microphone is available: ${errorMessage(bothError)}`);
      }
      return {
        stream,
        errors: [audioResult, videoResult]
          .filter((result): result is PromiseRejectedResult => result.status === "rejected")
          .map((result) => errorMessage(result.reason)),
      };
    }
  } finally {
    signal?.removeEventListener("abort", stopCaptured);
  }
}

export async function captureDevice(kind: DeviceKind, deviceId: string): Promise<MediaStreamTrack> {
  if (!navigator.mediaDevices?.getUserMedia) throw new Error("This browser does not provide camera or microphone access.");
  const stream = await navigator.mediaDevices.getUserMedia({
    audio: kind === "audio" ? deviceConstraint(deviceId) : false,
    video: kind === "video" ? deviceConstraint(deviceId) : false,
  });
  const track = kind === "audio" ? stream.getAudioTracks()[0] : stream.getVideoTracks()[0];
  if (!track) throw new Error(`The selected ${kind === "audio" ? "microphone" : "camera"} did not produce a media track.`);
  return track;
}

export function populateDeviceSelect(
  select: HTMLSelectElement,
  kind: DeviceKind,
  devices: MediaDeviceInfo[],
  activeDeviceId: string,
): void {
  const label = kind === "audio" ? "Default microphone" : "Default camera";
  select.replaceChildren(new Option(label, ""));
  for (const device of devices) {
    select.add(new Option(device.label || `${kind === "audio" ? "Microphone" : "Camera"} ${select.length}`, device.deviceId));
  }
  if (activeDeviceId && !Array.from(select.options).some((option) => option.value === activeDeviceId)) {
    select.add(new Option("Disconnected device", activeDeviceId));
  }
  select.value = activeDeviceId;
}

export function activeDeviceLabel(track: MediaStreamTrack | undefined, devices: AvailableDevices, kind: DeviceKind): string {
  if (!track) return kind === "audio" ? "No microphone" : "No camera";
  const deviceId = track.getSettings().deviceId;
  const list = kind === "audio" ? devices.audioInputs : devices.videoInputs;
  const device = list.find((candidate) => candidate.deviceId === deviceId);
  return device?.label || track.label || (kind === "audio" ? "Default microphone" : "Default camera");
}

function deviceConstraint(deviceId: string): boolean | MediaTrackConstraints {
  return deviceId ? { deviceId: { exact: deviceId } } : true;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
