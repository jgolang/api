package api

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer safe for the server goroutine to write while
// the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// records decodes the JSON records written so far.
func (b *syncBuffer) records(t *testing.T) []logRecord {
	t.Helper()
	b.mu.Lock()
	snapshot := bytes.NewBuffer(bytes.Clone(b.buf.Bytes()))
	b.mu.Unlock()
	return decodeRecords(t, snapshot)
}

// streamSetup builds the handler under test from ProcessRequest(next) and
// returns the buffer its records go to: on an instance, or on the default
// instance through slog.Default.
type streamSetup struct {
	name  string
	build func(t *testing.T, next http.HandlerFunc, opts ...Option) (http.Handler, *syncBuffer)
}

var streamSetups = []streamSetup{
	{name: "default instance", build: func(t *testing.T, next http.HandlerFunc, _ ...Option) (http.Handler, *syncBuffer) {
		t.Helper()
		logs := &syncBuffer{}
		previous, previousOutput, previousFlags := slog.Default(), log.Writer(), log.Flags()
		slog.SetDefault(slog.New(slog.NewJSONHandler(logs, nil)))
		t.Cleanup(func() {
			slog.SetDefault(previous)
			log.SetOutput(previousOutput)
			log.SetFlags(previousFlags)
		})
		return ProcessRequest(next), logs
	}},
	{name: "instance", build: func(t *testing.T, next http.HandlerFunc, opts ...Option) (http.Handler, *syncBuffer) {
		t.Helper()
		logs := &syncBuffer{}
		opts = append([]Option{WithLogger(slog.New(slog.NewJSONHandler(logs, nil)))}, opts...)
		return New(opts...).Handler(ProcessRequest(next)), logs
	}},
}

// receiveFromHandler returns what the handler sent on got. The handler
// sends before it returns, and the response is complete only after that, so
// an empty channel means the handler never ran.
func receiveFromHandler[T any](t *testing.T, got <-chan T) T {
	t.Helper()
	select {
	case value := <-got:
		return value
	default:
		t.Fatal("handler did not run")
		var zero T
		return zero
	}
}

// serveStream starts a test server for handler, closed when the test ends.
func serveStream(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func TestProcessRequestFlushReachesClientBeforeHandlerReturns(t *testing.T) {
	for _, setup := range streamSetups {
		t.Run(setup.name, func(t *testing.T) {
			release := make(chan struct{})
			handler, _ := setup.build(t, func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, "first")
				w.(http.Flusher).Flush()
				<-release
				io.WriteString(w, "second")
			})
			server := serveStream(t, handler)
			// A response held until the handler returns never arrives, since
			// the handler waits for the first chunk to be read: the client
			// times out, and release lets the server shut down.
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

func TestProcessRequestResponseControllerReachesConnection(t *testing.T) {
	for _, setup := range streamSetups {
		t.Run(setup.name, func(t *testing.T) {
			errs := make(chan []error, 1)
			handler, _ := setup.build(t, func(w http.ResponseWriter, r *http.Request) {
				controller := http.NewResponseController(w)
				errs <- []error{
					controller.SetWriteDeadline(time.Now().Add(time.Minute)),
					controller.EnableFullDuplex(),
					controller.Flush(),
				}
			})
			server := serveStream(t, handler)

			res, err := http.Get(server.URL + "/controller")
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			res.Body.Close()

			for _, err := range <-errs {
				if err != nil {
					t.Fatalf("ResponseController call = %v, want nil", err)
				}
			}
		})
	}
}

func TestProcessRequestPanic(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		aborted bool
	}{
		{name: "before writing", handler: func(http.ResponseWriter, *http.Request) { panic("boom") }},
		{name: "after a 1xx status", handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusEarlyHints)
			panic("boom")
		}},
		{name: "with an invalid status", handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(1000)
		}},
		{name: "after a status", aborted: true, handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			panic("boom")
		}},
		{name: "after a body", aborted: true, handler: func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "partial")
			panic("boom")
		}},
		{name: "after a flush", aborted: true, handler: func(w http.ResponseWriter, r *http.Request) {
			w.(http.Flusher).Flush()
			panic("boom")
		}},
		{name: "after a controller flush", aborted: true, handler: func(w http.ResponseWriter, r *http.Request) {
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Errorf("flush: %v", err)
			}
			panic("boom")
		}},
	}
	for _, setup := range streamSetups {
		for _, tt := range tests {
			t.Run(setup.name+"/"+tt.name, func(t *testing.T) {
				handler, logs := setup.build(t, tt.handler)
				server := serveStream(t, handler)

				res, err := http.Get(server.URL + "/panic")
				var status int
				if err == nil {
					status = res.StatusCode
					_, err = io.ReadAll(res.Body)
					res.Body.Close()
				}

				records := logs.records(t)
				recordNamed(t, records, "panic recovered")
				if tt.aborted {
					if err == nil {
						t.Fatalf("status = %d with a complete body, want the connection aborted", status)
					}
					for _, record := range records {
						if record["msg"] == "response" {
							t.Fatalf("records = %v, want no response record", records)
						}
					}
					return
				}
				if err != nil || status != http.StatusInternalServerError {
					t.Fatalf("response = %d, %v; want 500", status, err)
				}
			})
		}
	}
}

func TestProcessRequestPassesMultipartBodyThrough(t *testing.T) {
	for _, setup := range streamSetups {
		t.Run(setup.name, func(t *testing.T) {
			type received struct {
				field, file, eventID string
				err                  error
			}
			got := make(chan received, 1)
			handler, logs := setup.build(t, func(w http.ResponseWriter, r *http.Request) {
				var result received
				defer func() { got <- result }()
				rctx, err := GetRequestContext(r)
				if err != nil {
					result.err = err
					return
				}
				result.eventID = rctx.EventID
				if result.err = r.ParseMultipartForm(1 << 20); result.err != nil {
					return
				}
				result.field = r.FormValue("name")
				file, _, err := r.FormFile("upload")
				if err != nil {
					result.err = err
					return
				}
				defer file.Close()
				content, err := io.ReadAll(file)
				result.file, result.err = string(content), err
			})
			server := serveStream(t, handler)
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			form.WriteField("name", "report")
			part, _ := form.CreateFormFile("upload", "report.txt")
			io.WriteString(part, "file content")
			form.Close()

			res, err := http.Post(server.URL+"/upload", form.FormDataContentType(), &body)
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			res.Body.Close()
			result := receiveFromHandler(t, got)

			if result.err != nil || result.field != "report" || result.file != "file content" {
				t.Fatalf("handler got field %q, file %q, err %v; want the parts", result.field, result.file, result.err)
			}
			if result.eventID == "" {
				t.Fatal("event ID is empty, want one")
			}
			recordNamed(t, logs.records(t), "request")
		})
	}
}

func TestProcessRequestPassesBodyThroughInEveryRequestMode(t *testing.T) {
	for _, mode := range []RequestMode{RequestModePlain, RequestModeEnvelope, RequestModeAuto} {
		t.Run(string(mode), func(t *testing.T) {
			type received struct {
				token, body string
				content     int
				err         error
			}
			got := make(chan received, 1)
			handler := New(WithRequestMode(mode)).Handler(ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
				var result received
				defer func() { got <- result }()
				rctx, err := GetRequestContext(r)
				if err != nil {
					result.err = err
					return
				}
				result.token, result.content = rctx.SecurityToken, len(rctx.Content)
				body, err := io.ReadAll(r.Body)
				result.body, result.err = string(body), err
			}))
			server := serveStream(t, handler)
			payload := `{"header":{"token":"in-body"},"content":{}}`
			req, _ := http.NewRequest(http.MethodPost, server.URL+"/upload", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set(SecurityTokenHeaderKey, "token-placeholder")

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			res.Body.Close()
			result := receiveFromHandler(t, got)

			if res.StatusCode != http.StatusOK || result.err != nil {
				t.Fatalf("status %d, handler err %v; want 200 and the body", res.StatusCode, result.err)
			}
			if result.body != payload || result.content != 0 {
				t.Fatalf("handler got body %q and %d content bytes, want the body unread and no content", result.body, result.content)
			}
			if result.token != "token-placeholder" {
				t.Fatalf("token = %q, want the header token", result.token)
			}
		})
	}
}

func TestProcessRequestPassesOctetStreamBodyThrough(t *testing.T) {
	for _, setup := range streamSetups {
		t.Run(setup.name, func(t *testing.T) {
			payload := make([]byte, 1<<20)
			rand.Read(payload)
			got := make(chan []byte, 1)
			handler, _ := setup.build(t, func(w http.ResponseWriter, r *http.Request) {
				content, _ := io.ReadAll(r.Body)
				got <- content
			})
			server := serveStream(t, handler)

			res, err := http.Post(server.URL+"/blob", "application/octet-stream", bytes.NewReader(payload))
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			res.Body.Close()

			if content := receiveFromHandler(t, got); !bytes.Equal(content, payload) {
				t.Fatalf("handler read %d bytes, want the %d sent unchanged", len(content), len(payload))
			}
		})
	}
}

func TestProcessRequestPassthroughBodyKeepsLimit(t *testing.T) {
	tests := []struct {
		name string
		wrap func(http.Handler) http.Handler
		size int
	}{
		{name: "instance", size: 9, wrap: func(next http.Handler) http.Handler {
			return New(WithLogger(slog.New(slog.DiscardHandler)), WithMaxBodyBytes(8)).Handler(next)
		}},
		{name: "default instance", size: defaultMaxBodyBytes + 1, wrap: func(next http.Handler) http.Handler {
			captureDefaultLogger(t)
			return next
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make(chan error, 1)
			handler := tt.wrap(ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
				_, err := io.ReadAll(r.Body)
				got <- err
			}))
			req := httptest.NewRequest(http.MethodPost, "/blob", bytes.NewReader(make([]byte, tt.size)))
			req.Header.Set("Content-Type", "application/octet-stream")

			handler.ServeHTTP(httptest.NewRecorder(), req)

			var maxBytesErr *http.MaxBytesError
			if err := receiveFromHandler(t, got); !errors.As(err, &maxBytesErr) {
				t.Fatalf("read error = %v, want *http.MaxBytesError", err)
			}
		})
	}
}

func TestProcessRequestEarlyPanicDropsHandlerHeaders(t *testing.T) {
	for _, setup := range streamSetups {
		t.Run(setup.name, func(t *testing.T) {
			handler, _ := setup.build(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "5000")
				w.Header().Set("Content-Disposition", `attachment; filename="report.csv"`)
				panic("boom")
			})
			outer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Outer", "kept")
				handler.ServeHTTP(w, r)
			})
			server := serveStream(t, outer)

			res, err := http.Get(server.URL + "/panic")
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			defer res.Body.Close()
			body, readErr := io.ReadAll(res.Body)

			if readErr != nil || res.StatusCode != http.StatusInternalServerError || !json.Valid(body) {
				t.Fatalf("response = %d %q, %v; want a complete 500 JSON body", res.StatusCode, body, readErr)
			}
			if res.Header.Get("Content-Disposition") != "" || res.ContentLength == 5000 {
				t.Fatalf("headers = %v, want the handler's body headers dropped", res.Header)
			}
			if res.Header.Get("X-Outer") != "kept" {
				t.Fatalf("headers = %v, want the outer middleware's X-Outer kept", res.Header)
			}
		})
	}
}

func TestProcessRequestRemovesMultipartTempFiles(t *testing.T) {
	endings := []struct {
		name   string
		finish func(w http.ResponseWriter)
	}{
		{name: "handler returns", finish: func(http.ResponseWriter) {}},
		{name: "handler panics", finish: func(http.ResponseWriter) { panic("boom") }},
		{name: "handler panics after writing", finish: func(w http.ResponseWriter) {
			io.WriteString(w, "partial")
			panic("boom")
		}},
	}
	for _, setup := range streamSetups {
		for _, ending := range endings {
			t.Run(setup.name+"/"+ending.name, func(t *testing.T) {
				removesMultipartTempFile(t, setup, ending.finish)
			})
		}
	}
}

// removesMultipartTempFile posts a multipart upload whose handler parses it
// to disk and then calls finish, and checks the temp file is gone after the
// response.
func removesMultipartTempFile(t *testing.T, setup streamSetup, finish func(w http.ResponseWriter)) {
	t.Helper()
	tempFile := make(chan string, 1)
	handler, _ := setup.build(t, func(w http.ResponseWriter, r *http.Request) {
		// maxMemory 1 puts the file part on disk.
		if err := r.ParseMultipartForm(1); err != nil {
			t.Errorf("parse: %v", err)
			return
		}
		file, _, err := r.FormFile("upload")
		if err != nil {
			t.Errorf("form file: %v", err)
			return
		}
		if osFile, ok := file.(*os.File); ok {
			tempFile <- osFile.Name()
		}
		file.Close()
		finish(w)
	})
	server := serveStream(t, handler)
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("upload", "report.txt")
	io.WriteString(part, strings.Repeat("x", 1<<12))
	form.Close()

	// An aborted response fails the read; only the cleanup matters.
	if res, err := http.Post(server.URL+"/upload", form.FormDataContentType(), &body); err == nil {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
	}

	name := receiveFromHandler(t, tempFile)
	if _, err := os.Stat(name); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat %s = %v, want the temp file removed", name, err)
	}
}

func TestProcessRequestReadsJSONBody(t *testing.T) {
	// Only multipart and octet-stream bodies pass through; every other type
	// keeps the read, so a client labeling JSON as a form or text is served
	// as before v1.2.0.
	contentTypes := []string{
		"", "application/json", "application/json; charset=utf-8", "application/problem+json", ";;;",
		"text/plain", "text/plain; charset=utf-8", "application/x-www-form-urlencoded", "application/xml",
	}
	for _, contentType := range contentTypes {
		t.Run(contentType, func(t *testing.T) {
			got := make(chan string, 1)
			handler := New(WithLogger(slog.New(slog.DiscardHandler))).Handler(ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
				content, _ := io.ReadAll(r.Body)
				got <- string(content)
			}))
			req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader("not json"))
			req.Header.Set("Content-Type", contentType)
			res := httptest.NewRecorder()

			handler.ServeHTTP(res, req)

			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 for a body read as JSON", res.Code)
			}
			select {
			case content := <-got:
				t.Fatalf("handler ran with %q, want the body rejected", content)
			default:
			}
		})
	}
}

func TestProcessRequestLogsWrittenStatus(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		status  int
		// logged is the status of the response record when it differs
		// from the one the client gets.
		logged float64
	}{
		{name: "no write", handler: func(http.ResponseWriter, *http.Request) {}, status: http.StatusOK},
		{name: "created", handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.WriteHeader(http.StatusAccepted)
		}, status: http.StatusCreated},
		{name: "body before status", handler: func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "body")
			w.WriteHeader(http.StatusInternalServerError)
		}, status: http.StatusOK},
		// The recorder ProcessRequest used before v1.2.0 kept the first
		// code, so Warning's default 100 stays the logged status.
		{name: "informational first", handler: func(w http.ResponseWriter, r *http.Request) {
			Warning{}.Write(w, r)
		}, status: http.StatusOK, logged: http.StatusContinue},
	}
	for _, setup := range streamSetups {
		for _, tt := range tests {
			t.Run(setup.name+"/"+tt.name, func(t *testing.T) {
				handler, logs := setup.build(t, tt.handler)
				server := serveStream(t, handler)

				res, err := http.Get(server.URL + "/status")
				if err != nil {
					t.Fatalf("get: %v", err)
				}
				res.Body.Close()

				if res.StatusCode != tt.status {
					t.Fatalf("status = %d, want %d", res.StatusCode, tt.status)
				}
				logged := tt.logged
				if logged == 0 {
					logged = float64(tt.status)
				}
				if record := recordNamed(t, logs.records(t), "response"); record["status"] != logged {
					t.Fatalf("logged status = %v, want %v", record["status"], logged)
				}
			})
		}
	}
}

// The recorder ProcessRequest used before v1.2.0 ignored a repeated
// WriteHeader. Forwarding it makes net/http log a superfluous call that
// names the wrapper instead of the handler.
func TestProcessRequestIgnoresRepeatedWriteHeader(t *testing.T) {
	handlers := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "after a status", handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.WriteHeader(http.StatusAccepted)
		}},
		{name: "after a body", handler: func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "body")
			w.WriteHeader(http.StatusAccepted)
		}},
	}
	for _, setup := range streamSetups {
		for _, tt := range handlers {
			t.Run(setup.name+"/"+tt.name, func(t *testing.T) {
				handler, _ := setup.build(t, tt.handler)
				serverLogs := &syncBuffer{}
				server := httptest.NewUnstartedServer(handler)
				server.Config.ErrorLog = log.New(serverLogs, "", 0)
				server.Start()
				t.Cleanup(server.Close)

				res, err := http.Get(server.URL + "/twice")
				if err != nil {
					t.Fatalf("get: %v", err)
				}
				res.Body.Close()

				serverLogs.mu.Lock()
				logged := serverLogs.buf.String()
				serverLogs.mu.Unlock()
				if logged != "" {
					t.Fatalf("server log = %q, want nothing", logged)
				}
			})
		}
	}
}
