package server

import (
	"encoding/json"
	"net/http"
)

// NewHandler serves liveness, room state, media status/signaling, and the
// locally built browser assets from one origin. Media handlers are optional so
// the shell's focused HTTP tests do not need to start a media process.
func NewHandler(staticDir string, roomHandler http.Handler, mediaHandlers ...http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	if roomHandler != nil {
		mux.Handle("GET /ws", roomHandler)
	}
	if len(mediaHandlers) > 0 && mediaHandlers[0] != nil {
		mux.Handle("/media/", mediaHandlers[0])
	}
	if len(mediaHandlers) > 1 && mediaHandlers[1] != nil {
		mux.Handle("GET /media/status", mediaHandlers[1])
	}
	mux.Handle("/", http.FileServer(http.Dir(staticDir)))
	return mux
}
