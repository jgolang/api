package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

var (
	planNotFound = codes.New("PLN-111", "task not found").WithKind(codes.KindNotFound)
	planConflict = codes.New("PLN-200", "task is locked").WithKind(codes.KindConflict)
)

// hasCodeMessage tells whether message is code's message followed by the
// trace suffix the formatter appends.
func hasCodeMessage(message string, code codes.Code) bool {
	return strings.HasPrefix(message, code.Msg()+" (")
}

// errorHeader is the part of an envelope error body these tests check.
type errorHeader struct {
	Title   string `json:"title"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeErrorThrough answers one request with WriteError(err), wrapped by a
// unless a is nil, and returns the request WriteError received.
func writeErrorThrough(rec *httptest.ResponseRecorder, a *API, err error) *http.Request {
	var received *http.Request
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r
		WriteError(w, r, err)
	})
	if a != nil {
		handler = a.Handler(handler)
	}
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tasks", nil))
	return received
}

// writeErrorStatus returns the status WriteError(err) answers with through a.
func writeErrorStatus(a *API, err error) int {
	rec := httptest.NewRecorder()
	writeErrorThrough(rec, a, err)
	return rec.Code
}

func decodeErrorHeader(t *testing.T, rec *httptest.ResponseRecorder) errorHeader {
	t.Helper()
	var body struct {
		Header errorHeader `json:"header"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return body.Header
}

func TestWithKindStatusOverridesOnlyItsKind(t *testing.T) {
	a := New(WithKindStatus(codes.KindValidation, http.StatusUnprocessableEntity))
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"overridden kind behind fmt.Errorf", fmt.Errorf("x: %w", errors.NewC(planValidation, "title")), http.StatusUnprocessableEntity, "PLN-001"},
		{"other kind keeps StatusForKind", errors.NewC(planNotFound, "task 42"), http.StatusNotFound, "PLN-111"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			writeErrorThrough(rec, a, tt.err)

			header := decodeErrorHeader(t, rec)
			if rec.Code != tt.wantStatus || header.Code != tt.wantCode {
				t.Fatalf("status, code = %d, %q; want %d, %q", rec.Code, header.Code, tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestWithKindStatusKeepsTheCodeMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	a := New(WithKindStatus(codes.KindValidation, http.StatusUnprocessableEntity))

	writeErrorThrough(rec, a, fmt.Errorf("x: %w", errors.NewC(planValidation, "title")))

	if header := decodeErrorHeader(t, rec); !hasCodeMessage(header.Message, planValidation) || header.Title != DefaultErrorTitle {
		t.Fatalf("title, message = %q, %q; want %q and %q", header.Title, header.Message, DefaultErrorTitle, planValidation.Msg())
	}
}

func TestWriteErrorWithoutOverridesUsesStatusForKind(t *testing.T) {
	instances := map[string]*API{"default instance": nil, "instance without overrides": New()}
	errs := map[string]error{
		"validation": errors.NewC(planValidation, "title"),
		"not found":  errors.NewC(planNotFound, "task 42"),
		"uncoded":    fmt.Errorf("boom"),
		"nil":        nil,
	}
	want := map[string]int{"validation": 400, "not found": 404, "uncoded": 500, "nil": 500}

	for instanceName, a := range instances {
		for errName, err := range errs {
			t.Run(instanceName+"/"+errName, func(t *testing.T) {
				if got := writeErrorStatus(a, err); got != want[errName] {
					t.Fatalf("status = %d, want %d", got, want[errName])
				}
			})
		}
	}
}

func TestWithKindStatusIgnoresStatusesOutsideTheErrorRange(t *testing.T) {
	for _, status := range []int{0, 99, 200, 302, 399, 600, 1000} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			a := New(WithKindStatus(codes.KindValidation, status))

			if got := writeErrorStatus(a, errors.NewC(planValidation, "title")); got != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", got)
			}
		})
	}
}

func TestWithKindStatusAcceptsTheErrorRangeBounds(t *testing.T) {
	for _, status := range []int{400, 599} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			a := New(WithKindStatus(codes.KindNotFound, status))

			if got := writeErrorStatus(a, errors.NewC(planNotFound, "task 42")); got != status {
				t.Fatalf("status = %d, want %d", got, status)
			}
		})
	}
}

func TestWithKindStatusIgnoresTheEmptyKind(t *testing.T) {
	a := New(WithKindStatus("", http.StatusServiceUnavailable))
	errs := map[string]error{
		"uncoded":           fmt.Errorf("boom"),
		"nil":               nil,
		"code without kind": errors.NewC(codes.New("X-1", "x"), "x"),
	}
	for name, err := range errs {
		t.Run(name, func(t *testing.T) {
			if got := writeErrorStatus(a, err); got != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", got)
			}
		})
	}
}

func TestWithKindStatusLaterCallForTheSameKindWins(t *testing.T) {
	a := New(
		WithKindStatus(codes.KindValidation, http.StatusUnprocessableEntity),
		WithKindStatus(codes.KindValidation, http.StatusTeapot),
	)

	if got := writeErrorStatus(a, errors.NewC(planValidation, "title")); got != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", got, http.StatusTeapot)
	}
}

func TestWithKindStatusTitleFollowsTheDecidedStatus(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantTitle string
	}{
		{"500 takes the internal title", http.StatusInternalServerError, InternalServerTitle},
		{"403 takes the unauthorized title", http.StatusForbidden, UnauthorizedTitle},
		{"401 takes the unauthorized title", http.StatusUnauthorized, UnauthorizedTitle},
		{"other status takes the default title", http.StatusLocked, DefaultErrorTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			a := New(WithKindStatus(codes.KindConflict, tt.status))

			writeErrorThrough(rec, a, errors.NewC(planConflict, "locked"))

			header := decodeErrorHeader(t, rec)
			if rec.Code != tt.status || header.Title != tt.wantTitle {
				t.Fatalf("status, title = %d, %q; want %d, %q", rec.Code, header.Title, tt.status, tt.wantTitle)
			}
			if header.Code != "PLN-200" || !hasCodeMessage(header.Message, planConflict) {
				t.Fatalf("code, message = %q, %q; want PLN-200, %q", header.Code, header.Message, planConflict.Msg())
			}
		})
	}
}

// hookCall records one call of an error hook.
type hookCall struct {
	r                  *http.Request
	err                error
	status             int
	bodyLength         int
	contentTypeWritten string
}

// recordingHook returns a hook that appends its calls to calls, reading rec
// to tell what was written before the hook ran.
func recordingHook(rec *httptest.ResponseRecorder, calls *[]hookCall) func(*http.Request, error, int) {
	return func(r *http.Request, err error, status int) {
		*calls = append(*calls, hookCall{
			r:                  r,
			err:                err,
			status:             status,
			bodyLength:         rec.Body.Len(),
			contentTypeWritten: rec.Header().Get("Content-Type"),
		})
	}
}

func TestWithErrorHookIsCalledOnceBeforeTheResponseIsWritten(t *testing.T) {
	rec := httptest.NewRecorder()
	var calls []hookCall
	a := New(
		WithKindStatus(codes.KindValidation, http.StatusUnprocessableEntity),
		WithErrorHook(recordingHook(rec, &calls)),
	)
	err := fmt.Errorf("saving: %w", errors.NewC(planValidation, "title"))

	received := writeErrorThrough(rec, a, err)

	if len(calls) != 1 {
		t.Fatalf("hook called %d times, want 1", len(calls))
	}
	call := calls[0]
	if call.r != received || call.err != err || call.status != http.StatusUnprocessableEntity {
		t.Fatalf("hook got request %p, err %v, status %d; want %p, %v, 422", call.r, call.err, call.status, received, err)
	}
	if call.bodyLength != 0 || call.contentTypeWritten != "" {
		t.Fatalf("at hook time body length = %d, Content-Type = %q; want nothing written", call.bodyLength, call.contentTypeWritten)
	}
	if rec.Code != http.StatusUnprocessableEntity || rec.Body.Len() == 0 {
		t.Fatalf("response status %d with %d body bytes, want 422 with a body", rec.Code, rec.Body.Len())
	}
}

func TestWithErrorHookReceivesANilError(t *testing.T) {
	rec := httptest.NewRecorder()
	var calls []hookCall
	a := New(WithErrorHook(recordingHook(rec, &calls)))

	writeErrorThrough(rec, a, nil)

	if len(calls) != 1 || calls[0].err != nil || calls[0].status != http.StatusInternalServerError {
		t.Fatalf("hook calls = %+v, want one with a nil error and 500", calls)
	}
}

func TestWithErrorHookOnlyRunsForItsInstance(t *testing.T) {
	var calls atomic.Int32
	withHook := New(WithErrorHook(func(*http.Request, error, int) { calls.Add(1) }))
	err := errors.NewC(planValidation, "title")

	writeErrorStatus(withHook, err)
	writeErrorStatus(New(), err)
	writeErrorStatus(nil, err)

	if got := calls.Load(); got != 1 {
		t.Fatalf("hook called %d times, want 1", got)
	}
}

func TestWithErrorHookLaterCallReplacesTheEarlier(t *testing.T) {
	var first, second atomic.Int32
	tests := []struct {
		name       string
		a          *API
		wantFirst  int32
		wantSecond int32
	}{
		{"later hook wins", New(
			WithErrorHook(func(*http.Request, error, int) { first.Add(1) }),
			WithErrorHook(func(*http.Request, error, int) { second.Add(1) }),
		), 0, 1},
		{"nil removes the hook", New(
			WithErrorHook(func(*http.Request, error, int) { first.Add(1) }),
			WithErrorHook(nil),
		), 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first.Store(0)
			second.Store(0)

			status := writeErrorStatus(tt.a, errors.NewC(planValidation, "title"))

			if first.Load() != tt.wantFirst || second.Load() != tt.wantSecond || status != http.StatusBadRequest {
				t.Fatalf("first, second, status = %d, %d, %d; want %d, %d, 400", first.Load(), second.Load(), status, tt.wantFirst, tt.wantSecond)
			}
		})
	}
}

func TestWithErrorHookIsNotCalledByTheErrorHelpers(t *testing.T) {
	var calls atomic.Int32
	a := New(WithErrorHook(func(*http.Request, error, int) { calls.Add(1) }))
	helpers := map[string]http.HandlerFunc{
		"Error400": func(w http.ResponseWriter, r *http.Request) {
			Error400(errors.NewC(planValidation, "title")).Write(w, r)
		},
		"Error500": func(w http.ResponseWriter, r *http.Request) { Error500().Write(w, r) },
		"Error":    func(w http.ResponseWriter, r *http.Request) { Error{HTTPStatusCode: http.StatusConflict}.Write(w, r) },
	}
	for name, helper := range helpers {
		t.Run(name, func(t *testing.T) {
			serve(a.Handler(helper), http.MethodGet, "", nil)

			if got := calls.Load(); got != 0 {
				t.Fatalf("hook called %d times, want 0", got)
			}
		})
	}
}

func TestWithErrorHookPanicPropagatesBeforeWriting(t *testing.T) {
	rec := httptest.NewRecorder()
	a := New(WithErrorHook(func(*http.Request, error, int) { panic("hook failed") }))

	recovered := func() (recovered any) {
		defer func() { recovered = recover() }()
		writeErrorThrough(rec, a, errors.NewC(planValidation, "title"))
		return nil
	}()

	if recovered != "hook failed" {
		t.Fatalf("recovered %v, want the hook panic", recovered)
	}
	if rec.Body.Len() != 0 || rec.Header().Get("Content-Type") != "" || rec.Flushed {
		t.Fatalf("response written before the hook panic: %d body bytes, Content-Type %q", rec.Body.Len(), rec.Header().Get("Content-Type"))
	}
}

func TestWithErrorHookPanicUnderProcessRequestAnswers500(t *testing.T) {
	logger, buf := newRecordingLogger()
	a := New(WithLogger(logger), WithErrorHook(func(*http.Request, error, int) { panic("hook failed") }))
	handler := a.Handler(ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, errors.NewC(planValidation, "title"))
	}))

	rec := serve(handler, http.MethodGet, "", nil)

	logged := strings.Contains(buf.String(), `"msg":"panic recovered"`)
	if rec.Code != http.StatusInternalServerError || !logged {
		t.Fatalf("status = %d, logged %v; want 500 and a panic record", rec.Code, logged)
	}
}

func TestInstancesWithOverridesAndHooksServeConcurrently(t *testing.T) {
	var unprocessableCalls, conflictCalls atomic.Int32
	unprocessable := New(
		WithKindStatus(codes.KindValidation, http.StatusUnprocessableEntity),
		WithErrorHook(func(_ *http.Request, _ error, status int) {
			if status == http.StatusUnprocessableEntity {
				unprocessableCalls.Add(1)
			}
		}),
	)
	conflict := New(
		WithKindStatus(codes.KindValidation, http.StatusConflict),
		WithErrorHook(func(_ *http.Request, _ error, status int) {
			if status == http.StatusConflict {
				conflictCalls.Add(1)
			}
		}),
	)
	instances := map[int]*API{http.StatusUnprocessableEntity: unprocessable, http.StatusConflict: conflict, http.StatusBadRequest: nil}
	err := errors.NewC(planValidation, "title")
	const rounds = 20

	var wg sync.WaitGroup
	for range rounds {
		for want, a := range instances {
			wg.Go(func() {
				if got := writeErrorStatus(a, err); got != want {
					t.Errorf("status = %d, want %d", got, want)
				}
			})
		}
	}
	wg.Wait()

	if unprocessableCalls.Load() != rounds || conflictCalls.Load() != rounds {
		t.Fatalf("hook calls = %d, %d; want %d each", unprocessableCalls.Load(), conflictCalls.Load(), rounds)
	}
}
