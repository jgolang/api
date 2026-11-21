package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jgolang/api/core"
	"github.com/jgolang/errors/codes"
)

// API holds one service's api configuration. Create it with New. It is
// immutable after New and safe for concurrent use. The built-in receivers
// and formatter of an API from New never read CurrentRequestMode or
// CurrentResponseMode; their modes come only from its options. A custom
// component that embeds RequestReceiver, RequestReceiverV2 or
// ResponseFormatter is used as given, so its promoted methods read the
// package mode variables. WriteError answers with the instance's
// WithKindStatus overrides and calls its WithErrorHook.
//
// Requests that carry no instance are served by the default instance, which
// behaves like New() with its defaults: codes as declared, no request token
// echo, request bodies bounded at 10 MiB where the library reads them, and
// slog records instead of Print and PrintError. It differs from New() only
// in where its configuration comes from: it logs through slog.Default (read
// at call time), masks the default sensitive keys plus SensitiveLogKeys, and
// takes its modes, formatter settings and components from the deprecated
// package-level setters. A zero or nil API is the default instance.
type API struct {
	core       *core.API
	kindStatus map[codes.Kind]int
	errorHook  func(r *http.Request, err error, status int)
	// logger is nil only on the default instance, which logs through
	// slog.Default; New always sets one.
	logger *slog.Logger
	// sensitiveLogKeys is nil only on the default instance, which masks the
	// default keys plus SensitiveLogKeys.
	sensitiveLogKeys []string
	maxBodyBytes     int64
}

// options collects what the Option values passed to New set. New owns it, so
// an Option cannot reach an API after New returns.
type options struct {
	requestMode      RequestMode
	responseMode     ResponseMode
	lowercaseCodes   bool
	receiver         core.APIRequestReceiver
	formatter        core.APIResponseFormatter
	writer           core.APIResponseWriter
	kindStatus       map[codes.Kind]int
	errorHook        func(r *http.Request, err error, status int)
	logger           *slog.Logger
	sensitiveLogKeys []string
	maxBodyBytes     int64
}

// Option configures an API in New. New skips a nil Option.
type Option func(*options)

// New returns an API configured by opts. Without options it parses plain
// requests, writes envelope responses and uses the built-in request
// receiver, response formatter and response writer, the v0 defaults. Its
// built-in formatter sends response codes as declared (PLN-001 stays
// PLN-001); WithLowercaseCodes restores the v0 lowercasing. The security
// guarantor and param validator are shared with the default instance:
// RegisterNewAPISecurityGuarantor and RegisterParamValidator apply to every
// instance. WriteError has no kind overrides and no error hook unless
// WithKindStatus or WithErrorHook set them. It writes no logs unless
// WithLogger sets a logger, and its built-in response writer never exits; a
// custom writer owns its behavior. Its Handler bounds request bodies at
// 10 MiB unless WithMaxBodyBytes sets another limit.
func New(opts ...Option) *API {
	o := options{
		requestMode:  RequestModePlain,
		responseMode: ResponseModeEnvelope,
		maxBodyBytes: defaultMaxBodyBytes,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.logger == nil {
		o.logger = slog.New(slog.DiscardHandler)
	}
	formatter := o.responseFormatter()
	return &API{
		core: core.New(
			o.requestReceiver(),
			formatter,
			o.responseWriter(formatter),
			sharedSecurityGuarantor{},
			&mapMethods,
			sharedParamValidator,
		),
		kindStatus:       o.kindStatus, // built by WithKindStatus inside New; nothing else holds it
		errorHook:        o.errorHook,
		logger:           o.logger,
		sensitiveLogKeys: instanceSensitiveLogKeys(o.sensitiveLogKeys),
		maxBodyBytes:     o.maxBodyBytes,
	}
}

// defaultMaxBodyBytes is the request body limit of an API from New without
// WithMaxBodyBytes, and of the default instance.
const defaultMaxBodyBytes = 10 << 20

// requestReceiver returns the receiver New uses. A built-in receiver without
// a mode takes the instance's, so the instance never reads the package mode
// variable; a custom receiver is used as is. A nil one, including a nil
// pointer to a built-in type, means the built-in RequestReceiver.
func (o options) requestReceiver() core.APIRequestReceiver {
	switch receiver := o.receiver.(type) {
	case nil:
	case RequestReceiver:
		return receiver.withMode(o.requestMode)
	case *RequestReceiver:
		if receiver != nil {
			return receiver.withMode(o.requestMode)
		}
	case RequestReceiverV2:
		return receiver.withMode(o.requestMode)
	case *RequestReceiverV2:
		if receiver != nil {
			return receiver.withMode(o.requestMode)
		}
	default:
		return receiver
	}
	return RequestReceiver{mode: o.requestMode}
}

// responseFormatter returns the formatter New uses, bound to the instance
// mode in the same way as requestReceiver. A built-in formatter also takes
// the instance's code case; a custom one is used as is.
func (o options) responseFormatter() core.APIResponseFormatter {
	switch formatter := o.formatter.(type) {
	case nil:
	case ResponseFormatter:
		return o.bindFormatter(formatter)
	case *ResponseFormatter:
		if formatter != nil {
			return o.bindFormatter(*formatter)
		}
	default:
		return formatter
	}
	return o.bindFormatter(ResponseFormatter{})
}

// bindFormatter binds formatter to the instance's response mode and code
// case, and to the v0 defaults of TraceVisibility, TraceIDLength and
// BlankSuccess (ADR 0001: instances do not read those variables). Callers
// cannot set these fields, so they are always the instance's.
func (o options) bindFormatter(formatter ResponseFormatter) ResponseFormatter {
	formatter = formatter.withMode(o.responseMode)
	formatter.lowercaseCodes = o.lowercaseCodes
	formatter.v0Settings = true
	return formatter
}

// responseWriter returns the writer New uses. The built-in ResponseWriter,
// as a value or pointer, or nil, is bound to the instance's logger and
// formatter, so it logs through the instance logger; a custom writer is used
// as is.
func (o options) responseWriter(formatter core.APIResponseFormatter) core.APIResponseWriter {
	switch o.writer.(type) {
	case nil, ResponseWriter, *ResponseWriter:
		return ResponseWriter{logger: o.logger, formatter: formatter}
	default:
		return o.writer
	}
}

// WithRequestMode sets how the instance's built-in request receiver parses
// request bodies. An unknown mode falls back to RequestModePlain. A custom
// receiver set with WithRequestReceiver alone decides how it parses.
func WithRequestMode(mode RequestMode) Option {
	return func(o *options) {
		o.requestMode = normalizeRequestMode(mode)
	}
}

// WithResponseMode sets how the instance's built-in response formatter
// formats responses. An unknown mode falls back to ResponseModeEnvelope. With
// ResponseModeProblem, error responses are RFC 9457 problem details sent as
// application/problem+json, unless Headers sets a Content-Type, and other
// responses are their content alone. A custom formatter set with
// WithResponseFormatter alone decides the body.
func WithResponseMode(mode ResponseMode) Option {
	return func(o *options) {
		o.responseMode = normalizeResponseMode(mode)
	}
}

// WithLowercaseCodes makes the instance's built-in response formatter
// lowercase response codes in the code field and the message trace suffix,
// as v0 does. Without it, an instance from New sends codes as declared
// (PLN-001 stays PLN-001). A custom formatter set with WithResponseFormatter
// alone decides the code, and this option does not apply to it. The default
// instance sends codes as declared too.
func WithLowercaseCodes() Option {
	return func(o *options) {
		o.lowercaseCodes = true
	}
}

// WithKindStatus makes WriteError answer errors whose code has kind with
// status instead of StatusForKind(kind). Repeat it to override more kinds;
// a later valid call for the same kind wins, and kinds without an override
// keep StatusForKind. Only error statuses (400 to 599) are accepted: any
// other status is ignored, so an error never answers 2xx or 3xx. The empty
// kind is ignored too, so errors without a kind, and nil errors, always
// answer 500. An ignored call leaves any earlier override in place. The
// response title follows the status as in WriteError: the internal title
// for any 5xx, the unauthorized title for 401 and 403.
func WithKindStatus(kind codes.Kind, status int) Option {
	return func(o *options) {
		if kind == "" || status < http.StatusBadRequest || status > maxErrorStatus {
			return
		}
		if o.kindStatus == nil {
			o.kindStatus = make(map[codes.Kind]int)
		}
		o.kindStatus[kind] = status
	}
}

// maxErrorStatus is the highest status WithKindStatus accepts.
const maxErrorStatus = 599

// WithErrorHook sets fn to observe every error the instance's WriteError
// writes, for example to log or count it. fn receives the request, the
// error exactly as passed to WriteError (nil included) and the status
// WriteError decided. That status may differ from the one sent when the
// handler already wrote a header or a custom formatter or writer changes it.
// It runs synchronously on the request goroutine, after the status is
// decided and before anything is written. Only WriteError calls fn: the
// other error helpers (Error400, Error500, ...) and the library's own
// rejections (body-parse failures in RequestBody and ProcessRequest, the
// token middlewares' 401s, ProcessRequest's panic 500) do not. A panic in fn
// is not recovered: it propagates out of WriteError with nothing written.
// Under ProcessRequest it answers 500 and is logged, with the panic type,
// through the instance's WithLogger logger; nothing is logged without one.
// fn is shared by all
// requests the instance serves and may run concurrently, so it must be safe
// for concurrent use. A later WithErrorHook replaces an earlier one, and nil
// means no hook.
func WithErrorHook(fn func(r *http.Request, err error, status int)) Option {
	return func(o *options) {
		o.errorHook = fn
	}
}

// WithLogger makes the instance log through logger: ProcessRequest writes a
// "request" and a "response" info record per request, and errors met while
// serving one (a rejected body, a failed Get*Value* helper, a recovered
// panic, a response that cannot be encoded) are written as error records.
// Request and response bodies and the parsed form are never logged. Header
// and query values whose key is sensitive (see WithSensitiveLogKeys) are
// masked, as is the user information of an absolute-form URI, and header
// values that carry a URL are masked as the URI is; a failed
// Get*Value* helper logs the name as "key" and, for a sensitive name, only
// the kind of error, never the value. A recovered panic logs its Go type as
// "panic_type" and its value only for a runtime.Error. A panic with
// http.ErrAbortHandler is a "handler aborted" debug record instead of an
// error record.
// The built-in writer's records carry no request context or event_id, since
// core.APIResponseWriter.Write receives no request.
// Without it, or with nil, the instance writes no logs. A later WithLogger
// replaces an earlier one.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

// WithSensitiveLogKeys adds keys to the ones whose header and query values
// the instance masks in its records, and whose Get*Value* helper errors are
// logged without the value. A key matches any name that contains it,
// ignoring case. Secrets in path segments (/reset/{token}) are not masked.
// The defaults (the shipped SensitiveLogKeys list, which includes cookie and
// session since v0.11.0; changes to the variable apply only to the default
// instance) are always kept: this option only adds. Keys are trimmed and
// lowercased, blank keys are ignored; repeat it to add more.
func WithSensitiveLogKeys(keys ...string) Option {
	return func(o *options) {
		for _, key := range keys {
			key = strings.ToLower(strings.TrimSpace(key))
			if key == "" {
				continue
			}
			o.sensitiveLogKeys = append(o.sensitiveLogKeys, key)
		}
	}
}

// WithMaxBodyBytes sets the most bytes of a request body the instance's
// Handler lets anything under it read; the default is 10 MiB. When
// ProcessRequest, RequestBody or UnmarshalBody meet a larger body, they
// answer 413 Request Entity Too Large with the response code
// ResponseCodes.InvalidJSON (E002) and the title "Request content too
// large". A custom receiver, or a handler that reads r.Body itself, gets the
// *http.MaxBytesError and decides how to answer. A value n <= 0 is ignored
// and the earlier value or the default stays, so a zero configuration value
// never removes the bound; pass math.MaxInt64 for no practical limit. A
// later valid call replaces an earlier one. With nested instances each
// Handler adds its bound, so the smallest limit applies. The default instance
// bounds bodies at 10 MiB where the library reads them (ProcessRequest,
// RequestBody, ProcessBody, ProcessEncryptedBody, UnmarshalBody); a handler
// that reads r.Body itself outside an instance is not bounded.
func WithMaxBodyBytes(n int64) Option {
	return func(o *options) {
		if n <= 0 {
			return
		}
		o.maxBodyBytes = n
	}
}

// WithRequestReceiver sets the receiver that parses the instance's request
// bodies. RequestReceiver and RequestReceiverV2 parse with the instance's
// request mode; a custom receiver alone decides how it parses, and
// WithRequestMode does not apply to it. A type that embeds RequestReceiver
// or RequestReceiverV2 is custom: its promoted methods read
// CurrentRequestMode. A nil interface value keeps the built-in; a typed nil
// pointer to a custom receiver is used as given.
func WithRequestReceiver(receiver core.APIRequestReceiver) Option {
	return func(o *options) {
		o.receiver = receiver
	}
}

// WithResponseFormatter sets the formatter that builds the instance's
// response bodies. ResponseFormatter formats with the instance's response
// mode and code case; a custom formatter alone decides the body and its code
// text, and WithResponseMode and WithLowercaseCodes do not apply to it. A
// type that embeds ResponseFormatter is custom: its promoted methods read
// CurrentResponseMode and send codes as declared. A nil interface value
// keeps the built-in; a typed nil pointer to a custom formatter is used as
// given.
func WithResponseFormatter(formatter core.APIResponseFormatter) Option {
	return func(o *options) {
		o.formatter = formatter
	}
}

// WithResponseWriter sets the writer that sends the instance's formatted
// responses. The built-in ResponseWriter, as a value, a pointer or a nil
// interface value, is bound to the instance's logger and formatter: it logs a
// body that cannot be encoded through the instance logger and answers 500
// with the instance formatter's body. A custom writer alone decides how it
// writes and is used as given, including a typed nil pointer. A type that
// embeds ResponseWriter is custom: its promoted Write keeps the default
// instance's behavior (slog.Default and the registered formatter).
func WithResponseWriter(writer core.APIResponseWriter) Option {
	return func(o *options) {
		o.writer = writer
	}
}

// sharedSecurityGuarantor forwards to the default instance's guarantor, so
// RegisterNewAPISecurityGuarantor applies to every instance, as ADR 0001
// keeps it package-level.
type sharedSecurityGuarantor struct{}

func (sharedSecurityGuarantor) ValidateBasicToken(token string) (client, secret string, valid bool) {
	return api.ValidateBasicToken(token)
}

func (sharedSecurityGuarantor) ValidateCustomToken(token string, validator core.CustomTokenValidator) (json.RawMessage, bool) {
	return api.ValidateCustomToken(token, validator)
}

// sharedParamValidator forwards to the default instance's param validator,
// so RegisterParamValidator applies to every instance.
func sharedParamValidator(v any) (errMsg string, err error) {
	return api.ValidateParams(v)
}

// Handler makes next answer with this instance: responses, WriteError,
// ProcessBody, ProcessEncryptedBody, ProcessRequest, RequestBody and the
// token middlewares called while serving a request use the instance's
// configuration instead of the package-level one. The receiver, formatter
// and writer set with RegisterNewAPIRequestReceiver,
// RegisterNewAPIResponseFormatter and RegisterNewAPIResponseWriter affect
// only the default instance; a uses its own.
//
// Wrap the outermost chain (usually the router). When instances are nested,
// the innermost one wins. ProcessRequest and the other middlewares placed
// outside a.Handler run with the default instance, so they parse with the
// package-level request mode. Handler bounds the request body with
// http.MaxBytesReader at the instance's limit (see WithMaxBodyBytes); nested
// limits combine, so the smallest applies. On a zero or nil API, Handler
// returns next unchanged.
func (a *API) Handler(next http.Handler) http.Handler {
	if a == nil || a.core == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), instanceKey{}, a))
		// A body-less request keeps http.NoBody, so it costs no allocation.
		if r.Body != nil && r.Body != http.NoBody {
			r.Body = http.MaxBytesReader(w, r.Body, a.maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

type instanceKey struct{}

// defaultInstance serves requests that carry no instance. It wraps the
// package-level core, so RegisterNew* and the package mode variables
// configure it; its nil logger and sensitive keys are resolved at call time.
var defaultInstance = &API{core: api, maxBodyBytes: defaultMaxBodyBytes}

// instanceFrom returns the instance stored in r's context by Handler, or the
// default instance when there is none.
func instanceFrom(r *http.Request) *API {
	if r == nil {
		return defaultInstance
	}
	if a, ok := r.Context().Value(instanceKey{}).(*API); ok {
		return a
	}
	return defaultInstance
}

// boundDefaultBody bounds r's body at the default limit when r is served by
// the default instance, which has no Handler to do it. An instance's Handler
// already bounded the body, so its requests are left alone. w may be nil for
// callers that have no response writer.
func boundDefaultBody(w http.ResponseWriter, r *http.Request) {
	if r == nil || r.Body == nil || r.Body == http.NoBody || instanceFrom(r) != defaultInstance {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodyBytes)
}
