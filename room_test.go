package gx_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alternayte/gx"
)

// noteSignals is the Signals struct that the compiler writes for a component
// with two shared signals and one signal of the instance.
type noteSignals struct {
	Typing bool   `json:"typing"`
	Title  string `json:"title"`
	Draft  string `json:"draft"`
}

func (s *noteSignals) Rules() gx.Rules {
	return gx.Rules{gx.Field(&s.Title, gx.MaxLen(5))}
}

func (s *noteSignals) GxFieldName(ptr any) string {
	switch ptr {
	case &s.Typing:
		return "typing"
	case &s.Title:
		return "title"
	}
	return ""
}

// noteRoom returns the token that a page carries for a room of the
// component notes.Note.
func noteRoom(key string) gx.Room {
	return gx.Room(gx.RoomToken("notes.Note", gx.RoomKey(nil, key)))
}

// roomApp mounts the routes of the shared signals behind a middleware that
// refuses a request with no pass.
func roomApp(t *testing.T, bus gx.Bus) *httptest.Server {
	t.Helper()
	pass := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Pass") == "" && r.URL.Query().Get("pass") == "" {
				http.Error(w, "no pass", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	app := gx.New(gx.Config{Bus: bus, Secret: []byte("test-secret")})
	app.Group("/", pass, gx.SharedSignals[noteSignals]("notes.Note", "typing", "title"))
	srv := httptest.NewServer(app)
	t.Cleanup(srv.Close)
	return srv
}

// roomViewer reads the stream of a room. Each value of the channel is the
// state that one frame holds.
func roomViewer(t *testing.T, srv *httptest.Server, room gx.Room) (<-chan map[string]any, int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/_gx/room/notes.Note?pass=1&room="+string(room), nil)
	req.Header.Set("Accept", "text/event-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	out := make(chan map[string]any, 16)
	if res.StatusCode != http.StatusOK {
		return out, res.StatusCode
	}
	// The first line of the stream says that the viewer is in the room.
	reader := bufio.NewReader(res.Body)
	if line, _ := reader.ReadString('\n'); !strings.HasPrefix(line, ": gx room") {
		t.Fatalf("the stream starts with %q", line)
	}
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if data, ok := strings.CutPrefix(line, "data: "); ok {
				var state map[string]any
				if json.Unmarshal([]byte(data), &state) == nil {
					out <- state
				}
			}
		}
	}()
	return out, res.StatusCode
}

func roomWrite(t *testing.T, srv *httptest.Server, room gx.Room, signals string, pass bool) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", srv.URL+"/_gx/room/notes.Note", strings.NewReader(`{"room":"`+string(room)+`","signals":`+signals+`}`))
	req.Header.Set("Content-Type", "application/json")
	if pass {
		req.Header.Set("X-Pass", "1")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func next(t *testing.T, ch <-chan map[string]any) map[string]any {
	t.Helper()
	select {
	case state := <-ch:
		return state
	case <-time.After(3 * time.Second):
		t.Fatal("no value came from the room")
		return nil
	}
}

func quiet(t *testing.T, ch <-chan map[string]any) {
	t.Helper()
	select {
	case state := <-ch:
		t.Fatalf("a viewer got %v, want no value", state)
	case <-time.After(150 * time.Millisecond):
	}
}

// TestREQ_ACT_21_RoomSharesAValue checks that a write of one viewer reaches
// each viewer of the room, the writer too, that a viewer of a different room
// gets nothing, and that a new viewer gets the last value (REQ-ACT-21).
func TestREQ_ACT_21_RoomSharesAValue(t *testing.T) {
	srv := roomApp(t, nil)
	doc1, doc2 := noteRoom("doc-1"), noteRoom("doc-2")
	writer, _ := roomViewer(t, srv, doc1)
	reader, _ := roomViewer(t, srv, doc1)
	other, _ := roomViewer(t, srv, doc2)

	if res := roomWrite(t, srv, doc1, `{"typing":true}`, true); res.StatusCode != http.StatusNoContent {
		t.Fatalf("write: status %d", res.StatusCode)
	}
	for name, ch := range map[string]<-chan map[string]any{"the writer": writer, "the second viewer": reader} {
		if state := next(t, ch); state["typing"] != true {
			t.Errorf("%s got %v, want typing true", name, state)
		}
	}
	quiet(t, other)

	// A second signal: the state holds the two last values.
	roomWrite(t, srv, doc1, `{"title":"Plan"}`, true)
	if state := next(t, reader); state["typing"] != true || state["title"] != "Plan" {
		t.Errorf("after a second write the viewer got %v", state)
	}
	// A viewer that comes later gets the last values with no write.
	late, _ := roomViewer(t, srv, doc1)
	if state := next(t, late); state["typing"] != true || state["title"] != "Plan" {
		t.Errorf("a late viewer got %v, want the last values", state)
	}
}

// TestSI_17_RoomKeyIsSigned checks the rules of a room: a key that the
// server did not sign gets 403 for a subscribe and for a write; the two
// requests pass the middleware of the group; a value passes the rules of
// the Signals struct; and only a shared signal has a value in a room
// (SI-17).
func TestSI_17_RoomKeyIsSigned(t *testing.T) {
	srv := roomApp(t, nil)
	room := noteRoom("doc-1")
	viewer, status := roomViewer(t, srv, room)
	if status != http.StatusOK {
		t.Fatalf("a signed key: status %d", status)
	}

	// A different key with the signature of this one, and a key with no
	// signature.
	enc, sig, _ := strings.Cut(string(room), ".")
	other, _, _ := strings.Cut(string(noteRoom("doc-2")), ".")
	for name, forged := range map[string]gx.Room{"an edited key": gx.Room(other + "." + sig), "no signature": gx.Room(enc), "an empty key": ""} {
		if _, status := roomViewer(t, srv, forged); status != http.StatusForbidden {
			t.Errorf("subscribe with %s: status %d, want 403", name, status)
		}
		if res := roomWrite(t, srv, forged, `{"typing":true}`, true); res.StatusCode != http.StatusForbidden {
			t.Errorf("write with %s: status %d, want 403", name, res.StatusCode)
		}
	}
	quiet(t, viewer)

	// The middleware of the group refuses a request with no pass.
	if res := roomWrite(t, srv, room, `{"typing":true}`, false); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("a write that the middleware refuses: status %d, want 401", res.StatusCode)
	}
	req, _ := http.NewRequest("GET", srv.URL+"/_gx/room/notes.Note?room="+string(room), nil)
	req.Header.Set("Accept", "text/event-stream")
	if res, err := http.DefaultClient.Do(req); err != nil || res.StatusCode != http.StatusUnauthorized {
		t.Errorf("a subscribe that the middleware refuses: %v, %v", res, err)
	}
	quiet(t, viewer)

	// A value that fails a rule is a field error and reaches no viewer.
	res := roomWrite(t, srv, room, `{"title":"too long a title"}`, true)
	var body struct {
		Errors []map[string]string `json:"errors"`
	}
	_ = json.NewDecoder(res.Body).Decode(&body)
	if res.StatusCode != http.StatusUnprocessableEntity || len(body.Errors) != 1 || body.Errors[0]["field"] != "title" {
		t.Errorf("a value that fails a rule: status %d, errors %v, want 422 for the field title", res.StatusCode, body.Errors)
	}
	// A value of the wrong type, and a signal that is not shared.
	for name, signals := range map[string]string{"a wrong type": `{"typing":"yes"}`, "a signal of the instance": `{"draft":"x"}`, "an unknown signal": `{"admin":true}`} {
		if res := roomWrite(t, srv, room, signals, true); res.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, res.StatusCode)
		}
	}
	quiet(t, viewer)

	// A different secret signs a different key: a page of an app with a
	// different secret has no valid key here.
	gx.SetSecret([]byte("a different secret"))
	foreign := noteRoom("doc-1")
	gx.SetSecret([]byte("test-secret"))
	if res := roomWrite(t, srv, foreign, `{"typing":true}`, true); res.StatusCode != http.StatusForbidden {
		t.Errorf("a key of a different secret: status %d, want 403", res.StatusCode)
	}
}

// TestSI_17_RoomTokenIsForOneComponent checks that the token of a room is
// valid only at the routes of its component and only before its expiry, and
// that the value of the loader is not a token (SI-17).
func TestSI_17_RoomTokenIsForOneComponent(t *testing.T) {
	srv := roomApp(t, nil)
	key := gx.RoomKey(nil, "doc-1")
	for name, bad := range map[string]gx.Room{
		"the token of a different component": gx.Room(gx.RoomToken("notes.Other", key)),
		"the value of the loader":            key,
		"a token after its expiry":           gx.RoomTokenAt("notes.Note", "doc-1", time.Now().Add(-time.Second)),
	} {
		if bad == "" {
			t.Fatalf("%s: no token", name)
		}
		if _, status := roomViewer(t, srv, bad); status != http.StatusForbidden {
			t.Errorf("subscribe with %s: status %d, want 403", name, status)
		}
		if res := roomWrite(t, srv, bad, `{"typing":true}`, true); res.StatusCode != http.StatusForbidden {
			t.Errorf("write with %s: status %d, want 403", name, res.StatusCode)
		}
	}
	if token := gx.RoomToken("notes.Note", "bm8.c2ln"); token != "" {
		t.Errorf("a room that RoomKey did not make gives the token %q", token)
	}
}

// TestSI_17_RoomStreamRenewsTheToken checks that the stream gives a viewer a
// new token before the expiry of its token, and ends at that expiry
// (SI-17).
func TestSI_17_RoomStreamRenewsTheToken(t *testing.T) {
	srv := roomApp(t, nil)
	// The token is at less than half of its life.
	old := gx.RoomTokenAt("notes.Note", "doc-1", time.Now().Add(2*time.Second))
	req, _ := http.NewRequest("GET", srv.URL+"/_gx/room/notes.Note?pass=1&room="+string(old), nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("subscribe: %v, %v", res, err)
	}
	defer res.Body.Close()
	done := make(chan string, 1)
	go func() {
		var renewed string
		reader := bufio.NewReader(res.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				// The stream ended.
				done <- renewed
				return
			}
			if strings.HasPrefix(line, "event: gx-room-key") {
				data, _ := reader.ReadString('\n')
				renewed = strings.TrimSpace(strings.TrimPrefix(data, "data: "))
			}
		}
	}()
	select {
	case renewed := <-done:
		if renewed == "" || renewed == string(old) {
			t.Fatalf("the stream gave the token %q, want a new token", renewed)
		}
		if res := roomWrite(t, srv, gx.Room(renewed), `{"typing":true}`, true); res.StatusCode != http.StatusNoContent {
			t.Errorf("a write with the new token: status %d, want 204", res.StatusCode)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the stream did not end at the expiry of its token")
	}
}

// crossSignals has a rule that reads two signals and names no field.
type crossSignals struct {
	Low  int `json:"low"`
	High int `json:"high"`
}

func (s *crossSignals) Rules() gx.Rules {
	return gx.Rules{gx.Check(func(any) error {
		if s.Low > s.High {
			return errors.New("low is above high")
		}
		return nil
	})}
}

// TestSI_17_RuleWithNoFieldReadsTheRoom checks that a rule with no field
// reads the last values of the room for the signals that a write does not
// hold (SI-17).
func TestSI_17_RuleWithNoFieldReadsTheRoom(t *testing.T) {
	app := gx.New(gx.Config{Secret: []byte("test-secret")})
	app.Group("/", gx.SharedSignals[crossSignals]("notes.Note", "low", "high"))
	srv := httptest.NewServer(app)
	t.Cleanup(srv.Close)
	room := noteRoom("range-1")
	viewer, _ := roomViewer(t, srv, room)
	if res := roomWrite(t, srv, room, `{"high":10}`, true); res.StatusCode != http.StatusNoContent {
		t.Fatalf("write of high: status %d", res.StatusCode)
	}
	next(t, viewer)
	// The write holds only low: the rule reads the high of the room.
	if res := roomWrite(t, srv, room, `{"low":5}`, true); res.StatusCode != http.StatusNoContent {
		t.Errorf("a low below the high of the room: status %d, want 204", res.StatusCode)
	}
	next(t, viewer)
	if res := roomWrite(t, srv, room, `{"low":20}`, true); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("a low above the high of the room: status %d, want 422", res.StatusCode)
	}
	quiet(t, viewer)
}

// primeBus primes each new room with one message when a viewer subscribes.
// It makes the race between the join and the first read of the stream loop
// deterministic (REQ-ACT-21).
type primeBus struct{ message []byte }

func (b *primeBus) Publish(context.Context, string, []byte) error { return nil }

func (b *primeBus) Subscribe(_ context.Context, _ string, receive func([]byte)) (func(), error) {
	receive(b.message)
	return func() {}, nil
}

// TestREQ_ACT_21_RoomStateReachesAViewerOneTime checks that a state that
// arrives before the stream loop reads the room reaches the viewer one
// time. A write between the join and the first read used to send the same
// state two times: once from the join and once more from the signal of the
// write.
func TestREQ_ACT_21_RoomStateReachesAViewerOneTime(t *testing.T) {
	bus := &primeBus{message: []byte(`{"high":10}`)}
	app := gx.New(gx.Config{Bus: bus, Secret: []byte("test-secret")})
	app.Group("/", gx.SharedSignals[crossSignals]("notes.Note", "low", "high"))
	srv := httptest.NewServer(app)
	t.Cleanup(srv.Close)
	viewer, _ := roomViewer(t, srv, noteRoom("once-1"))
	if state := next(t, viewer); state["high"] != float64(10) {
		t.Fatalf("the first frame = %v, want high 10", state)
	}
	quiet(t, viewer)
}

// relayBus is a bus of an app: it gives each message to each subscriber of
// each replica. It counts its messages, so a test can see that it carried
// them.
type relayBus struct {
	mu        sync.Mutex
	next      int
	receivers map[string]map[int]func([]byte)
	published int
}

func (b *relayBus) Publish(_ context.Context, topic string, message []byte) error {
	b.mu.Lock()
	b.published++
	var receivers []func([]byte)
	for _, receive := range b.receivers[topic] {
		receivers = append(receivers, receive)
	}
	b.mu.Unlock()
	for _, receive := range receivers {
		go receive(append([]byte(nil), message...))
	}
	return nil
}

func (b *relayBus) Subscribe(_ context.Context, topic string, receive func([]byte)) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.receivers == nil {
		b.receivers = map[string]map[int]func([]byte){}
	}
	if b.receivers[topic] == nil {
		b.receivers[topic] = map[int]func([]byte){}
	}
	b.next++
	id := b.next
	b.receivers[topic][id] = receive
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.receivers[topic], id)
	}, nil
}

// busContract is what each bus does: a subscriber of a topic gets each
// message of the topic, a subscriber of a different topic gets none, and a
// subscriber that cancelled gets none.
func busContract(t *testing.T, bus gx.Bus) {
	t.Helper()
	got := make(chan string, 8)
	cancelA, err := bus.Subscribe(context.Background(), "a", func(m []byte) { got <- "a1:" + string(m) })
	if err != nil {
		t.Fatal(err)
	}
	cancelA2, _ := bus.Subscribe(context.Background(), "a", func(m []byte) { got <- "a2:" + string(m) })
	cancelB, _ := bus.Subscribe(context.Background(), "b", func(m []byte) { got <- "b:" + string(m) })
	defer cancelA2()
	defer cancelB()
	if err := bus.Publish(context.Background(), "a", []byte("one")); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case m := <-got:
			seen[m] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("got %v, want the message for the two subscribers of the topic", seen)
		}
	}
	if !seen["a1:one"] || !seen["a2:one"] {
		t.Errorf("got %v, want a1:one and a2:one", seen)
	}
	cancelA()
	_ = bus.Publish(context.Background(), "a", []byte("two"))
	select {
	case m := <-got:
		if m != "a2:two" {
			t.Errorf("after a cancel, got %q, want only a2:two", m)
		}
	case <-time.After(2 * time.Second):
		t.Error("the subscriber that stays got no message")
	}
	select {
	case m := <-got:
		t.Errorf("an extra message %q", m)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestREQ_ACT_22_BusContract runs the contract of a bus against the
// in-memory bus of Gx and against a bus of an app (REQ-ACT-22).
func TestREQ_ACT_22_BusContract(t *testing.T) {
	t.Run("the in-memory bus", func(t *testing.T) { busContract(t, gx.NewMemoryBus()) })
	t.Run("a bus of an app", func(t *testing.T) { busContract(t, &relayBus{}) })
}

// TestREQ_ACT_22_TwoAppsShareABus checks the bus of gx.Config with two apps
// in one test, as two replicas: with one bus of the app, a write to one app
// reaches a viewer of the other app. With the in-memory bus of Gx, each app
// has its own rooms (REQ-ACT-22).
func TestREQ_ACT_22_TwoAppsShareABus(t *testing.T) {
	bus := &relayBus{}
	first, second := roomApp(t, bus), roomApp(t, bus)
	room := noteRoom("doc-9")
	viewer, _ := roomViewer(t, first, room)
	roomWrite(t, second, room, `{"title":"Hi"}`, true)
	if state := next(t, viewer); state["title"] != "Hi" {
		t.Errorf("the viewer of the first app got %v, want the title that the second app got", state)
	}
	bus.mu.Lock()
	published := bus.published
	bus.mu.Unlock()
	if published != 1 {
		t.Errorf("the bus of the app carried %d messages, want 1", published)
	}

	// With no bus in gx.Config, two apps do not share a room.
	alone, other := roomApp(t, nil), roomApp(t, nil)
	lonely, _ := roomViewer(t, alone, room)
	roomWrite(t, other, room, `{"title":"Hi"}`, true)
	quiet(t, lonely)
}
