# PTT (Push to talk) Intercom Functionality

## PTT Client

A new standalone headless client that runs on MacOS / Linux. 

Intended Linux clients include but are not limited to: PopOS, RaspberryPi OS.

It streams audio from the existing BIG.BROTHER server to the client's audio output. 

It accepts button inputs from the keyboard. Via a config file, these can be remapped to any keyboard key.

KEYBOARD INPUT 1: "Push to talk" When held down, audio is sent from the microphone and will be heard by other clients. 

KEYBOARD INPUT 2: Increase system audio output volume

KEYBOARD INPUT 3: Decrease system audio output volume

KEYBOARD INPUT 4: Mute/unmute system audio output volume

## PTT Client Usage

Usage: `bigbrother-ptt <hostname or IP address> --friendlyname <user's choice of friendly name>`

`--friendlyname` is optional. If not supplied, default to the client's hostname. 

## Existing Web UI Updates

### Display of Bigbrother-PTT clients

When a `bigbrother-ptt` client is connected, it gets its own tile just like the other connected users... except with no video, obviously.

### New "Push to Talk" Mode When Muted

When a user mutes their own volume, they can hold down the spacebar for "push to talk mode". While the spacebar is depressed they are temporarily unmuted. When the spacebar is released they are returned to a muted state


## Big Brother Server Updates

The existing server software will be updated to support the audio-only `bigbrother-ptt` clients.
