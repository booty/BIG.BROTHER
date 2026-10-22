(() => {
  const denied = {
    getUserMedia: async () => { throw new DOMException("Permission denied", "NotAllowedError"); },
    enumerateDevices: async () => [],
    addEventListener: () => {},
  };
  Object.defineProperty(navigator, "mediaDevices", { configurable: true, value: denied });
})();
