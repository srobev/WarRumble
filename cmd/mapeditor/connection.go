package main

import (
	"encoding/json"
	"net/http"
	neturl "net/url"
	"time"

	"github.com/gorilla/websocket"
)

// dialWS establishes a WebSocket connection with authentication
func dialWS(url, token string) (*websocket.Conn, error) {
	d := websocket.Dialer{HandshakeTimeout: 5 * time.Second,
		Proxy: func(*http.Request) (*neturl.URL, error) { return nil, nil }}
	// add token as header and query param (server accepts either)
	if token != "" {
		if u, err := neturl.Parse(url); err == nil {
			q := u.Query()
			q.Set("token", token)
			u.RawQuery = q.Encode()
			url = u.String()
		}
	}
	hdr := http.Header{}
	if token != "" {
		hdr.Set("Authorization", "Bearer "+token)
	}
	c, _, err := d.Dial(url, hdr)
	return c, err
}

// runReader handles incoming WebSocket messages
func (e *editor) runReader() {
	for {
		_, data, err := e.ws.ReadMessage()
		if err != nil {
			close(e.inCh)
			return
		}
		var m wsMsg
		if json.Unmarshal(data, &m) == nil {
			e.inCh <- m
		}
	}
}
