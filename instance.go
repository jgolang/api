package api

import (
	"context"
	"net/http"

	"github.com/jgolang/api/core"
)

// API holds one service's api configuration. Create it with New; a zero API
// is served as the default instance. It is immutable after New and safe for
// concurrent use. Its modes come only from its options: it never reads
// CurrentRequestMode or CurrentResponseMode.
type API struct {
	requestMode  RequestMode
	responseMode ResponseMode
	core         *core.API
}

// Option configures an API in New.
type Option func(*API)

// New returns an API configured by opts. Without options it parses plain
// requests and writes envelope responses, the v0 defaults. It uses the
// built-in request receiver, response formatter and response writer:
// RegisterNew* affect only the default instance (per-instance options come
// later).
func New(opts ...Option) *API {
	a := &API{
		requestMode:  RequestModePlain,
		responseMode: ResponseModeEnvelope,
	}
	for _, opt := range opts {
		opt(a)
	}
	// Guarantor, map methods and param validator stay package-level (ADR 0001).
	a.core = core.New(
		RequestReceiver{mode: a.requestMode},
		ResponseFormatter{mode: a.responseMode},
		ResponseWriter{},
		&SecurityGuaranter{},
		&mapMethods,
		ParamValidatorV0,
	)
	return a
}

// WithRequestMode sets how the instance parses request bodies. An unknown
// mode falls back to RequestModePlain.
func WithRequestMode(mode RequestMode) Option {
	return func(a *API) {
		a.requestMode = normalizeRequestMode(mode)
	}
}

// WithResponseMode sets how the instance formats responses. An unknown mode
// falls back to ResponseModeEnvelope.
func WithResponseMode(mode ResponseMode) Option {
	return func(a *API) {
		a.responseMode = normalizeResponseMode(mode)
	}
}

// Handler makes next answer with this instance: responses, WriteError,
// ProcessBody, ProcessEncryptedBody, ProcessRequest, RequestBody and the
// token middlewares called while serving a request use the instance's
// configuration instead of the package-level one. Implementations set with
// RegisterNew* affect only the default instance; a is served with the
// built-in ones.
func (a *API) Handler(next http.Handler) http.Handler {
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
// default instance when there is none or it was not built by New.
func instanceFrom(r *http.Request) *API {
	if r == nil {
		return defaultInstance
	}
	if a, ok := r.Context().Value(instanceKey{}).(*API); ok && a != nil && a.core != nil {
		return a
	}
	return defaultInstance
}
