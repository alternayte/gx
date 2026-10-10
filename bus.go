package gx

import (
	"context"
	"sync"
)

// Bus carries the values of shared signals between the replicas of an app
// (REQ-ACT-22). Publish gives a message to each subscriber of the topic, on
// each replica. Subscribe calls receive for each message of the topic until
// the caller calls cancel.
//
// Gx has one implementation: the in-memory bus, for an app with one replica.
// An app with more than one replica gives its own in gx.Config, for example
// on LISTEN and NOTIFY of its database. A message is small and is not
// durable: a subscriber that is not connected does not get it.
type Bus interface {
	Publish(ctx context.Context, topic string, message []byte) error
	Subscribe(ctx context.Context, topic string, receive func(message []byte)) (cancel func(), err error)
}

// NewMemoryBus returns a bus that lives in this process. Two replicas of an
// app do not see the messages of each other through it.
func NewMemoryBus() Bus { return &memoryBus{topics: map[string]map[uint64]func([]byte){}} }

type memoryBus struct {
	mu     sync.Mutex
	next   uint64
	topics map[string]map[uint64]func([]byte)
}

func (b *memoryBus) Publish(_ context.Context, topic string, message []byte) error {
	b.mu.Lock()
	receivers := make([]func([]byte), 0, len(b.topics[topic]))
	for _, receive := range b.topics[topic] {
		receivers = append(receivers, receive)
	}
	b.mu.Unlock()
	for _, receive := range receivers {
		receive(message)
	}
	return nil
}

func (b *memoryBus) Subscribe(_ context.Context, topic string, receive func([]byte)) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	id := b.next
	if b.topics[topic] == nil {
		b.topics[topic] = map[uint64]func([]byte){}
	}
	b.topics[topic][id] = receive
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.topics[topic], id)
		if len(b.topics[topic]) == 0 {
			delete(b.topics, topic)
		}
	}, nil
}

// defaultRooms holds the rooms of the routes of shared signals that no
// gx.App mounted: a route on a different router (REQ-RTE-17). They use an
// in-memory bus.
var defaultRooms = sync.OnceValue(func() *roomHub { return newRoomHub(NewMemoryBus()) })
