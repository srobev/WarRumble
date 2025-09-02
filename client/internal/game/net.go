package game

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Net struct {
	mu     sync.Mutex
	conn   *websocket.Conn
	inCh   chan Msg
	closed bool
}

type Msg struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// HasToken reports whether a non-empty token file exists.
// Use the unified loader from auth.go (which should read via ConfigPath).
func HasToken() bool {
	return strings.TrimSpace(LoadToken()) != ""
}

func NewNet(wsURL string) (*Net, error) {
	tok := strings.TrimSpace(LoadToken()) // <-- use the same token as HTTP login saved

	// Prepare headers and also add token as a query param (belt & suspenders)
	hdr := http.Header{}
	if tok != "" {
		hdr.Set("Authorization", "Bearer "+tok)
		if u, err := neturl.Parse(wsURL); err == nil {
			q := u.Query()
			q.Set("token", tok)
			u.RawQuery = q.Encode()
			wsURL = u.String()
		}
	}

	log.Printf("WS dial: %s (token=%d chars)", wsURL, len(tok))

	dialer := websocket.Dialer{
		HandshakeTimeout:  5 * time.Second,
		EnableCompression: true,
		Proxy: func(*http.Request) (*neturl.URL, error) {
			return nil, nil // disable proxies
		},
	}

	c, resp, err := dialer.Dial(wsURL, hdr)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			log.Printf("WS dial failed: %s\n%s", resp.Status, string(body))
		} else {
			log.Printf("WS dial failed: %v", err)
		}
		return nil, err
	}

	n := &Net{conn: c, inCh: make(chan Msg, 128)}
	go n.reader()
	return n, nil
}

func (n *Net) reader() {
	for {
		_, data, err := n.conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			n.mu.Lock()
			n.closed = true
			n.conn = nil
			n.mu.Unlock()
			close(n.inCh)
			return
		}
		var m Msg
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		n.inCh <- m
	}
}

func (n *Net) Send(t string, v interface{}) error {
	n.mu.Lock()
	if n.closed || n.conn == nil {
		n.mu.Unlock()
		return errors.New("net: write on closed")
	}
	c := n.conn
	n.mu.Unlock()

	b, _ := json.Marshal(struct {
		Type string      `json:"type"`
		Data interface{} `json:"data"`
	}{Type: t, Data: v})

	if err := c.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println("write:", err)
		n.mu.Lock()
		n.closed = true
		n.conn = nil
		n.mu.Unlock()
		return err
	}
	return nil
}

// IsClosed reports whether Close() was called or the connection was torn down.
func (n *Net) IsClosed() bool {
	if n == nil {
		return true
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.closed
}

// Close closes the websocket and marks the Net as closed.
func (n *Net) Close() error {
	if n == nil {
		return nil
	}
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return nil
	}
	n.closed = true
	c := n.conn
	n.conn = nil
	n.mu.Unlock()

	var err error
	if c != nil {
		err = c.Close()
	}
	return err
}
