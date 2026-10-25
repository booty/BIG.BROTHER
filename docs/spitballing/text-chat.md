# BIG.BROTHER Text Chat

## Vocabulary

BB = BIG.BROTHER (our application)

## Overview

We'll add a simple text chat feature.

The web UI will be a simplified version of the text chat feature available in Google Meet or Zoom.

The right side of the BB SPA will be the chat column. Default width is 40 chars. Min width is 20 chars. Max width is is 80 chars. Chat column does not overlap the user "tiles". There is a resize widget between the tiled user tiles and the chat column that can be dragged right and left to resize them. Window size should be remembered between sessions via cookie or browser localstorage.

On mobile, the text chat should be below the tiled users.

Messages should show the friendly name of the client that sent the message.

Displayed text should be HTML/JS sanitized to avoid injection attacks or breaking the HTML display.

When connecting to a chat, prepopulate the chat history with the last 50 messages. Scroll to the bottom by default. (Yes, this means the server must persist message history)

If user scrolls up in message history, display a "return to current" widget to scroll them back to newer messages.

The field for user to type a new message, and the send button, are at the bottom of the chat column. Shift+enter or shift+return are equivalent to clicking the send button.

Limit the max message size to 500 chars. Web UI shouldn't let the user type more than this and the server should truncate any message longer than 500 chars before processing.

Show the friendlyname of the user and timestamp above each message. Use relative time ("4h ago", "34 min ago", "9 seconds ago") if the message is less than 24h old.

Consecutive messages by a single user less than an hour apart should be grouped, iMessage / Apple Message style - so the user/timestamp can be shown just once, at the top of the group.

## Architecture Decisions/Notes

How to implement this on the server is up to you. Give me some options and compare/contrast them.

If an off-the-shelf package/component fits our needs, great! Not opposed to rolling our own though, since you'll be doing the work heh heh heh.

Consider seeing if an off-the-shelf react component does what we need.

## Future Functionality

We'll implement these features in future iterations. Make architecture choices compatible with these planned features

- Reactions per post, similar to Discord or Slack but we'll limit them to standard unicode emoji
- ability for users to type slash commands (like `/title` to change the chat title)
- Basic rate limiting
- image posting
- admin users who can delete messages
