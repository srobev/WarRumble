package main

import (
	"log"
	"net/http"
	"time"

	"rumble/server/srv"
	"rumble/shared/protocol"

	"github.com/gorilla/websocket"
)

type Player struct {
	ID      int64
	Name    string
	Profile protocol.Profile
	// ...
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  2048,
	WriteBufferSize: 2048,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func wsHandler(h *srv.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("upgrade:", err)
			return
		}
		h.HandleWS(conn)
	}
}

func main() {
	hub := srv.NewHub()
	go hub.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler(hub))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })

	srvAddr := ":8080"
	s := &http.Server{
		Addr:         srvAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Println("server listening on", srvAddr)
	log.Fatal(s.ListenAndServe())
}
