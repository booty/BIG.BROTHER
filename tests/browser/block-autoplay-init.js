(() => {
  const originalPlay = HTMLMediaElement.prototype.play;
  window.__bbBlockRemotePlayback = true;
  HTMLMediaElement.prototype.play = function (...args) {
    if (this instanceof HTMLVideoElement && !this.muted && window.__bbBlockRemotePlayback) {
      return Promise.reject(new DOMException("Playback requires a user gesture", "NotAllowedError"));
    }
    return originalPlay.apply(this, args);
  };
})();
