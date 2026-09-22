package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/aibox/skillbox/benchmark/internal/benchmark"
	"github.com/aibox/skillbox/benchmark/internal/chat"
	benchmarkconfig "github.com/aibox/skillbox/benchmark/internal/config"
	"github.com/aibox/skillbox/benchmark/internal/mcp"
	"github.com/aibox/skillbox/benchmark/internal/provider"
	"github.com/aibox/skillbox/benchmark/internal/storage"
	"github.com/go-chi/chi/v5"
)

type API struct {
	config *benchmarkconfig.Manager
	store  *storage.Store
	chat   *chat.Service
	bench  *benchmark.Service
}

func Handler(manager *benchmarkconfig.Manager, store *storage.Store) http.Handler {
	api := &API{config: manager, store: store, chat: chat.NewService(manager, store), bench: benchmark.NewService(manager, store)}
	router := chi.NewRouter()
	router.Get("/health", api.health)
	router.Get("/config", api.getConfig)
	router.Put("/config", api.updateConfig)
	router.Post("/config/reload", api.reloadConfig)
	router.Get("/providers/{providerID}/models", api.models)
	router.Post("/providers/{providerID}/test", api.testProvider)
	router.Post("/mcp/{serverID}/test", api.testMCP)
	router.Get("/sessions", api.listSessions)
	router.Get("/sessions/{sessionID}", api.getSession)
	router.Delete("/sessions/{sessionID}", api.deleteSession)
	router.Post("/chat", api.sendChat)
	router.Get("/benchmarks/cases", api.listBenchmarkCases)
	router.Post("/benchmarks/cases", api.saveBenchmarkCase)
	router.Get("/benchmarks/cases/{caseID}", api.getBenchmarkCase)
	router.Post("/benchmarks/cases/{caseID}/runs", api.startBenchmarkRun)
	router.Get("/benchmarks/runs", api.listBenchmarkRuns)
	router.Get("/benchmarks/runs/{runID}", api.getBenchmarkRun)
	return router
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	a.writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "skillbox-bench"})
}

func (a *API) getConfig(w http.ResponseWriter, _ *http.Request) {
	a.writeJSON(w, http.StatusOK, a.config.Public())
}

func (a *API) updateConfig(w http.ResponseWriter, r *http.Request) {
	var input benchmarkconfig.PublicConfig
	if err := decodeJSON(r, &input); err != nil {
		a.writeError(w, http.StatusBadRequest, err)
		return
	}
	restartRequired, err := a.config.Update(benchmarkconfig.FromPublic(input))
	if err != nil {
		a.writeError(w, http.StatusBadRequest, err)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"config": a.config.Public(), "restart_required": restartRequired})
}

func (a *API) reloadConfig(w http.ResponseWriter, _ *http.Request) {
	if err := a.config.Reload(); err != nil {
		a.writeError(w, http.StatusBadRequest, err)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"config": a.config.Public()})
}

func (a *API) models(w http.ResponseWriter, r *http.Request) {
	item, ok := a.provider(chi.URLParam(r, "providerID"))
	if !ok {
		a.writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	models, err := provider.New(item).Models(r.Context())
	if err != nil {
		a.writeError(w, http.StatusBadGateway, err)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (a *API) testProvider(w http.ResponseWriter, r *http.Request) {
	item, ok := a.provider(chi.URLParam(r, "providerID"))
	if !ok {
		a.writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	models, err := provider.New(item).Models(r.Context())
	if err != nil {
		a.writeError(w, http.StatusBadGateway, err)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"status": "connected", "models": len(models)})
}

func (a *API) testMCP(w http.ResponseWriter, r *http.Request) {
	item, ok := a.mcpServer(chi.URLParam(r, "serverID"))
	if !ok {
		a.writeError(w, http.StatusNotFound, errors.New("MCP server not found"))
		return
	}
	client := mcp.New(item)
	if err := client.Initialize(r.Context()); err != nil {
		a.writeError(w, http.StatusBadGateway, err)
		return
	}
	tools, err := client.ListTools(r.Context())
	if err != nil {
		a.writeError(w, http.StatusBadGateway, err)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"status": "connected", "tools": tools})
}

func (a *API) listSessions(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListSessions(r.Context(), 100)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, err)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"sessions": items})
}

func (a *API) getSession(w http.ResponseWriter, r *http.Request) {
	item, err := a.store.GetSession(r.Context(), chi.URLParam(r, "sessionID"))
	if err != nil {
		a.storageError(w, err)
		return
	}
	messages, err := a.store.ListMessages(r.Context(), item.ID)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, err)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"session": item, "messages": messages})
}

func (a *API) deleteSession(w http.ResponseWriter, r *http.Request) {
	if err := a.store.DeleteSession(r.Context(), chi.URLParam(r, "sessionID")); err != nil {
		a.storageError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) sendChat(w http.ResponseWriter, r *http.Request) {
	var input chat.SendRequest
	if err := decodeJSON(r, &input); err != nil {
		a.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := a.chat.Send(r.Context(), input)
	if err != nil {
		a.writeError(w, http.StatusBadGateway, err)
		return
	}
	a.writeJSON(w, http.StatusOK, result)
}

func (a *API) listBenchmarkCases(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListBenchmarkCases(r.Context())
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, err)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"cases": items})
}

func (a *API) saveBenchmarkCase(w http.ResponseWriter, r *http.Request) {
	var item storage.BenchmarkCase
	if err := decodeJSON(r, &item); err != nil {
		a.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := a.bench.SaveCase(r.Context(), &item); err != nil {
		a.writeError(w, http.StatusBadRequest, err)
		return
	}
	a.writeJSON(w, http.StatusOK, item)
}

func (a *API) getBenchmarkCase(w http.ResponseWriter, r *http.Request) {
	item, err := a.store.GetBenchmarkCase(r.Context(), chi.URLParam(r, "caseID"))
	if err != nil {
		a.storageError(w, err)
		return
	}
	a.writeJSON(w, http.StatusOK, item)
}

func (a *API) startBenchmarkRun(w http.ResponseWriter, r *http.Request) {
	item, err := a.bench.Start(r.Context(), chi.URLParam(r, "caseID"))
	if err != nil {
		if storage.IsNotFound(err) {
			a.writeError(w, http.StatusNotFound, errors.New("benchmark case not found"))
			return
		}
		a.writeError(w, http.StatusBadRequest, err)
		return
	}
	a.writeJSON(w, http.StatusAccepted, item)
}

func (a *API) listBenchmarkRuns(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListBenchmarkRuns(r.Context(), strings.TrimSpace(r.URL.Query().Get("case_id")), 100)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, err)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"runs": items})
}

func (a *API) getBenchmarkRun(w http.ResponseWriter, r *http.Request) {
	item, err := a.store.GetBenchmarkRun(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		a.storageError(w, err)
		return
	}
	a.writeJSON(w, http.StatusOK, item)
}

func (a *API) provider(id string) (benchmarkconfig.Provider, bool) {
	for _, item := range a.config.Current().Providers {
		if item.ID == id {
			return item, true
		}
	}
	return benchmarkconfig.Provider{}, false
}

func (a *API) mcpServer(id string) (benchmarkconfig.MCPServer, bool) {
	for _, item := range a.config.Current().MCPServers {
		if item.ID == id {
			return item, true
		}
	}
	return benchmarkconfig.MCPServer{}, false
}

func (a *API) storageError(w http.ResponseWriter, err error) {
	if storage.IsNotFound(err) {
		a.writeError(w, http.StatusNotFound, errors.New("not found"))
		return
	}
	a.writeError(w, http.StatusInternalServerError, err)
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func (a *API) writeError(w http.ResponseWriter, status int, err error) {
	message := strings.TrimSpace(err.Error())
	if len(message) > 2000 {
		message = message[:2000]
	}
	a.writeJSON(w, status, map[string]string{"error": message})
}

func (a *API) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
