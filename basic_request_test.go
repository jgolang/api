package api

import (
	"net/http"
	"testing"
)

// requestInfoGetters call each RequestBasic header getter and write the
// response it returns.
var requestInfoGetters = map[string]func(*RequestBasic) Response{
	"GetSecurityToken":    (*RequestBasic).GetSecurityToken,
	"GetUserID":           (*RequestBasic).GetUserID,
	"GetEventID":          (*RequestBasic).GetEventID,
	"GetRequestBasicInfo": (*RequestBasic).GetRequestBasicInfo,
	"GetRequestFullInfo":  (*RequestBasic).GetRequestFullInfo,
}

// requestInfoHandler answers with the response getter returns for a request
// without any of the RequestBasic headers.
func requestInfoHandler(getter func(*RequestBasic) Response) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := getter(&RequestBasic{HTTPReq: r})
		if response == nil {
			Success200().Write(w, r)
			return
		}
		response.Write(w, r)
	}
}

func TestRequestBasicGettersSendTheRequestInfoError(t *testing.T) {
	wantMessages := map[string]string{
		"GetSecurityToken":    "The session id was not obtained",
		"GetUserID":           "The user id was not obtained",
		"GetEventID":          "The event id was not obtained",
		"GetRequestBasicInfo": "The session id was not obtained",
		"GetRequestFullInfo":  "The session id was not obtained",
	}
	// The message ends with the trace suffix; without an event ID it holds
	// only the code, as declared on both.
	instances := map[string]struct {
		wrap        func(http.Handler) http.Handler
		traceSuffix string
	}{
		"default instance": {func(next http.Handler) http.Handler { return next }, " (-E001)"},
		"instance":         {New().Handler, " (-E001)"},
	}
	for instanceName, instance := range instances {
		for getterName, getter := range requestInfoGetters {
			t.Run(instanceName+"/"+getterName, func(t *testing.T) {
				rec := serve(instance.wrap(requestInfoHandler(getter)), http.MethodGet, "", nil)

				header, _ := decodeMembers(t, rec.Body.Bytes())["header"].(map[string]any)
				if header["title"] != "Request info error!" {
					t.Errorf("title = %v, want %q", header["title"], "Request info error!")
				}
				wantMessage := wantMessages[getterName] + instance.traceSuffix
				if header["message"] != wantMessage {
					t.Errorf("message = %v, want %q", header["message"], wantMessage)
				}
				if rec.Code != http.StatusBadRequest {
					t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
				}
			})
		}
	}
}

func TestRequestBasicInfoStoresTheHeaderValues(t *testing.T) {
	headers := map[string]string{
		SecurityTokenHeaderKey: "session-1",
		UserIDHeaderKey:        "user-1",
		EventIDHeaderKey:       "event-1",
	}
	var request *RequestBasic
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request = &RequestBasic{HTTPReq: r}
		if response := request.GetRequestBasicInfo(); response != nil {
			t.Fatalf("GetRequestBasicInfo() = %v, want nil", response)
		}
	})

	serve(handler, http.MethodGet, "", headers)

	if request.SessionID != "session-1" || request.UserID != "user-1" || request.EventID != "event-1" {
		t.Fatalf("got %q, %q, %q; want session-1, user-1, event-1", request.SessionID, request.UserID, request.EventID)
	}
}
