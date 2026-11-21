package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type contextTestKey string

func TestContextFindsRequestDataInDerivedContexts(t *testing.T) {
	var gotEventID, wantEventID string
	tests := []struct {
		name   string
		derive func(context.Context) (context.Context, context.CancelFunc)
	}{
		{name: "request context", derive: func(ctx context.Context) (context.Context, context.CancelFunc) { return ctx, func() {} }},
		{name: "WithValue", derive: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithValue(ctx, contextTestKey("k"), "v"), func() {}
		}},
		{name: "nested WithValue", derive: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithValue(context.WithValue(ctx, contextTestKey("a"), 1), contextTestKey("b"), 2), func() {}
		}},
		{name: "WithTimeout", derive: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithTimeout(ctx, time.Minute)
		}},
		{name: "WithCancel", derive: context.WithCancel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
				wantEventID = Context(r.Context()).EventID
				ctx, cancel := tt.derive(r.Context())
				defer cancel()
				gotEventID = Context(ctx).EventID
				Success200().Write(w, r)
			})
			res := httptest.NewRecorder()

			handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/tasks", nil))

			if res.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", res.Code)
			}
			if wantEventID == "" || gotEventID != wantEventID {
				t.Fatalf("expected event ID %q, got %q", wantEventID, gotEventID)
			}
		})
	}
}

func TestContextWithoutRequestDataWrapsContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{name: "Background", ctx: context.Background()},
		{name: "WithValue", ctx: context.WithValue(context.Background(), contextTestKey("k"), "v")},
		{name: "WithTimeout", ctx: func() context.Context {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			t.Cleanup(cancel)
			return ctx
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Context(tt.ctx)

			if got == nil || got.RequestDataContext == nil {
				t.Fatal("expected a request context")
			}
			if got.Context != tt.ctx {
				t.Fatal("expected the request context to wrap the given context")
			}
		})
	}
}
