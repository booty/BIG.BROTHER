(() => {
  const original = navigator.mediaDevices;
  const getUserMedia = original.getUserMedia.bind(original);
  const enumerateDevices = original.enumerateDevices.bind(original);
  Object.defineProperty(navigator, "mediaDevices", {
    configurable: true,
    value: {
      getUserMedia(constraints = {}) {
        if (constraints.video) return Promise.reject(new DOMException("No camera is connected", "NotFoundError"));
        return getUserMedia({ audio: constraints.audio, video: false });
      },
      enumerateDevices() {
        return enumerateDevices().then((devices) => devices.filter((device) => device.kind === "audioinput"));
      },
      addEventListener: original.addEventListener.bind(original),
    },
  });
})();
