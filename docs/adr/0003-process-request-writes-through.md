# 0003 — ProcessRequest writes through and passes upload bodies

- Status: accepted (2026-11-21)
- Issue: API-18 (#74)
- Release: v1.2.0 (MINOR)

## Context

`api.ProcessRequest` ran the handler against an `httptest.ResponseRecorder` and copied the recorded status, headers and body to the real `http.ResponseWriter` when the handler returned. Nothing streamed: `Flush` reached the recorder, `http.NewResponseController(w)` returned `http.ErrNotSupported` for deadlines, hijacking and full duplex, and every response sat whole in memory. It also read every request body before the handler and parsed it as JSON, so a `multipart/form-data` or `application/octet-stream` upload was answered with 400 and could not sit behind it. Issue #74 asked for both to change and for a per-route body limit.

## Decision

1. **Write through.** The handler gets a small unexported `statusWriter` over the real writer. It forwards every call and records only what `ProcessRequest` needs after the handler: the status for the `response` record and whether the response started. It implements `Flush`, `FlushError` and `Unwrap`, so a `w.(http.Flusher)` assertion and `http.ResponseController` (deadlines, `EnableFullDuplex`) reach the connection. It does not implement `http.Hijacker` or `io.ReaderFrom` itself. `ProcessRequest` does not see a hijack, so its `response` record and panic handling do not apply after one: a route that hijacks the connection (websockets) should be a `SkipPaths` route.
2. **Same logged status.** The recorded status follows `httptest.ResponseRecorder`: the first `WriteHeader` code wins, even a 1xx (`Warning` and `Informative` default to 100), and 200 applies on a body, a flush, or no write. `response` records keep their values.
3. **Late panic aborts.** A panic before the response started (no final status, no body, no flush; a 1xx other than 101 does not start it) is answered with 500 as before, with the same headers: `ProcessRequest` clones `w.Header()` before the handler runs and restores that clone, so headers the handler set for its own body (a stale `Content-Length`, `Content-Disposition`) are dropped. The clone costs one allocation per request. After that, a 500 can no longer replace the response, so the panic is logged as `panic recovered` and re-panicked as `http.ErrAbortHandler`: `net/http` aborts the connection without logging, the client sees a truncated response, and there is no `response` record. Panics with `http.ErrAbortHandler` keep their behavior.
4. **Upload bodies pass through.** A request with a body whose `Content-Type` is `multipart/*` or `application/octet-stream` is not read before the handler: `r.Body` stays as it came, still under the instance's `WithMaxBodyBytes` limit (or the default 10 MiB). The receiver runs on a copy of the request with `http.NoBody`, so header metadata, the event ID, the request log and the request context are produced as before; `Content` and `RawBody` are empty. `net/http` removes multipart temp files only for the request it created, so `ProcessRequest` calls `MultipartForm.RemoveAll` on the request it passed on after the handler returns or panics, and logs a failure as `multipart form cleanup failed`. Every other type, a missing or unparsable one included (`text/plain`, `application/x-www-form-urlencoded`, XML), keeps the read-and-decode path, so a client that sends JSON or an envelope under another label is served as before. Owner decision 2026-11-22: an allowlist of the upload types, not "anything not JSON", which would have passed such mislabeled JSON through and dropped its envelope header.
5. **No per-route body limit.** Declined: a nested instance does it (`api.New(api.WithMaxBodyBytes(n)).Handler(route)`; nested limits combine and the smallest applies), and a new option would be a second way to say the same thing (YAGNI-01).

No exported name changed; the deprecated `LogResponse(*httptest.ResponseRecorder)` and `LogResponseWithDuration` stay as they are.

## Consequences

- Streaming, `Flush` and `http.ResponseController` work behind `ProcessRequest`, and responses are no longer buffered twice. `BenchmarkProcessRequest` (Go 1.26.11 linux/amd64, `-count 5`): default instance 105 → 99 allocs/op, 11623 → 10677 B/op, about 20 → 15 µs/op.
- Uploads can be served behind `ProcessRequest`; the handler reads them with `r.ParseMultipartForm`, `r.MultipartReader` or `io.Copy`, and a body over the limit returns `*http.MaxBytesError` from that read instead of a 413 from `ProcessRequest`.
- Behavior changes that follow from `net/http` semantics, listed in the migration guide's v1.2.0 section:
  - a `multipart/*` or `application/octet-stream` body reaches the handler instead of a 400;
  - a panic after the first write aborts the connection instead of answering 500;
  - headers set after `WriteHeader` or the first `Write` are no longer sent (the recorder copied the final header map);
  - headers set by outer middlewares are visible to the handler on `w.Header()`;
  - a route that hijacks the connection belongs in `SkipPaths`.

## Alternatives

- **Keep the recorder and copy on flush.** Would keep header-after-write behavior, but duplicates `net/http`'s response state machine (KISS-01) and still cannot expose deadlines or hijacking.
- **An option or route opt-out to pass bodies through.** A new exported name for what the `Content-Type` already says; declined (YAGNI-01).
- **Answer 500 after a late panic by appending to the body.** The status and part of the body are already on the wire; appending an error body would produce a corrupt response that looks successful.
