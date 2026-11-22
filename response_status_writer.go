package api

import "net/http"

// statusWriter writes through to the http.ResponseWriter it wraps and
// remembers what ProcessRequest needs after the handler: the status to log
// and whether the response has started. The status follows
// httptest.ResponseRecorder, which ProcessRequest used before, so the logged
// status does not change: the first WriteHeader code wins, even a 1xx, and
// 200 applies when the body or a flush comes first or nothing is written.
//
// Hijack, ReadFrom and the other optional interfaces are not implemented
// here; http.NewResponseController reaches them through Unwrap.
type statusWriter struct {
	http.ResponseWriter
	status int
	// started reports whether a final status or body was written: from then
	// on a 500 can no longer replace the response. A 1xx other than 101 is
	// informational in net/http and does not start it.
	started bool
}

// WriteHeader records the first code and forwards it. A call after the
// response started is dropped, as the recorder did: net/http would only log
// it as superfluous, naming this wrapper instead of the handler.
func (w *statusWriter) WriteHeader(code int) {
	if w.started {
		return
	}
	if w.status == 0 {
		w.status = code
	}
	// Set after the wrapped call: an invalid code makes net/http panic
	// before anything is written, and that panic still gets the 500.
	w.ResponseWriter.WriteHeader(code)
	if code >= http.StatusOK || code == http.StatusSwitchingProtocols {
		w.started = true
	}
}

// Write forwards b; a body without a status starts the response with 200.
func (w *statusWriter) Write(b []byte) (int, error) {
	w.startResponse()
	return w.ResponseWriter.Write(b)
}

// Flush flushes the wrapped writer when it is an http.Flusher, so a handler
// that asserts w.(http.Flusher) streams as it would without ProcessRequest.
func (w *statusWriter) Flush() {
	w.startResponse()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// FlushError flushes the wrapped writer and returns its error, for
// http.ResponseController.
func (w *statusWriter) FlushError() error {
	w.startResponse()
	return http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap returns the wrapped writer, for http.ResponseController.
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// loggedStatus returns the status to log: 200 when nothing was written.
func (w *statusWriter) loggedStatus() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// startResponse records what net/http does on a body or flush before any
// final status: the response starts with 200.
func (w *statusWriter) startResponse() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.started = true
}
