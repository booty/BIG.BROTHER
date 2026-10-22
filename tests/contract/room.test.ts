import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { once } from "node:events";
import test from "node:test";
import WebSocket from "ws";

type Message = { type: string; payload: Record<string, unknown> };
type Participant = {
  id: string;
  name: string;
  audioEnabled: boolean;
  videoEnabled: boolean;
  streamId: string;
  publishUrl: string;
  readUrl: string;
  streamReady: boolean;
};

const baseURL = process.env.BASE_URL ?? "https://127.0.0.1:8443";
const websocketURL = baseURL.replace(/^http/, "ws").replace(/\/$/, "") + "/ws";
const tlsOptions = process.env.CA_CERT ? { ca: readFileSync(process.env.CA_CERT) } : {};

function connect(): WebSocket {
  return new WebSocket(websocketURL, tlsOptions);
}

function waitForMessage(socket: WebSocket, type: string, timeoutMs = 5_000): Promise<Message> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      socket.off("message", onMessage);
      reject(new Error(`timed out waiting for ${type} message`));
    }, timeoutMs);
    const onMessage = (data: WebSocket.RawData) => {
      const message = JSON.parse(data.toString()) as Message;
      if (message.type !== type) return;
      clearTimeout(timer);
      socket.off("message", onMessage);
      resolve(message);
    };
    socket.on("message", onMessage);
  });
}

async function joinedClient(name: string): Promise<{ socket: WebSocket; welcome: Message }> {
  const socket = connect();
  await once(socket, "open");
  const welcome = waitForMessage(socket, "welcome");
  const initialSnapshot = waitForMessage(socket, "participants");
  socket.send(JSON.stringify({ type: "join", payload: { name } }));
  const joined = await welcome;
  const snapshot = await initialSnapshot;
  assert.deepEqual(snapshot.payload.participants, joined.payload.participants);
  return { socket, welcome: joined };
}

test("room snapshots, rename, media state, disconnect, reconnect, and malformed JSON", async (t) => {
  const first = await joinedClient("Kitchen");
  t.after(() => first.socket.close());
  const firstId = first.welcome.payload.selfId as string;
  const firstSnapshot = first.welcome.payload.participants as Participant[];
  assert.equal(firstSnapshot.length, 1);
  assert.equal(firstSnapshot[0].name, "Kitchen");
  assert.match(firstSnapshot[0].streamId, /^[a-f0-9]{24}$/);
  assert.equal(firstSnapshot[0].publishUrl, `${new URL(baseURL).origin}/media/${firstSnapshot[0].streamId}/whip`);
  assert.equal(firstSnapshot[0].readUrl, `${new URL(baseURL).origin}/media/${firstSnapshot[0].streamId}/whep`);
  assert.equal(firstSnapshot[0].streamReady, false);

  const firstAfterSecondPromise = waitForMessage(first.socket, "participants");
  const second = await joinedClient("Hallway");
  t.after(() => second.socket.close());
  const secondId = second.welcome.payload.selfId as string;
  const secondSnapshot = second.welcome.payload.participants as Participant[];
  assert.equal(secondSnapshot.length, 2);
  assert.deepEqual(secondSnapshot.map(({ id }) => id).sort(), [firstId, secondId].sort());
  const firstAfterSecond = await firstAfterSecondPromise;
  assert.deepEqual(firstAfterSecond.payload.participants, secondSnapshot);

  const firstRename = waitForMessage(first.socket, "participants");
  const secondRename = waitForMessage(second.socket, "participants");
  first.socket.send(JSON.stringify({ type: "rename", payload: { name: "Dining room" } }));
  const renameSnapshot = (await firstRename).payload.participants as Participant[];
  await secondRename;
  assert.equal(renameSnapshot.find(({ id }) => id === firstId)?.name, "Dining room");

  const firstMedia = waitForMessage(first.socket, "participants");
  const secondMedia = waitForMessage(second.socket, "participants");
  first.socket.send(JSON.stringify({ type: "mediaState", payload: { audioEnabled: false, videoEnabled: true } }));
  const mediaSnapshot = (await firstMedia).payload.participants as Participant[];
  await secondMedia;
  assert.equal(mediaSnapshot.find(({ id }) => id === firstId)?.audioEnabled, false);
  assert.equal(mediaSnapshot.find(({ id }) => id === firstId)?.videoEnabled, true);

  const firstStream = waitForMessage(first.socket, "participants");
  const secondStream = waitForMessage(second.socket, "participants");
  first.socket.send(JSON.stringify({ type: "streamState", payload: { ready: true } }));
  const readySnapshot = (await firstStream).payload.participants as Participant[];
  await secondStream;
  assert.equal(readySnapshot.find(({ id }) => id === firstId)?.streamReady, true);

  const secondDisconnect = waitForMessage(first.socket, "participants");
  second.socket.close();
  await once(second.socket, "close");
  const afterDisconnect = (await secondDisconnect).payload.participants as Participant[];
  assert.deepEqual(afterDisconnect.map(({ id }) => id), [firstId]);

  const third = await joinedClient("Porch");
  t.after(() => third.socket.close());
  const thirdId = third.welcome.payload.selfId as string;
  assert.notEqual(thirdId, secondId);
  assert.equal((third.welcome.payload.participants as Participant[]).length, 2);

  const malformedError = waitForMessage(third.socket, "error");
  third.socket.send("{");
  const error = await malformedError;
  assert.equal(typeof error.payload.message, "string");

  const thirdRename = waitForMessage(third.socket, "participants");
  third.socket.send(JSON.stringify({ type: "rename", payload: { name: "Still connected" } }));
  const stillConnected = (await thirdRename).payload.participants as Participant[];
  assert.equal(stillConnected.find(({ id }) => id === thirdId)?.name, "Still connected");
  assert.ok(stillConnected.some(({ id }) => id === firstId), "malformed JSON should not remove the other participant");
});
