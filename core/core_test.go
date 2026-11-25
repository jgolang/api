package core

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// routeVarReceiver is an APIRequestReceiver that only serves route vars.
type routeVarReceiver map[string]string

func (routeVarReceiver) ProcessBody(*http.Request) (*RequestDataContext, error) { return nil, nil }

func (routeVarReceiver) ProcessEncryptedBody(*http.Request) (*RequestEncryptedData, error) {
	return nil, nil
}

func (vars routeVarReceiver) GetRouteVar(key string, _ *http.Request) string { return vars[key] }

func newTestAPI() *API {
	vars := routeVarReceiver{"s": "abc", "i": "42", "f": "1.5", "b": "true", "bad": "x"}
	return New(vars, nil, nil, nil, &MapMethods{}, nil)
}

func newTestRequest() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/?s=abc&i=42&f=1.5&b=true&bad=x", nil)
	r.Header.Set("s", "abc")
	r.Header.Set("i", "42")
	r.Header.Set("f", "1.5")
	r.Header.Set("b", "true")
	r.Header.Set("bad", "x")
	return r
}

// typedGetters returns one getter per value type for a source (route var,
// header or query param), each returning its value as any.
func typedGetters(
	getString func(string, *http.Request) (string, error),
	getInt func(string, *http.Request) (int, error),
	getInt64 func(string, *http.Request) (int64, error),
	getFloat func(string, *http.Request) (float64, error),
	getBool func(string, *http.Request) (bool, error),
) map[string]func(string, *http.Request) (any, error) {
	return map[string]func(string, *http.Request) (any, error){
		"string":  func(k string, r *http.Request) (any, error) { return getString(k, r) },
		"int":     func(k string, r *http.Request) (any, error) { return getInt(k, r) },
		"int64":   func(k string, r *http.Request) (any, error) { return getInt64(k, r) },
		"float64": func(k string, r *http.Request) (any, error) { return getFloat(k, r) },
		"bool":    func(k string, r *http.Request) (any, error) { return getBool(k, r) },
	}
}

func TestValueGetters(t *testing.T) {
	api := newTestAPI()
	sources := map[string]map[string]func(string, *http.Request) (any, error){
		"route var": typedGetters(api.GetRouteVarValueString, api.GetRouteVarValueInt, api.GetRouteVarValueInt64,
			api.GetRouteVarValueFloat64, api.GetRouteVarValueBool),
		"header": typedGetters(api.GetHeaderValueString, api.GetHeaderValueInt, api.GetHeaderValueInt64,
			api.GetHeaderValueFloat64, api.GetHeaderValueBool),
		"query param": typedGetters(api.GetQueryParamValueString, api.GetQueryParamValueInt, api.GetQueryParamValueInt64,
			api.GetQueryParamValueFloat64, api.GetQueryParamValueBool),
	}
	tests := []struct {
		getter  string
		key     string
		want    any
		wantErr bool
	}{
		{getter: "string", key: "s", want: "abc"},
		{getter: "string", key: "missing", want: "", wantErr: true},
		{getter: "int", key: "i", want: 42},
		{getter: "int", key: "bad", wantErr: true},
		{getter: "int64", key: "i", want: int64(42)},
		{getter: "int64", key: "bad", wantErr: true},
		{getter: "float64", key: "f", want: 1.5},
		{getter: "float64", key: "bad", wantErr: true},
		{getter: "bool", key: "b", want: true},
		{getter: "bool", key: "bad", want: false, wantErr: true},
	}

	for sourceName, getters := range sources {
		for _, tt := range tests {
			t.Run(sourceName+"/"+tt.getter+"/"+tt.key, func(t *testing.T) {
				got, err := getters[tt.getter](tt.key, newTestRequest())

				if (err != nil) != tt.wantErr {
					t.Fatalf("error = %v, want error %v", err, tt.wantErr)
				}
				if !tt.wantErr && got != tt.want {
					t.Fatalf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
				}
			})
		}
	}
}

func TestValidateMethods(t *testing.T) {
	api := newTestAPI()
	api.AddMapMethod("tasks", []string{"get", "post"})

	tests := []struct {
		key    string
		method string
		want   bool
	}{
		{key: "tasks", method: "get", want: true},
		{key: "tasks", method: "post", want: true},
		{key: "tasks", method: "delete", want: false},
		{key: "unknown", method: "get", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.key+"/"+tt.method, func(t *testing.T) {
			if got := api.ValidateMethods(tt.key, tt.method); got != tt.want {
				t.Fatalf("ValidateMethods(%q, %q) = %v, want %v", tt.key, tt.method, got, tt.want)
			}
		})
	}
}

func TestMiddlewaresChainRunsInOrderAndWritesTheResponse(t *testing.T) {
	var order []string
	tag := func(name string) Middleware {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next(w, r)
			}
		}
	}
	final := func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "final")
		w.Header().Set("X-Test", "yes")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("body"))
	}
	rec := httptest.NewRecorder()

	MiddlewaresChain(tag("first"), tag("second"))(final)(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if want := []string{"first", "second", "final"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	if rec.Code != http.StatusCreated || rec.Header().Get("X-Test") != "yes" || rec.Body.String() != "body" {
		t.Fatalf("got status %d, header %q, body %q", rec.Code, rec.Header().Get("X-Test"), rec.Body.String())
	}
}

// markedWriter is a ResponseWriter of a type the chain cannot create, so a
// final handler that receives it got the writer passed in.
type markedWriter struct {
	*httptest.ResponseRecorder
}

func TestMiddlewaresChainPassesTheWriterThrough(t *testing.T) {
	passThrough := func(next http.HandlerFunc) http.HandlerFunc { return next }
	var got http.ResponseWriter
	final := func(w http.ResponseWriter, r *http.Request) { got = w }
	writer := markedWriter{httptest.NewRecorder()}

	MiddlewaresChain(passThrough, passThrough)(final)(writer, httptest.NewRequest(http.MethodGet, "/", nil))

	if got != http.ResponseWriter(writer) {
		t.Fatalf("final handler got writer %T, want the one passed in", got)
	}
}

func TestMiddlewaresChainWithoutMiddlewaresRunsTheFinalHandler(t *testing.T) {
	final := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("final"))
	}
	rec := httptest.NewRecorder()

	MiddlewaresChain()(final)(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusAccepted || rec.Body.String() != "final" {
		t.Fatalf("got status %d, body %q; want %d, %q", rec.Code, rec.Body.String(), http.StatusAccepted, "final")
	}
}

func TestRequestDataContextInfo(t *testing.T) {
	var data RequestDataContext
	data.Set("s", "abc")
	data.Set("i", 42)
	data.Set("i64", int64(64))
	data.Set("f", 1.5)
	data.Set("b", true)
	data.Set("struct", map[string]string{"name": "task"})
	data.AddInfo("added", "yes")
	data.AddHeader("X-Test", "yes")

	var decoded struct{ Name string }
	err := data.GetStruct("struct", &decoded)

	if err != nil || decoded.Name != "task" {
		t.Fatalf("GetStruct = %+v, %v", decoded, err)
	}
	if data.Get("s") != "abc" || data.GetString("s") != "abc" || data.GetString("added") != "yes" {
		t.Fatalf("string values: %v %q %q", data.Get("s"), data.GetString("s"), data.GetString("added"))
	}
	if data.GetInt("i") != 42 || data.GetInt64("i64") != 64 || data.GetFloat("f") != 1.5 || !data.GetBool("b") {
		t.Fatalf("typed values: %d %d %v %v", data.GetInt("i"), data.GetInt64("i64"), data.GetFloat("f"), data.GetBool("b"))
	}
	if data.Headers["X-Test"] != "yes" {
		t.Fatalf("headers = %v", data.Headers)
	}
}

func TestJSONContent(t *testing.T) {
	data := RequestDataContext{Content: JSONContent(`{"name":"task"}`)}
	var decoded struct{ Name string }

	err := data.DecodeContent(&decoded)

	if err != nil || decoded.Name != "task" {
		t.Fatalf("DecodeContent = %+v, %v", decoded, err)
	}
	if got, _ := JSONContent(nil).MarshalJSON(); string(got) != "null" {
		t.Fatalf("nil content marshals to %q, want null", got)
	}
	if err := data.DecodeContent(&[]int{}); err == nil || !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Fatalf("expected an unmarshal error, got %v", err)
	}
}
