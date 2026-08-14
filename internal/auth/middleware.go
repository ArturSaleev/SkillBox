package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return fmt.Sprintf("sha256:%x", sum[:])
}

type Middleware struct {
	mode string
	keys []string
}

func New(mode string, keys []string) *Middleware { return &Middleware{mode: mode, keys: keys} }
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	if m.mode == "disabled" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get("X-API-Key"))
		if key == "" {
			if h := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
				key = strings.TrimSpace(h[7:])
			}
		}
		for _, candidate := range m.keys {
			if len(key) == len(candidate) && subtle.ConstantTimeCompare([]byte(key), []byte(candidate)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "unauthorized", "message": "valid API key is required"}})
	})
}
