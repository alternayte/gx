package gx

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Room is the key of a room with the signature of the server (SI-17). A
// room is the set of viewers that share the shared signals of one component
// instance (REQ-ACT-21). A loader makes the value with RoomKey and gives it
// to the component as a prop; the page carries it, and a viewer cannot make
// one for a different key.
type Room string

// roomSecret signs the room keys. gx.New sets it from Config.Secret. With no
// secret, a process makes its own at the first use: a page of one replica
// then has no valid key for a second replica.
var (
	roomSecret     atomic.Pointer[[]byte]
	roomSecretOnce sync.Once
	// roomSecretRandom is true when the process made its own secret.
	roomSecretRandom atomic.Bool
)

// SetSecret sets the secret that signs the room keys. gx.New calls it for
// Config.Secret.
func SetSecret(secret []byte) {
	if len(secret) > 0 {
		s := append([]byte(nil), secret...)
		roomSecret.Store(&s)
		roomSecretRandom.Store(false)
	}
}

func secret() []byte {
	if s := roomSecret.Load(); s != nil {
		return *s
	}
	roomSecretOnce.Do(func() {
		if roomSecret.Load() != nil {
			return
		}
		s := make([]byte, 32)
		if _, err := rand.Read(s); err != nil {
			panic("gx: no random bytes for the secret of the room keys: " + err.Error())
		}
		roomSecret.Store(&s)
		roomSecretRandom.Store(true)
	})
	return *roomSecret.Load()
}

func signRoom(key string) string {
	mac := hmac.New(sha256.New, secret())
	mac.Write([]byte(key))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// RoomKey returns the room with the given key, signed by the server
// (SI-17). A loader calls it for a viewer that can be in the room: the
// middleware of the group of the page ran before the loader. The key is what
// the viewers share, for example the id of a document.
func RoomKey(c *Ctx, key string) Room {
	return Room(base64.RawURLEncoding.EncodeToString([]byte(key)) + "." + signRoom(key))
}

// roomKeyOf returns the key of a signed room, or false for a value that the
// server did not sign.
func roomKeyOf(room string) (string, bool) {
	enc, sig, ok := strings.Cut(room, ".")
	if !ok {
		return "", false
	}
	key, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil || !hmac.Equal([]byte(sig), []byte(signRoom(string(key)))) {
		return "", false
	}
	return string(key), true
}

// roomPath is the path of the routes of the shared signals of a component.
const roomPath = "/_gx/room/"

// RoomURL returns the address of the shared signals of a component, for the
// runtime. Generated code writes it into the root of the component.
func RoomURL(base string) string { return BasePath() + roomPath + base }

// ShareEffect returns the effect of the root of a component with shared
// signals: it gives the runtime the value of each shared signal, at the
// start and after each change. The runtime writes a changed value to the
// room. Generated code calls it (REQ-ACT-21).
func ShareEffect(base string, key Key, names ...string) string {
	var b strings.Builder
	b.WriteString("window.__gx&&window.__gx.share(el,{")
	for i, name := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Quote(name))
		b.WriteByte(':')
		b.WriteString(SignalPath(base, key, name))
	}
	b.WriteString("})")
	return b.String()
}

// sharedCount is the number of components with a shared signal in this
// program.
var sharedCount atomic.Int32

// SharedSignals returns the two routes of the shared signals of a component
// (REQ-ACT-21): the stream that a viewer of a room reads, and the write of a
// value. S is the generated Signals struct of the component, base is the
// signal namespace of the component, and names are its shared signals.
// Generated code makes the value; the app mounts it in a group, so each
// request passes the middleware of that group (SI-17).
func SharedSignals[S any](base string, names ...string) []Handler {
	sharedCount.Add(1)
	shared := map[string]bool{}
	for _, name := range names {
		shared[name] = true
	}
	h := &sharedSignals[S]{base: base, names: shared}
	return []Handler{
		&sharedRoute{pattern: "GET " + roomPath + base, serve: h.subscribe},
		&sharedRoute{pattern: "POST " + roomPath + base, serve: h.write},
	}
}

// sharedRoute is one route of the shared signals of a component. The app
// that mounts it gives it the rooms of that app: two apps, as two replicas,
// share a room only through their bus.
type sharedRoute struct {
	pattern string
	serve   func(*roomHub, http.ResponseWriter, *http.Request)
	rooms   *roomHub
}

func (s *sharedRoute) Pattern() string { return s.pattern }

// exportFeature names the route for the export check: a room needs a
// server, and its stream is not a page (REQ-EXP-02).
func (s *sharedRoute) exportFeature() (kind string, serverOnly bool) {
	return "shared signal", true
}

func (s *sharedRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rooms := s.rooms
	if rooms == nil {
		rooms = defaultRooms()
	}
	s.serve(rooms, w, r)
}

// forApp returns the route with the rooms of an app.
func (s *sharedRoute) forApp(a *App) *sharedRoute {
	return &sharedRoute{pattern: s.pattern, serve: s.serve, rooms: a.rooms}
}

type sharedSignals[S any] struct {
	base  string
	names map[string]bool
}

func (h *sharedSignals[S]) topic(key string) string { return "gx.room:" + h.base + ":" + key }

// subscribe streams the values of the room to one viewer. The first frame
// holds the last values that this replica knows.
func (h *sharedSignals[S]) subscribe(rooms *roomHub, w http.ResponseWriter, r *http.Request) {
	key, ok := roomKeyOf(r.URL.Query().Get("room"))
	if !ok {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "gx: the connection cannot stream", http.StatusInternalServerError)
		return
	}
	topic := h.topic(key)
	sub, err := rooms.join(r.Context(), topic)
	if err != nil {
		http.Error(w, "gx: the bus refused the room", http.StatusServiceUnavailable)
		return
	}
	defer rooms.leave(topic, sub)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, ": gx room\n\n")
	flusher.Flush()
	for {
		if state := rooms.state(topic); state != nil {
			if _, err := io.WriteString(w, "event: gx-room\ndata: "+string(state)+"\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-sub:
		}
	}
}

// write takes the new value of shared signals from one viewer, checks it,
// and gives it to the bus. Each viewer of the room gets it from the stream,
// the writer too.
func (h *sharedSignals[S]) write(rooms *roomHub, w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	var body struct {
		Room    string                     `json:"room"`
		Signals map[string]json.RawMessage `json:"signals"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		writeAPIErrors(w, http.StatusBadRequest, apiError{Key: "gx.bad_input"})
		return
	}
	key, ok := roomKeyOf(body.Room)
	if !ok {
		writeAPIErrors(w, http.StatusForbidden, apiError{Key: "gx.forbidden"})
		return
	}
	names := make([]string, 0, len(body.Signals))
	for name := range body.Signals {
		if !h.names[name] {
			// Only a shared signal has a value in a room.
			writeAPIErrors(w, http.StatusBadRequest, apiError{Key: "gx.bad_input", Field: name})
			return
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sort.Strings(names)
	// The value has the type of the signal, and passes the rules of the
	// Signals struct (DR-07).
	var signals S
	raw, _ := json.Marshal(body.Signals)
	if err := json.Unmarshal(raw, &signals); err != nil {
		writeAPIErrors(w, http.StatusBadRequest, apiError{Key: "gx.bad_input"})
		return
	}
	var errs []apiError
	for _, v := range RunAllRulesContext(r.Context(), &signals) {
		// A rule of a signal that this write does not hold is not a
		// failure of this write.
		if v.Field == "" || body.Signals[v.Field] != nil {
			errs = append(errs, apiError{Key: v.Key, Field: v.Field, Message: v.Message})
		}
	}
	if len(errs) > 0 {
		writeAPIErrors(w, http.StatusUnprocessableEntity, errs...)
		return
	}
	// The values go out in the JSON form of the struct, not as the
	// viewer wrote them.
	var typed map[string]json.RawMessage
	clean, _ := json.Marshal(signals)
	_ = json.Unmarshal(clean, &typed)
	out := map[string]json.RawMessage{}
	for _, name := range names {
		out[name] = typed[name]
	}
	message, _ := json.Marshal(out)
	if err := rooms.bus.Publish(r.Context(), h.topic(key), message); err != nil {
		writeAPIErrors(w, http.StatusServiceUnavailable, apiError{Key: "gx.error"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// roomHub holds the rooms that have a viewer on one replica, and the bus
// that carries their values to the other replicas.
type roomHub struct {
	bus    Bus
	mu     sync.Mutex
	topics map[string]*roomTopic
}

func newRoomHub(bus Bus) *roomHub {
	if bus == nil {
		bus = NewMemoryBus()
	}
	return &roomHub{bus: bus, topics: map[string]*roomTopic{}}
}

// roomTopic is one room on this replica: its viewers and the last value of
// each signal. The values go when the last viewer leaves: a shared signal is
// not durable (REQ-ACT-21).
type roomTopic struct {
	subs   map[chan struct{}]bool
	last   map[string]json.RawMessage
	cancel func()
}

// join adds a viewer. The returned channel gets a value when the room has
// new values; the viewer then reads the state.
func (h *roomHub) join(ctx context.Context, topic string) (chan struct{}, error) {
	sub := make(chan struct{}, 1)
	h.mu.Lock()
	t := h.topics[topic]
	if t != nil {
		t.subs[sub] = true
		h.mu.Unlock()
		return sub, nil
	}
	t = &roomTopic{subs: map[chan struct{}]bool{sub: true}, last: map[string]json.RawMessage{}}
	h.topics[topic] = t
	h.mu.Unlock()
	// The subscription outlives the request of the first viewer.
	cancel, err := h.bus.Subscribe(context.WithoutCancel(ctx), topic, func(message []byte) { h.deliver(topic, message) })
	h.mu.Lock()
	defer h.mu.Unlock()
	if err != nil {
		delete(h.topics, topic)
		return nil, err
	}
	t.cancel = cancel
	return sub, nil
}

func (h *roomHub) leave(topic string, sub chan struct{}) {
	h.mu.Lock()
	t := h.topics[topic]
	var cancel func()
	if t != nil {
		delete(t.subs, sub)
		if len(t.subs) == 0 {
			delete(h.topics, topic)
			cancel = t.cancel
		}
	}
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// deliver keeps the values of a message and tells each viewer.
func (h *roomHub) deliver(topic string, message []byte) {
	var values map[string]json.RawMessage
	if json.Unmarshal(message, &values) != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	t := h.topics[topic]
	if t == nil {
		return
	}
	for name, value := range values {
		t.last[name] = value
	}
	for sub := range t.subs {
		select {
		case sub <- struct{}{}:
		default:
			// The viewer has a value to read: it reads the state.
		}
	}
}

// state returns the JSON of the last values of a room, or nil for a room
// with no value yet.
func (h *roomHub) state(topic string) []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	t := h.topics[topic]
	if t == nil || len(t.last) == 0 {
		return nil
	}
	data, _ := json.Marshal(t.last)
	return data
}

// logBus writes the bus of the shared signals to the log of a dev build
// (REQ-ACT-22). An app with no shared signal has no line.
func logBus(bus Bus) {
	if !Dev || sharedCount.Load() == 0 {
		return
	}
	name := "the bus of the app"
	if bus == nil {
		name = "the in-memory bus of Gx: two replicas do not share a room"
	}
	log.Printf("gx: shared signals use %s", name)
}
