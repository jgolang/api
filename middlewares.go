package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

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
func BasicToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(auth) != 2 || auth[0] != "Basic" {
			response := Error500()
			response.Message = DefaultInvalidAuthHeaderMsg
			response.Write(w, r)
			return
		}
		client, secret, tokenValid := ValidateBasicToken(auth[1])
		if !tokenValid {
			response := Error401()
			response.Message = DefaultBasicUnauthorizedMsg
			response.Write(w, r)
			return
		}
		r.Header.Set("Basic-Client", client)
		r.Header.Set("Basic-Secret", secret)
		next(w, r)
	}
}

// CustomToken middleware to validates custom token authorization method.
func CustomToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(auth) != 2 || auth[0] != CustomTokenPrefix {
			response := Error500()
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
		contentType := r.Header.Get("Content-Type")
		if len(contentType) == 0 {
			Error{
				Message: "No content-type!",
			}.Write(w, r)
			return
		}
		if contentType != "application/json" {
			Error{
				Message:      "Content-Type not is JSON!",
				ResponseCode: ResponseCodes.InvalidJSON,
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
var RequestBody = NewRequestBodyMiddleware(PPPGMethodsKey)

// NewRequestBodyMiddleware doc ...
func NewRequestBodyMiddleware(keyListMethods string) core.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if ValidateMethods(keyListMethods, r.Method) {
				requestData, err := api.ProcessBody(r)
				if err != nil {
					PrintError(err)
					Error{
						Title:        "Invalid request content",
						Message:      "Request content empty json structure",
						ResponseCode: ResponseCodes.InvalidJSON,
					}.Write(w, r)
					return
				}
				r.Body = io.NopCloser(bytes.NewBuffer(requestData.RawBody))
			}
			next.ServeHTTP(w, r)
		}
	}
}

// ProcessRequest process request information.
func ProcessRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestData, err := ProcessBody(r)
		if err != nil {
			PrintError(err)
			Error{
				Title:        "Invalid request content",
				Message:      "Request content empty json structure",
				ResponseCode: ResponseCodes.InvalidJSON,
			}.Write(w, r)
			return
		}

		proxiedIPAddress := r.Header.Get("X-Forwarded-For")
		if proxiedIPAddress != "" {
			ips := strings.Split(proxiedIPAddress, ", ")
			proxiedIPAddress = ips[0]
		} else {
			proxiedIPAddress = r.RemoteAddr
		}

		prefixEventID := requestData.UUID
		if prefixEventID == "" {
			prefixEventID = proxiedIPAddress
		}

		requestData.EventID = generateEventID(prefixEventID, r.RequestURI)
		requestData.AddInfo("proxiedIPAddress", proxiedIPAddress)

		LogRequest(r.Method, r.RequestURI, requestData.EventID, r.Form.Encode(), r.Header, requestData.RawBody)

		r = UpdateRequestContext(requestData, r)

		r.Header.Set(EventIDHeaderKey, requestData.EventID)
		r.Header.Set("UUID", requestData.UUID)
		r.Header.Set("DeviceType", requestData.DeviceType)
		r.Header.Set("DeviceBrand", requestData.DeviceBrand)
		r.Header.Set("DeviceModel", requestData.DeviceModel)
		r.Header.Set("DeviceOS", requestData.DeviceOS)
		r.Header.Set("OSVersion", requestData.OSVersion)
		r.Header.Set("OSTimezone", requestData.OSTimezone)
		r.Header.Set("AppLanguage", requestData.AppLanguage)
		r.Header.Set("AppVersion", requestData.AppVersion)
		r.Header.Set("AppBuildVersion", requestData.AppBuildInfo)
		r.Header.Set("AppName", requestData.AppName)
		r.Header.Set(SecurityTokenHeaderKey, requestData.SecurityToken)

		r.Body = io.NopCloser(bytes.NewBuffer(requestData.Content))
		rec := httptest.NewRecorder()

		defer func() {
			if recvr := recover(); recvr != nil {
				err, ok := recvr.(error)
				if ok {
					PrintError(err)
				} else {
					PrintError("Not response: ", fmt.Sprintf("%v", r))
				}
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
		LogResponse(requestData.EventID, rec)
	}
}
