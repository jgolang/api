package api

import (
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

// chainSetups build the handler under test from
// MiddlewaresChain(ProcessRequest)(next), on the default instance and on an
// instance.
var chainSetups = []struct {
	name  string
	build func(t *testing.T, next http.HandlerFunc) http.Handler
}{
	{name: "default instance", build: func(t *testing.T, next http.HandlerFunc) http.Handler {
		captureDefaultLogs(t)
		return MiddlewaresChain(ProcessRequest)(next)
	}},
	{name: "instance", build: func(_ *testing.T, next http.HandlerFunc) http.Handler {
		return New().Handler(MiddlewaresChain(ProcessRequest)(next))
	}},
}

func TestMiddlewaresChainFlushReachesClientBeforeHandlerReturns(t *testing.T) {
	for _, setup := range chainSetups {
		t.Run(setup.name, func(t *testing.T) {
			release := make(chan struct{})
			server := serveStream(t, setup.build(t, func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, "first")
				w.(http.Flusher).Flush()
				<-release
				io.WriteString(w, "second")
			}))
			// A buffered response never arrives while the handler waits for
			// the first chunk to be read: the client times out, and release
			// lets the server shut down.
			var releaseOnce sync.Once
			releaseHandler := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(releaseHandler)
			client := &http.Client{Timeout: 5 * time.Second}

			res, err := client.Get(server.URL + "/stream")
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			defer res.Body.Close()
			first := make([]byte, len("first"))
			_, readErr := io.ReadFull(res.Body, first)
			releaseHandler()
			rest, restErr := io.ReadAll(res.Body)

			if readErr != nil || string(first) != "first" {
				t.Fatalf("first chunk = %q, %v; want %q before the handler returns", first, readErr, "first")
			}
			if restErr != nil || string(rest) != "second" {
				t.Fatalf("rest = %q, %v; want %q", rest, restErr, "second")
			}
		})
	}
}

func TestMiddlewaresChainResponseControllerReachesConnection(t *testing.T) {
	calls := []struct {
		name string
		call func(*http.ResponseController) error
	}{
		{name: "SetReadDeadline", call: func(c *http.ResponseController) error {
			return c.SetReadDeadline(time.Now().Add(time.Minute))
		}},
		{name: "SetWriteDeadline", call: func(c *http.ResponseController) error {
			return c.SetWriteDeadline(time.Now().Add(time.Minute))
		}},
		{name: "Flush", call: func(c *http.ResponseController) error { return c.Flush() }},
	}
	for _, setup := range chainSetups {
		for _, tt := range calls {
			t.Run(setup.name+"/"+tt.name, func(t *testing.T) {
				got := make(chan error, 1)
				server := serveStream(t, setup.build(t, func(w http.ResponseWriter, r *http.Request) {
					got <- tt.call(http.NewResponseController(w))
				}))

				res, err := http.Get(server.URL + "/controller")
				if err != nil {
					t.Fatalf("get: %v", err)
				}
				res.Body.Close()

				if err := receiveFromHandler(t, got); err != nil {
					t.Fatalf("%s = %v, want nil", tt.name, err)
				}
			})
		}
	}
}

func TestMiddlewaresChainFollowsNetHTTPSemantics(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
		wantBody   string
		aborted    bool
	}{
		{name: "no write", wantStatus: http.StatusOK, handler: func(http.ResponseWriter, *http.Request) {}},
		{name: "header after write", wantStatus: http.StatusOK, wantBody: "body", handler: func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "body")
			w.Header().Set("X-Late", "yes")
		}},
		{name: "panic after write", aborted: true, handler: func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "partial")
			panic("boom")
		}},
	}
	for _, setup := range chainSetups {
		for _, tt := range tests {
			t.Run(setup.name+"/"+tt.name, func(t *testing.T) {
				server := serveStream(t, setup.build(t, tt.handler))

				res, err := http.Get(server.URL + "/semantics")
				var body []byte
				if err == nil {
					body, err = io.ReadAll(res.Body)
					res.Body.Close()
				}

				if tt.aborted {
					if err == nil {
						t.Fatalf("status = %d with a complete body, want the connection aborted", res.StatusCode)
					}
					return
				}
				if err != nil {
					t.Fatalf("get: %v", err)
				}
				if res.StatusCode != tt.wantStatus || string(body) != tt.wantBody {
					t.Fatalf("response = %d %q, want %d %q", res.StatusCode, body, tt.wantStatus, tt.wantBody)
				}
				if late := res.Header.Get("X-Late"); late != "" {
					t.Fatalf("X-Late = %q, want a header set after the first write not sent", late)
				}
			})
		}
	}
}
