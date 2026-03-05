package sse

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID        string      `json:"id"`
	Event     string      `json:"event"`
	Data      interface{} `json:"data"`
	CreatedAt time.Time   `json:"created_at"`
}

type Broker struct {
	mu      sync.RWMutex
	clients map[string]chan Message
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[string]chan Message),
	}
}

func (b *Broker) Subscribe() (clientID string, ch <-chan Message, cancel func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := uuid.NewString()
	clientChan := make(chan Message, 100)
	b.clients[id] = clientChan

	return id, clientChan, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if ch, ok := b.clients[id]; ok {
			close(ch)
			delete(b.clients, id)
		}
	}
}

func (b *Broker) Publish(event string, data interface{}) {
	msg := Message{
		ID:        uuid.NewString(),
		Event:     event,
		Data:      data,
		CreatedAt: time.Now().UTC(),
	}

	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func Format(msg Message) ([]byte, error) {
	payload, err := json.Marshal(msg.Data)
	if err != nil {
		return nil, err
	}

	frame := "id: " + msg.ID + "\n" +
		"event: " + msg.Event + "\n" +
		"data: " + string(payload) + "\n\n"

	return []byte(frame), nil
}

type StreamHandler struct {
	broker *Broker
}

func NewStreamHandler(broker *Broker) *StreamHandler {
	return &StreamHandler{broker: broker}
}

func (h *StreamHandler) StreamOffers(ctx context.Context, write func([]byte) error, flush func() error) error {
	_, ch, cancel := h.broker.Subscribe()
	defer cancel()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-heartbeat.C:
			if err := write([]byte(": ping\n\n")); err != nil {
				return err
			}
			if err := flush(); err != nil {
				return err
			}
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			frame, err := Format(msg)
			if err != nil {
				continue
			}
			if err := write(frame); err != nil {
				return err
			}
			if err := flush(); err != nil {
				return err
			}
		}
	}
}

