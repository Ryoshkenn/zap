package telemetry

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/Ryoshkenn/zap/internal/state"
)

// apiKey is your PostHog project API key. Safe to embed — it's client-side only.
// Get it from posthog.com → Project Settings → Project API Key.
const apiKey = "phc_s78CWSGJWigdMEykv6RVYJkTrnkFE2Nf9c4Fs2BLcokp"

const batchEndpoint = "https://us.i.posthog.com/batch/"

var (
	installID    string
	zapVersion   string
	disabled     bool
	pending      []phEvent
	mu           sync.Mutex
	shutdownOnce sync.Once
)

type phEvent struct {
	Event      string         `json:"event"`
	DistinctID string         `json:"distinct_id"`
	Properties map[string]any `json:"properties"`
	Timestamp  string         `json:"timestamp"`
}

type phBatch struct {
	APIKey string    `json:"api_key"`
	Batch  []phEvent `json:"batch"`
}

// Init loads or generates a stable install ID and records the version.
// Must be called once at startup before any Track calls.
func Init(version string) {
	if os.Getenv("ZAP_NO_TELEMETRY") != "" || os.Getenv("DO_NOT_TRACK") != "" {
		disabled = true
		return
	}
	zapVersion = version

	s, err := state.Load()
	if err != nil || s == nil {
		disabled = true
		return
	}
	if s.InstallID == "" {
		s.InstallID = newUUID()
		_ = s.Save()
	}
	installID = s.InstallID
}

// Track enqueues an analytics event. Properties are merged with standard fields.
// No-op if telemetry is disabled or Init was not called.
func Track(event string, props map[string]any) {
	if disabled || installID == "" {
		return
	}
	if props == nil {
		props = map[string]any{}
	}
	props["version"] = zapVersion
	props["$os"] = runtime.GOOS
	props["arch"] = runtime.GOARCH

	mu.Lock()
	pending = append(pending, phEvent{
		Event:      event,
		DistinctID: installID,
		Properties: props,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	})
	mu.Unlock()
}

// Shutdown flushes all pending events with a 500ms deadline.
// Must be called before any syscall.Exec or os.Exit so events are not lost.
// Safe to call multiple times — only the first call sends.
func Shutdown() {
	shutdownOnce.Do(func() {
		if disabled || installID == "" {
			return
		}
		mu.Lock()
		events := pending
		pending = nil
		mu.Unlock()

		if len(events) == 0 {
			return
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			flush(events)
		}()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
	})
}

func flush(events []phEvent) {
	b := phBatch{APIKey: apiKey, Batch: events}
	data, err := json.Marshal(b)
	if err != nil {
		return
	}
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest(http.MethodPost, batchEndpoint, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
