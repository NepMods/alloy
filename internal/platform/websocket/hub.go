package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	gorilla "github.com/gorilla/websocket"
)

var upgrader = gorilla.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
	Room    string          `json:"room,omitempty"`
}

type Client struct {
	ID     int64
	UserID int64
	hub    *Hub
	conn   *gorilla.Conn
	send   chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]map[*Client]bool
	clients map[int64]*Client
}

func NewHub() *Hub {
	return &Hub{
		rooms:   make(map[string]map[*Client]bool),
		clients: make(map[int64]*Client),
	}
}

func (h *Hub) Connect(w http.ResponseWriter, r *http.Request, userID int64) (*Client, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	c := &Client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		UserID: userID,
	}
	h.mu.Lock()
	h.clients[userID] = c
	h.mu.Unlock()
	go c.writePump()
	go c.readPump()
	return c, nil
}

func (h *Hub) JoinRoom(room string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*Client]bool)
	}
	h.rooms[room][c] = true
}

func (h *Hub) LeaveRoom(room string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[room] != nil {
		delete(h.rooms[room], c)
		if len(h.rooms[room]) == 0 {
			delete(h.rooms, room)
		}
	}
}

func (h *Hub) Broadcast(room string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[room] {
		select {
		case c.send <- msg:
		default:
			close(c.send)
			delete(h.rooms[room], c)
		}
	}
}

func (h *Hub) SendToUser(userID int64, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if c, ok := h.clients[userID]; ok {
		select {
		case c.send <- msg:
		default:
		}
	}
}

func (h *Hub) Disconnect(c *Client) {
	h.mu.Lock()
	delete(h.clients, c.UserID)
	for _, clients := range h.rooms {
		delete(clients, c)
	}
	h.mu.Unlock()
	c.conn.Close()
}

func (c *Client) readPump() {
	defer c.hub.Disconnect(c)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		var m Message
		if json.Unmarshal(msg, &m) != nil {
			continue
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(gorilla.CloseMessage, []byte{})
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(gorilla.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(gorilla.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) SendJSON(ctx context.Context, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	select {
	case c.send <- data:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
