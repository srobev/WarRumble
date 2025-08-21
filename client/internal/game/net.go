package game

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"github.com/gorilla/websocket"
)

type Net struct {
	mu     sync.Mutex
	conn   *websocket.Conn
	inCh   chan Msg  // keep whatever channels/fields you already use
	closed bool
}

type Msg struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func NewNet(url string) (*Net, error) {
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
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
// Return an error only for the conn close; callers typically ignore it.
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
	// Optionally close your channels if your code expects that:
	// close(n.inCh) // only if all senders are done!
	return err
}
