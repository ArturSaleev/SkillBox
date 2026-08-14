package observability

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/aibox/skillbox/internal/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type requestIDKey struct{}

func RequestID(ctx context.Context) string { v, _ := ctx.Value(requestIDKey{}).(string); return v }

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func HTTP(logger *slog.Logger, m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = uuid.NewString()
			}
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			rec := &statusRecorder{ResponseWriter: w, status: 200}
			start := time.Now()
			next.ServeHTTP(rec, r.WithContext(ctx))
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}
			m.Requests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
			logger.InfoContext(ctx, "http request", "request_id", id, "method", r.Method, "route", route, "status", rec.status, "duration_ms", time.Since(start).Milliseconds())
		})
	}
}
