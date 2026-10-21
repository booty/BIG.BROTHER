# BIG.BROTHER Overview

We are creating a “toy” web based “video intercom” system for our home.

Simplicity of use is a priority.

- DO NOT OVER-ENGINEER THIS PROJECT:
  - This is a “toy” app for myself and my housemates
  - We are not taking this app to market
  - We are not selling this app
  - We do not need mission-critical reliability

## Hardware

The host server will ideally be a relatively low-powered device like a RaspberryPi 5 or an AMD A9 APU netbook running Linux. If these are truly not powerful enough, we can use an old 7th-gen Intel i5 laptop running Linux.

For development purposes, the host should be able to run on MacOS as well.

## Server Interface Overview

User simply lanches the app with `bigbrother —port 1234`

If no port is supplied:
- Default to port 80 if it is available. 
- If 80 not available, find another sensible port that doesn’t conflict with other well known services
- Let the user know what IP and port were chosen.

Log output to stdout when noteworthy events happen on the server. For each event, show the client’s IP, as well as the host IP and port of the server process.

Loggable events:
* A client connects
* A client disconnects


## Client Interface Overview

The overall interface is a simplified version of basic videoconferencing in popular video chat interfaces like Zoom or Google Meet. 

Users navigate to the host’s IP and port in a web browser.

They see a tiled view of all connected clients, including themselves. Tiles should fill the entire browser window as much as possible.

Users are automatically logged into the chat and camera and mic are activated if possible

Each tile shows:
- The video feed from that client, obviously
- The client’s friendly name, or 
- If it’s somebody else’s tile:
  - Controls to mute their audio and turn off their video (they keep transmitting to the server)
- If it’s “my” tile
  - The active camera/mic device names
  - ability to mute my mic or turn my video off
  - a “settings” button to change camera/mic
  - a pencil “edit” icon next to my friendly name, so that I can edit it

If a particular client supplies a friendly name, it should be remembered and the client should default to that friendly name next time they connect.

## Client Environments

Browsers: Support Firefox, Chrome, and Safari. Browsers can be mobile or desktop.

## Security

Do not implement any security in the application itself. The network itself is the security boundary. Any authenticated client on my wifi or ethernet network can use this application.

## Other Notes


- An unlimited number of devices can join the chat. If we eventually discover limits to the system we can hard code in a configurable limit later. 
- No app-level authentication is required. Our wifi network itself is the security boundary — anybody on our network can connect. 
- FWIW, we do not anticipate more than 6 simultaneously connected clients.  Typical will be more like 1-3 clients
- This application MUST function even if there is no internet access. As long as the home’s network is up, it should function. Any required client javascript libraries must be vendored and served by the server application


## Future Functionality

These are features we’ll implement in future versions of the application. They are mentioned here for the planning agent’s benefit, in case they impact our foundational architecture choices

- Ability to display/record audio and video from IP cameras that aren’t connected via a web client
- Ability to display/record audio and video from headless clients (ie, a RPi) that aren’t connected via a web client
- “Security cam” functionality: recording continuous 24/7 video is way out of scope for this project and hardware. However, we’d like some basic functionality such as “If client A’s camera detects motion after not detecting motion for B minutes, server should record C minutes of video and/or still snapshots D seconds apart”
  - A web based interface to view this video or the snapshots
- A web admin interface to rename (give new friendly names to) clients and change other settings
- A simple text chat interface, with messages persisted on server
- Clients should be able to move/resize the tiles for connected clients freely
