export interface Participant {
  id: string;
  name: string;
  audioEnabled: boolean;
  videoEnabled: boolean;
  streamId: string;
  publishUrl: string;
  readUrl: string;
  streamReady: boolean;
}

export interface RoomMessage {
  type: string;
  payload: Record<string, unknown>;
}

export interface RoomConnection {
  selfId: string;
  send(type: string, payload: Record<string, unknown>): void;
  close(): void;
}

export function joinRoom(
  name: string,
  onMessage: (message: RoomMessage) => void,
  onClose: () => void,
): Promise<RoomConnection> {
  const scheme = location.protocol === "https:" ? "wss:" : "ws:";
  const socket = new WebSocket(`${scheme}//${location.host}/ws`);

  return new Promise((resolve, reject) => {
    let joined = false;
    const timeout = window.setTimeout(() => {
      if (!joined) {
        socket.close();
        reject(new Error("The room did not respond to the join request."));
      }
    }, 10_000);

    socket.addEventListener("open", () => {
      socket.send(JSON.stringify({ type: "join", payload: { name } }));
    }, { once: true });

    socket.addEventListener("message", (event: MessageEvent<string>) => {
      let message: RoomMessage;
      try {
        message = JSON.parse(event.data) as RoomMessage;
      } catch {
        return;
      }
      onMessage(message);
      if (!joined && message.type === "welcome" && typeof message.payload.selfId === "string") {
        joined = true;
        window.clearTimeout(timeout);
        resolve({
          selfId: message.payload.selfId,
          send(type, payload) {
            if (socket.readyState === WebSocket.OPEN) {
              socket.send(JSON.stringify({ type, payload }));
            }
          },
          close() {
            socket.close();
          },
        });
      } else if (!joined && message.type === "error") {
        window.clearTimeout(timeout);
        reject(new Error(String(message.payload.message ?? "The room rejected the join request.")));
      }
    });

    socket.addEventListener("error", () => {
      if (!joined) {
        window.clearTimeout(timeout);
        reject(new Error("Could not connect to the room."));
      }
    });
    socket.addEventListener("close", () => {
      window.clearTimeout(timeout);
      onClose();
      if (!joined) reject(new Error("The room connection closed before joining."));
    });
  });
}
