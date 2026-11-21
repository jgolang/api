package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jgolang/api/core"
)

// API holds one service's api configuration. Create it with New. It is
// immutable after New and safe for concurrent use. The built-in receivers
// and formatter of an API from New never read CurrentRequestMode or
// CurrentResponseMode; their modes come only from its options. A custom
// component that embeds RequestReceiver, RequestReceiverV2 or
// ResponseFormatter is used as given, so its promoted methods read the
// package mode variables. A zero or nil API is the default instance.
type API struct {
	core *core.API
}

// options collects what the Option values passed to New set. New owns it, so
// an Option cannot reach an API after New returns.
type options struct {
	requestMode  RequestMode
	responseMode ResponseMode
	receiver     core.APIRequestReceiver
	formatter    core.APIResponseFormatter
	writer       core.APIResponseWriter
}

// Option configures an API in New. New skips a nil Option.
type Option func(*options)

// New returns an API configured by opts. Without options it parses plain
// requests, writes envelope responses and uses the built-in request
// receiver, response formatter and response writer, the v0 defaults. The
// security guarantor and param validator are shared with the default
// instance: RegisterNewAPISecurityGuarantor and RegisterParamValidator apply
// to every instance.
func New(opts ...Option) *API {
	o := options{
		requestMode:  RequestModePlain,
		responseMode: ResponseModeEnvelope,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return &API{core: core.New(
		o.requestReceiver(),
		o.responseFormatter(),
		o.responseWriter(),
		sharedSecurityGuarantor{},
		&mapMethods,
		sharedParamValidator,
	)}
}

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
// mode in the same way as requestReceiver.
func (o options) responseFormatter() core.APIResponseFormatter {
	switch formatter := o.formatter.(type) {
	case nil:
	case ResponseFormatter:
		return formatter.withMode(o.responseMode)
	case *ResponseFormatter:
		if formatter != nil {
			return formatter.withMode(o.responseMode)
		}
	default:
		return formatter
	}
	return ResponseFormatter{mode: o.responseMode}
}

func (o options) responseWriter() core.APIResponseWriter {
	if o.writer == nil {
		return ResponseWriter{}
	}
	return o.writer
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
// formats responses. An unknown mode falls back to ResponseModeEnvelope. A
// custom formatter set with WithResponseFormatter alone decides the body.
func WithResponseMode(mode ResponseMode) Option {
	return func(o *options) {
		o.responseMode = normalizeResponseMode(mode)
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
// mode; a custom formatter alone decides the body, and WithResponseMode does
// not apply to it. A type that embeds ResponseFormatter is custom: its
// promoted methods read CurrentResponseMode. A nil interface value keeps the
// built-in; a typed nil pointer to a custom formatter is used as given.
func WithResponseFormatter(formatter core.APIResponseFormatter) Option {
	return func(o *options) {
		o.formatter = formatter
	}
}

// WithResponseWriter sets the writer that sends the instance's formatted
// responses. A nil interface value keeps the built-in ResponseWriter; a
// typed nil pointer to a custom writer is used as given.
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
// package-level request mode. On a zero or nil API, Handler returns next
// unchanged.
func (a *API) Handler(next http.Handler) http.Handler {
	if a == nil || a.core == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), instanceKey{}, a)))
	})
}

type instanceKey struct{}

// defaultInstance serves requests that carry no instance. It wraps the
// package-level core, so RegisterNew* and the package mode variables keep
// their v0 behavior.
var defaultInstance = &API{core: api}

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
