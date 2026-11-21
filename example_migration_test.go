package api_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/jgolang/api"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

// The examples in this file hold the Go snippets of docs/migration-v1.md, so
// the guide's before/after code compiles and answers as it says.

var migrationTitleRequired = codes.New("TSK-001", "title is required").WithKind(codes.KindValidation)

// migrationCreateTask is the guide's handler. It is the same before and after
// the migration: only the start-up wiring changes.
func migrationCreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error400().Write(w, r)
		return
	}
	if req.Title == "" {
		api.WriteError(w, r, errors.NewC(migrationTitleRequired, "empty title"))
		return
	}
	api.SuccessWithContent(req).Write(w, r)
}

// serveMigration sends body to handler with a fixed event ID, so the output
// does not change between runs.
func serveMigration(handler http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(body))
	req.Header.Set(api.EventIDHeaderKey, "evt1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// Example_migrationBeforeAndAfter moves a v0 program that configures the
// package-level modes to an instance. With WithLowercaseCodes the body is
// byte-identical; with New() alone the code is sent as declared.
func Example_migrationBeforeAndAfter() {
	previousRequestMode, previousResponseMode := api.CurrentRequestMode, api.CurrentResponseMode
	defer func() {
		api.SetRequestMode(previousRequestMode)
		api.SetResponseMode(previousResponseMode)
	}()

	// Before (v0): package-level settings, the router served as is.
	api.SetRequestMode(api.RequestModePlain)
	api.SetResponseMode(api.ResponseModePlain)
	before := http.HandlerFunc(migrationCreateTask)

	// After (v1): one instance, the router wrapped by its Handler.
	lowercase := api.New(
		api.WithRequestMode(api.RequestModePlain),
		api.WithResponseMode(api.ResponseModePlain),
		api.WithLowercaseCodes(),
	).Handler(http.HandlerFunc(migrationCreateTask))
	declared := api.New(
		api.WithRequestMode(api.RequestModePlain),
		api.WithResponseMode(api.ResponseModePlain),
	).Handler(http.HandlerFunc(migrationCreateTask))

	beforeBody := serveMigration(before, `{"title":""}`).Body.String()
	lowercaseBody := serveMigration(lowercase, `{"title":""}`).Body.String()
	declaredRec := serveMigration(declared, `{"title":""}`)

	fmt.Println("identical with WithLowercaseCodes:", beforeBody == lowercaseBody)
	fmt.Print("before:   ", beforeBody)
	fmt.Print("New():    ", declaredRec.Body.String())
	fmt.Println("status:", declaredRec.Code)
	// Output:
	// identical with WithLowercaseCodes: true
	// before:   {"action":"","code":"tsk-001","content":null,"event_id":"evt1","info":null,"message":"title is required (evt1-tsk-001)","title":"Error response!","token":"","type":"error"}
	// New():    {"action":"","code":"TSK-001","content":null,"event_id":"evt1","info":null,"message":"title is required (evt1-TSK-001)","title":"Error response!","token":"","type":"error"}
	// status: 400
}

// Example_migrationOptions wires every option that replaces a deprecated
// package-level name.
func Example_migrationOptions() {
	logger := slog.New(slog.DiscardHandler) // your service's *slog.Logger

	a := api.New(
		api.WithRequestMode(api.RequestModePlain),          // SetRequestMode, CurrentRequestMode
		api.WithResponseMode(api.ResponseModeEnvelope),     // SetResponseMode, CurrentResponseMode
		api.WithRequestReceiver(api.RequestReceiver{}),     // RegisterNewAPIRequestReceiver
		api.WithResponseFormatter(api.ResponseFormatter{}), // RegisterNewAPIResponseFormatter
		api.WithResponseWriter(api.ResponseWriter{}),       // RegisterNewAPIResponseWriter
		api.WithLogger(logger),                             // Print, PrintError, Fatal, Log*
		api.WithSensitiveLogKeys("pin"),                    // SensitiveLogKeys
	)

	rec := serveMigration(a.Handler(api.ProcessRequest(migrationCreateTask)), `{"title":"Buy milk"}`)
	fmt.Println(rec.Code, rec.Header().Get("Content-Type"))
	// Output: 200 application/json
}

// Example_migrationKindStatusAndHook shows the two WriteError options that
// have no v0 equivalent.
func Example_migrationKindStatusAndHook() {
	a := api.New(
		api.WithKindStatus(codes.KindValidation, http.StatusUnprocessableEntity),
		api.WithErrorHook(func(r *http.Request, err error, status int) {
			// Runs only from WriteError, before the response is written, and
			// may run concurrently: keep it safe for concurrent use.
			fmt.Println("hook:", status, errors.CodeOf(err).Str())
		}),
	)

	rec := serveMigration(a.Handler(http.HandlerFunc(migrationCreateTask)), `{"title":""}`)
	fmt.Println("status:", rec.Code)
	// Output:
	// hook: 422 TSK-001
	// status: 422
}

// Example_migrationProblemDetails switches an instance's error responses to
// RFC 9457 problem details.
func Example_migrationProblemDetails() {
	a := api.New(api.WithResponseMode(api.ResponseModeProblem))

	rec := serveMigration(a.Handler(http.HandlerFunc(migrationCreateTask)), `{"title":""}`)
	fmt.Println(rec.Code, rec.Header().Get("Content-Type"))
	fmt.Print(rec.Body.String())
	// Output:
	// 400 application/problem+json
	// {"type":"about:blank","title":"Error response!","status":400,"detail":"title is required","code":"TSK-001","event_id":"evt1"}
}
