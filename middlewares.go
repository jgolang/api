package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/jgolang/api/core"
)

var (
	// DefaultInvalidAuthHeaderMsg default invalid authorization message.
	DefaultInvalidAuthHeaderMsg = "Invalid Authorization header!"

	// DefaultBasicUnauthorizedMsg default basic authetication method unauthorized message.
	DefaultBasicUnauthorizedMsg = "Invalid basic token"

	// DefaultBearerUnauthorizedMsg default bearer authentication method unauthorized message.
	DefaultBearerUnauthorizedMsg = "Invalid bearer token"

	// CustomTokenPrefix custom token authorization method prefix.
	CustomTokenPrefix = "Bearer"

	// DefaultCustomUnauthorizedMsg default custom token authorization method unauthorized message.
	DefaultCustomUnauthorizedMsg = fmt.Sprintf("Invalid %v token", CustomTokenPrefix)
)

// MiddlewaresChain provides syntactic sugar to create a new middleware
// which will be the result of chaining the ones received as parameters
var MiddlewaresChain = core.MiddlewaresChain

// BasicToken validates basic authentication token middleware.
//
// With the default validator it answers 401 to every request while
// Username or Password is empty or "default"; set both (or replace
// ValidateBasicAuthCredentialsFunc) to use it. After a successful check it
// copies the user name into the Basic-Client request header. Since v1.0.0
// it no longer copies the password into a Basic-Secret header, and it
// removes one the client sent, so a handler never reads a password there.
func BasicToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(auth) != 2 || auth[0] != "Basic" {
			w.Header().Set("WWW-Authenticate", `Basic realm="api"`)
			response := Error401()
			response.Message = DefaultInvalidAuthHeaderMsg
			response.Write(w, r)
			return
		}
		client, _, tokenValid := ValidateBasicToken(auth[1])
		if !tokenValid {
			response := Error401()
			response.Message = DefaultBasicUnauthorizedMsg
			response.Write(w, r)
			return
		}
		r.Header.Set("Basic-Client", client)
		r.Header.Del("Basic-Secret")
		next(w, r)
	}
}

// CustomToken middleware to validates custom token authorization method.
func CustomToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(auth) != 2 || auth[0] != CustomTokenPrefix {
			w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
			response := Error401()
			response.Message = DefaultInvalidAuthHeaderMsg
			response.Write(w, r)
			return
		}
		tokenInfo, tokenValid := ValidateCustomToken(auth[1])
		if !tokenValid {
			response := Error401()
			response.Message = DefaultBearerUnauthorizedMsg
			response.Write(w, r)
			return
		}
		buf, _ := tokenInfo.MarshalJSON()
		r.Header.Set("TokenInfo", string(buf))
		next(w, r)
	}
}

// RequestHeaderJSON validate header Content-Type, is required and equal to application/json
func RequestHeaderJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requestHasBody(r) {
			next.ServeHTTP(w, r)
			return
		}
		contentType := r.Header.Get("Content-Type")
		if len(contentType) == 0 {
			Error{
				Message:        "No content-type!",
				HTTPStatusCode: http.StatusUnsupportedMediaType,
			}.Write(w, r)
			return
		}
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			Error{
				Message:        "Content-Type not is JSON!",
				ResponseCode:   ResponseCodes.InvalidJSON,
				HTTPStatusCode: http.StatusUnsupportedMediaType,
			}.Write(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// RequestHeaderSession checks that the request carries a session ID.
//
// It answers 401 when the SecurityTokenHeaderKey request header is empty.
// Since v1.0.0 it no longer copies that header into the response header, so
// the request token is never echoed (SEC-08); a handler that wants to return
// a token sets the header itself.
func RequestHeaderSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.Header.Get(SecurityTokenHeaderKey)
		if sessionID == "" {
			response := Error401()
			response.Message = "Invalid session ID"
			response.Write(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// RequestBody wrapper middleware
var RequestBody = NewRequestBodyMiddleware(PPPMethodsKey)

// NewRequestBodyMiddleware doc ...
func NewRequestBodyMiddleware(keyListMethods string) core.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if ValidateMethods(keyListMethods, r.Method) {
				a := instanceFrom(r)
				boundDefaultBody(w, r)
				requestData, err := a.core.ProcessBody(r)
				if err != nil {
					a.logBodyError(r, "request body rejected", err, false)
					bodyRejection(err).Write(w, r)
					return
				}
				r.Body = io.NopCloser(bytes.NewBuffer(requestData.RawBody))
			}
			next.ServeHTTP(w, r)
		}
	}
}

// bodyRejection returns the error response for a request body that the
// receiver rejected with err: 413 when it exceeds a body limit, else 400.
func bodyRejection(err error) Error {
	if bodyTooLarge(err) {
		return bodyTooLargeError()
	}
	return Error{
		Title:        "Invalid request content",
		Message:      "Request content empty json structure",
		ResponseCode: ResponseCodes.InvalidJSON,
	}
}

// bodyTooLarge reports whether err comes from a request body limit: the
// instance's, the default one, or one the service installed itself.
func bodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

// bodyTooLargeError is the 413 answered for a body over a limit.
func bodyTooLargeError() Error {
	return Error{
		Title:          "Request content too large",
		Message:        "Request content exceeds the size limit",
		ResponseCode:   ResponseCodes.InvalidJSON,
		HTTPStatusCode: http.StatusRequestEntityTooLarge,
	}
}

// SkipPath identifies a route that ProcessRequest passes through untouched,
// as "METHOD:path" (see GenSkipPath).
type SkipPath string

// SkipPaths holds the routes ProcessRequest skips: a request whose method and
// URL path match a key is not parsed and goes straight to the next handler.
// Only presence counts; the bool value is ignored.
//
// Only a request path sent exactly as Go's canonical encoding of its decoded
// path (an empty r.URL.RawPath) can match. Any other spelling is processed
// even when its decoded form is a key: an escaped slash (/health%2Flive,
// %2f), an escaped unreserved character (/health/%6Cive), and the raw
// characters ! ' ( ) * that Go escapes in a path (/a(b) is processed, while
// /a%28b%29 matches the key /a(b)). A router that matches the escaped path
// may route an escaped slash to another handler (GET /{tenant}), which must
// not lose its processing.
var SkipPaths map[SkipPath]bool

// AddSkipPath adds path to SkipPaths. ProcessRequest skips path whatever skip
// is, because only the key's presence is checked.
func AddSkipPath(path SkipPath, skip bool) {
	if SkipPaths == nil {
		SkipPaths = make(map[SkipPath]bool)
	}
	SkipPaths[path] = skip
}

// DeleteSkipPath removes path from SkipPaths, so ProcessRequest parses its
// requests again.
func DeleteSkipPath(path SkipPath) {
	if SkipPaths == nil {
		SkipPaths = make(map[SkipPath]bool)
	}
	delete(SkipPaths, path)
}

// GenSkipPath returns the SkipPath of method and path, as matched against the
// request method and decoded URL path (no query) of a request whose path has
// Go's canonical encoding (see SkipPaths).
func GenSkipPath(method string, path string) SkipPath {
	return SkipPath(fmt.Sprintf("%s:%s", method, path))
}

// skipsRequest reports whether r matches a SkipPaths key. A non-empty
// RawPath means the request path is not the canonical encoding of Path (an
// escaped slash, for one), so the decoded path may not be the path the router
// matched; such a request is never skipped.
func skipsRequest(r *http.Request) bool {
	if r.URL.RawPath != "" {
		return false
	}
	_, exists := SkipPaths[GenSkipPath(r.Method, r.URL.Path)]
	return exists
}

// ProcessRequest process request information.
//
// The proxiedIPAddress info value is the first X-Forwarded-For header line up
// to its first ", ", which the client sets; trust it only behind a proxy that
// overwrites the header.
//
// A multipart/* or application/octet-stream body is not read: next gets
// r.Body as it came, and the request context carries only the header
// metadata. Any other body is read whole into memory and parsed as JSON per
// the request mode before next runs, and next reads its content from r.Body.
// Either way the body is bounded by WithMaxBodyBytes under an instance's
// Handler (10 MiB by default) and at 10 MiB without one; ProcessRequest
// answers 413 for a read body over the limit, while reading a larger
// passed-through body returns an *http.MaxBytesError to next.
//
// next writes straight to w, so its response streams, and Flush and
// http.ResponseController reach the connection. A panic in next before it
// writes a status or body is answered with 500; after that the response can
// no longer be replaced, so the panic is logged and re-panicked as
// http.ErrAbortHandler, which makes net/http abort the response.
func ProcessRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		a := instanceFrom(r)

		if skipsRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		boundDefaultBody(w, r)
		passThrough := passesBodyThrough(r)
		rctx, err := processRequestContent(r, passThrough)
		if err != nil {
			a.logBodyError(r, "request body rejected", err, false)
			bodyRejection(err).Write(w, r)
			return
		}

		proxiedIPAddress := r.Header.Get("X-Forwarded-For")
		if proxiedIPAddress != "" {
			ips := strings.Split(proxiedIPAddress, ", ")
			proxiedIPAddress = ips[0]
		} else {
			proxiedIPAddress = r.RemoteAddr
		}

		prefixEventID := rctx.UUID
		if prefixEventID == "" {
			prefixEventID = proxiedIPAddress
		}

		rctx.EventID = generateEventID(r.Context(), prefixEventID, r.RequestURI)
		rctx.AddInfo("proxiedIPAddress", proxiedIPAddress)

		a.logRequest(r, rctx.EventID)

		r = UpdateRequestContext(rctx, r)
		rctx.Context = r.Context()
		r = r.WithContext(rctx)

		r.Header.Set(EventIDHeaderKey, rctx.EventID)
		r.Header.Set("UUID", rctx.UUID)
		r.Header.Set("DeviceType", rctx.DeviceType)
		r.Header.Set("DeviceBrand", rctx.DeviceBrand)
		r.Header.Set("DeviceModel", rctx.DeviceModel)
		r.Header.Set("DeviceOS", rctx.DeviceOS)
		r.Header.Set("OSVersion", rctx.OSVersion)
		r.Header.Set("OSTimezone", rctx.OSTimezone)
		r.Header.Set("AppLanguage", rctx.AppLanguage)
		r.Header.Set("AppVersion", rctx.AppVersion)
		r.Header.Set("AppBuildVersion", rctx.AppBuildInfo)
		r.Header.Set("AppName", rctx.AppName)
		r.Header.Set(SecurityTokenHeaderKey, rctx.SecurityToken)

		if !passThrough {
			r.Body = io.NopCloser(bytes.NewBuffer(rctx.Content))
		}
		sw := &statusWriter{ResponseWriter: w}
		// The headers outer middlewares set, so an early panic's 500 drops
		// the ones next added for its own body (a stale Content-Length).
		outerHeaders := w.Header().Clone()

		// net/http removes multipart temp files only for the request it
		// created, not for the copy next parsed.
		defer removeMultipartFiles(a, r)
		defer func() {
			if recvr := recover(); recvr != nil {
				a.logPanic(r, rctx.EventID, recvr)
				// The plain sentinel is re-panicked even for a wrapped abort:
				// net/http logs any other value, unredacted, with a stack. A
				// started response cannot become a 500, so it is aborted too.
				if isAbortPanic(recvr) || sw.started {
					panic(http.ErrAbortHandler)
				}
				header := w.Header()
				clear(header)
				maps.Copy(header, outerHeaders)
				Error500().Write(w, r)
				return
			}
		}()

		next.ServeHTTP(sw, r)

		a.logResponse(r, rctx.EventID, sw.loggedStatus(), time.Since(start))
	}
}

// removeMultipartFiles removes the temp files of a multipart form that the
// handler parsed from r, logging a failure on a's logger.
func removeMultipartFiles(a *API, r *http.Request) {
	if r.MultipartForm == nil {
		return
	}
	if err := r.MultipartForm.RemoveAll(); err != nil {
		a.logBodyError(r, "multipart form cleanup failed", err, false)
	}
}

// passesBodyThrough reports whether ProcessRequest leaves r's body unread:
// it has a body whose Content-Type is multipart/* or
// application/octet-stream, the uploads ProcessRequest could not serve
// before v1.2.0. Every other type, a missing or unparsable one included,
// keeps the JSON path, so a client that labels JSON as a form or as text is
// served as before.
func passesBodyThrough(r *http.Request) bool {
	if !requestHasBody(r) {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	return strings.HasPrefix(mediaType, "multipart/") || mediaType == "application/octet-stream"
}

// processRequestContent runs the instance's receiver on r. For a body passed
// through, the receiver gets a copy of r without a body, so it still fills
// the header metadata while r.Body stays unread for the handler.
func processRequestContent(r *http.Request, passThrough bool) (*RequestContext, error) {
	if !passThrough {
		return processBody(r)
	}
	headersOnly := r.WithContext(r.Context())
	headersOnly.Body = http.NoBody
	headersOnly.ContentLength = 0
	return processBody(headersOnly)
}

func requestHasBody(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.ContentLength > 0 || len(r.TransferEncoding) > 0 {
		return true
	}
	return r.Body != nil && r.Body != http.NoBody
}
