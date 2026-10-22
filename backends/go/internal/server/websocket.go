package server

import (
	"net/http"

	"bigbrother/internal/room"
)

func RoomWebSocket(value *room.Room) http.Handler {
	return value.Handler()
}
