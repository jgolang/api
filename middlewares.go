package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
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

// RequestHeaderSession validates that session ID is valid.
func RequestHeaderSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.Header.Get(SecurityTokenHeaderKey)
		if sessionID == "" {
			response := Error401()
			response.Message = "Invalid session ID"
			response.Write(w, r)
			return
		}
		w.Header().Set(SecurityTokenHeaderKey, sessionID)
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
				requestData, err := a.core.ProcessBody(r)
				if err != nil {
					a.logError(r, "request body rejected", err)
					bodyRejection(a, err).Write(w, r)
					return
				}
				r.Body = io.NopCloser(bytes.NewBuffer(requestData.RawBody))
			}
			next.ServeHTTP(w, r)
		}
	}
}

// bodyRejection returns the error response for a request body that a's
// receiver rejected with err: 413 when it exceeds an instance's body limit,
// else the v0 400.
func bodyRejection(a *API, err error) Error {
	if bodyTooLarge(a, err) {
		return bodyTooLargeError()
	}
	return Error{
		Title:        "Invalid request content",
		Message:      "Request content empty json structure",
		ResponseCode: ResponseCodes.InvalidJSON,
	}
}

// bodyTooLarge reports whether err comes from the body limit of an instance
// from New. The default instance keeps its v0 400 even for a limit the
// service installed itself.
func bodyTooLarge(a *API, err error) bool {
	var maxBytesErr *http.MaxBytesError
	return a.fromNew && errors.As(err, &maxBytesErr)
}

// bodyTooLargeError is the 413 an instance answers for a body over its limit.
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
// request method and URL path (no query).
func GenSkipPath(method string, path string) SkipPath {
	return SkipPath(fmt.Sprintf("%s:%s", method, path))
}

// ProcessRequest process request information.
//
// The proxiedIPAddress info value is the first X-Forwarded-For header line up
// to its first ", ", which the client sets; trust it only behind a proxy that
// overwrites the header.
// The whole request body is read into memory. Under an instance's Handler it
// is bounded by WithMaxBodyBytes (10 MiB by default) and a larger body is
// answered with 413; the default instance has no limit, so bound it before
// this middleware runs (http.MaxBytesHandler or http.MaxBytesReader).
func ProcessRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		a := instanceFrom(r)

		if _, exists := SkipPaths[GenSkipPath(r.Method, r.URL.Path)]; exists {
			next.ServeHTTP(w, r)
			return
		}

		rctx, err := ProcessBody(r)
		if err != nil {
			a.logError(r, "request body rejected", err)
			bodyRejection(a, err).Write(w, r)
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

		a.logRequest(r, rctx.EventID, rctx.RawBody)

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

		r.Body = io.NopCloser(bytes.NewBuffer(rctx.Content))
		rec := httptest.NewRecorder()

		defer func() {
			if recvr := recover(); recvr != nil {
				a.logPanic(r, rctx.EventID, recvr)
				Error500().Write(w, r)
				return
			}
		}()

		next.ServeHTTP(rec, r)

		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		w.Write(rec.Body.Bytes())
		a.logResponse(r, rctx.EventID, rec, time.Since(start))
	}
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
