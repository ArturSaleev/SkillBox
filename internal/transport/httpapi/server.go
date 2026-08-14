package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aibox/skillbox/internal/application"
	"github.com/aibox/skillbox/internal/auth"
	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/metrics"
	"github.com/aibox/skillbox/internal/ports"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	app     *application.Service
	metrics *metrics.Metrics
}

func New(app *application.Service, m *metrics.Metrics) *Server { return &Server{app: app, metrics: m} }
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) {
		r.Get("/workspaces", s.listWorkspaces)
		r.Post("/workspaces", s.createWorkspace)
		r.Get("/projects", s.listProjects)
		r.Post("/projects", s.createProject)
		r.Get("/skills", s.listSkills)
		r.Post("/skills", s.createSkill)
		r.Post("/skills/search", s.search)
		r.Post("/skills/prepare", s.prepare)
		r.Route("/skills/{id}", func(r chi.Router) {
			r.Get("/", s.getSkill)
			r.Put("/", s.updateSkill)
			r.Get("/versions", s.versions)
			r.Get("/versions/{version}", s.version)
			r.Post("/versions/{version}/rollback", s.rollback)
			r.Get("/steps", s.skillPart("steps"))
			r.Get("/tools", s.skillPart("tools"))
			r.Get("/contexts", s.skillPart("contexts"))
			r.Get("/dependencies", s.skillPart("dependencies"))
			r.Get("/examples", s.skillPart("examples"))
		})
		r.Get("/executions", s.listExecutions)
		r.Post("/executions", s.createExecution)
		r.Get("/statistics", s.statistics)
		r.Get("/mcp-profiles", s.listMCPProfiles)
		r.Post("/mcp-profiles", s.upsertMCPProfile)
		r.Get("/mcp-connections", s.listMCPConnections)
		r.Post("/mcp-connections", s.createMCPConnection)
		r.Get("/skill-proposals", s.listSkillProposals)
		r.Get("/skill-proposals/{id}", s.getSkillProposal)
		r.Post("/skill-proposals/{id}/approve", s.reviewSkillProposal("approved"))
		r.Post("/skill-proposals/{id}/reject", s.reviewSkillProposal("rejected"))
		r.Post("/skill-proposals/{id}/publish", s.publishSkillProposal)
	})
	return r
}

type skillWrite struct {
	domain.Skill
	ChangeSummary string  `json:"change_summary"`
	CreatedBy     *string `json:"created_by,omitempty"`
}

func (s *Server) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var v domain.Workspace
	if !decode(w, r, &v) {
		return
	}
	if strings.TrimSpace(v.Slug) == "" || strings.TrimSpace(v.Name) == "" {
		fail(w, 422, "validation_error", "slug and name are required")
		return
	}
	if err := s.app.Store.CreateWorkspace(r.Context(), &v); err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 201, v)
}
func (s *Server) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	v, err := s.app.Store.ListWorkspaces(r.Context())
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, map[string]any{"workspaces": v})
}
func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var v domain.Project
	if !decode(w, r, &v) {
		return
	}
	if strings.TrimSpace(v.WorkspaceID) == "" || strings.TrimSpace(v.Slug) == "" || strings.TrimSpace(v.Name) == "" {
		fail(w, 422, "validation_error", "workspace_id, slug and name are required")
		return
	}
	if err := s.app.Store.CreateProject(r.Context(), &v); err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 201, v)
}
func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	var id *string
	if v := r.URL.Query().Get("workspace_id"); v != "" {
		id = &v
	}
	items, err := s.app.Store.ListProjects(r.Context(), id)
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, map[string]any{"projects": items})
}
func (s *Server) createSkill(w http.ResponseWriter, r *http.Request) {
	var v skillWrite
	if !decode(w, r, &v) {
		return
	}
	if err := s.app.Store.CreateSkill(r.Context(), &v.Skill, v.ChangeSummary, v.CreatedBy); err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 201, v.Skill)
}
func (s *Server) updateSkill(w http.ResponseWriter, r *http.Request) {
	var v skillWrite
	if !decode(w, r, &v) {
		return
	}
	v.ID = chi.URLParam(r, "id")
	if err := s.app.Store.UpdateSkill(r.Context(), &v.Skill, v.ChangeSummary, v.CreatedBy); err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, v.Skill)
}
func (s *Server) getSkill(w http.ResponseWriter, r *http.Request) {
	v, err := s.app.Store.GetSkill(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, v)
}
func (s *Server) listSkills(w http.ResponseWriter, r *http.Request) {
	v, err := s.app.Store.ListSkills(r.Context())
	if err != nil {
		s.dbFail(w, err)
		return
	}
	for i := range v {
		v[i].Instructions = ""
		v[i].Steps = nil
		v[i].Tools = nil
		v[i].Contexts = nil
		v[i].Dependencies = nil
		v[i].Examples = nil
	}
	write(w, 200, map[string]any{"skills": v})
}
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	var v domain.SearchFilter
	if !decode(w, r, &v) {
		return
	}
	start := time.Now()
	items, err := s.app.Search(r.Context(), v)
	s.metrics.SearchDuration.Observe(time.Since(start).Seconds())
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, map[string]any{"skills": items})
}
func (s *Server) prepare(w http.ResponseWriter, r *http.Request) {
	var v domain.PrepareRequest
	if !decode(w, r, &v) {
		return
	}
	start := time.Now()
	item, err := s.app.Prepare(r.Context(), v)
	s.metrics.CompileDuration.Observe(time.Since(start).Seconds())
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, item)
}
func (s *Server) versions(w http.ResponseWriter, r *http.Request) {
	v, err := s.app.Store.ListVersions(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, map[string]any{"versions": v})
}
func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil {
		fail(w, 400, "invalid_version", "version must be an integer")
		return
	}
	v, err := s.app.Store.GetVersion(r.Context(), chi.URLParam(r, "id"), n)
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, v)
}
func (s *Server) rollback(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil {
		fail(w, 400, "invalid_version", "version must be an integer")
		return
	}
	var body struct {
		CreatedBy *string `json:"created_by"`
	}
	if r.ContentLength > 0 && !decode(w, r, &body) {
		return
	}
	v, err := s.app.Store.RollbackSkill(r.Context(), chi.URLParam(r, "id"), n, body.CreatedBy)
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, v)
}
func (s *Server) skillPart(part string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := s.app.Store.GetSkill(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			s.dbFail(w, err)
			return
		}
		switch part {
		case "steps":
			write(w, 200, map[string]any{"steps": v.Steps})
		case "tools":
			write(w, 200, map[string]any{"tools": v.Tools})
		case "contexts":
			write(w, 200, map[string]any{"contexts": v.Contexts})
		case "dependencies":
			write(w, 200, map[string]any{"dependencies": v.Dependencies})
		case "examples":
			write(w, 200, map[string]any{"examples": v.Examples})
		}
	}
}
func (s *Server) createExecution(w http.ResponseWriter, r *http.Request) {
	var v domain.Execution
	if !decode(w, r, &v) {
		return
	}
	if err := s.app.Store.CreateExecution(r.Context(), &v); err != nil {
		s.dbFail(w, err)
		return
	}
	s.metrics.Executions.Inc()
	if v.Success {
		s.metrics.ExecutionSuccess.Inc()
	}
	write(w, 201, v)
}
func (s *Server) listExecutions(w http.ResponseWriter, r *http.Request) {
	var id *string
	if v := r.URL.Query().Get("skill_id"); v != "" {
		id = &v
	}
	items, err := s.app.Store.ListExecutions(r.Context(), id)
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, map[string]any{"executions": items})
}
func (s *Server) statistics(w http.ResponseWriter, r *http.Request) {
	var id *string
	if v := r.URL.Query().Get("skill_id"); v != "" {
		id = &v
	}
	items, err := s.app.Store.Statistics(r.Context(), id)
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, map[string]any{"statistics": items})
}

func (s *Server) listMCPProfiles(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.Store.ListMCPProfiles(r.Context())
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, map[string]any{"profiles": items})
}
func (s *Server) upsertMCPProfile(w http.ResponseWriter, r *http.Request) {
	var v domain.MCPProfile
	if !decode(w, r, &v) {
		return
	}
	if err := application.ValidateProfile(&v); err != nil {
		fail(w, 422, "validation_error", err.Error())
		return
	}
	if existing, err := s.app.Store.GetMCPProfileBySlug(r.Context(), v.Slug); err == nil && existing.BuiltIn {
		fail(w, 409, "built_in_profile", "built-in profiles cannot be changed through REST")
		return
	}
	v.BuiltIn = false
	if !v.Enabled {
		v.Enabled = true
	}
	if err := s.app.Store.UpsertMCPProfile(r.Context(), &v); err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, v)
}

type connectionWrite struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	WorkspaceID *string `json:"workspace_id,omitempty"`
	ProjectID   *string `json:"project_id,omitempty"`
	ProfileID   string  `json:"profile_id,omitempty"`
	ProfileSlug string  `json:"profile_slug,omitempty"`
	APIKey      string  `json:"api_key,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

func (s *Server) createMCPConnection(w http.ResponseWriter, r *http.Request) {
	var v connectionWrite
	if !decode(w, r, &v) {
		return
	}
	if strings.TrimSpace(v.Slug) == "" || strings.TrimSpace(v.Name) == "" {
		fail(w, 422, "validation_error", "slug and name are required")
		return
	}
	profileID := v.ProfileID
	if profileID == "" {
		profile, err := s.app.Store.GetMCPProfileBySlug(r.Context(), v.ProfileSlug)
		if err != nil {
			s.dbFail(w, err)
			return
		}
		profileID = profile.ID
	} else if _, err := s.app.Store.GetMCPProfile(r.Context(), profileID); err != nil {
		s.dbFail(w, err)
		return
	}
	if v.ProjectID != nil {
		if v.WorkspaceID == nil {
			fail(w, 422, "validation_error", "project-scoped connection requires workspace_id")
			return
		}
		projects, err := s.app.Store.ListProjects(r.Context(), v.WorkspaceID)
		if err != nil {
			s.dbFail(w, err)
			return
		}
		found := false
		for _, project := range projects {
			if project.ID == *v.ProjectID {
				found = true
				break
			}
		}
		if !found {
			fail(w, 422, "validation_error", "project_id does not belong to workspace_id")
			return
		}
	}
	key := strings.TrimSpace(v.APIKey)
	if key == "" {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			fail(w, 500, "key_generation_failed", err.Error())
			return
		}
		key = base64.RawURLEncoding.EncodeToString(raw)
	} else if len(key) < 32 {
		fail(w, 422, "weak_api_key", "provided API key must contain at least 32 characters")
		return
	}
	enabled := true
	if v.Enabled != nil {
		enabled = *v.Enabled
	}
	connection := domain.MCPConnection{Slug: v.Slug, Name: v.Name, WorkspaceID: v.WorkspaceID, ProjectID: v.ProjectID, ProfileID: profileID, AuthType: "api_key", CredentialHash: auth.HashAPIKey(key), Enabled: enabled}
	if err := s.app.Store.CreateMCPConnection(r.Context(), &connection); err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 201, map[string]any{"connection": connection, "api_key": key, "warning": "API key is returned once; store it securely"})
}
func (s *Server) listMCPConnections(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.Store.ListMCPConnections(r.Context())
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, map[string]any{"connections": items})
}
func (s *Server) listSkillProposals(w http.ResponseWriter, r *http.Request) {
	var skillID *string
	if v := r.URL.Query().Get("skill_id"); v != "" {
		skillID = &v
	}
	items, err := s.app.Store.ListSkillProposals(r.Context(), skillID, r.URL.Query().Get("status"))
	if err != nil {
		s.dbFail(w, err)
		return
	}
	for i := range items {
		items[i].ProposedSnapshot = ""
	}
	write(w, 200, map[string]any{"proposals": items})
}
func (s *Server) getSkillProposal(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.Store.GetSkillProposal(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, item)
}
func (s *Server) reviewSkillProposal(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ReviewedBy *string `json:"reviewed_by"`
			Note       *string `json:"note"`
		}
		if r.ContentLength > 0 && !decode(w, r, &body) {
			return
		}
		item, err := s.app.Store.ReviewSkillProposal(r.Context(), chi.URLParam(r, "id"), status, body.ReviewedBy, body.Note)
		if err != nil {
			s.dbFail(w, err)
			return
		}
		write(w, 200, item)
	}
}
func (s *Server) publishSkillProposal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PublishedBy string `json:"published_by"`
	}
	if r.ContentLength > 0 && !decode(w, r, &body) {
		return
	}
	if body.PublishedBy == "" {
		body.PublishedBy = "rest-admin"
	}
	item, err := s.app.PublishProposalAdmin(r.Context(), chi.URLParam(r, "id"), body.PublishedBy)
	if err != nil {
		s.dbFail(w, err)
		return
	}
	write(w, 200, item)
}
func (s *Server) dbFail(w http.ResponseWriter, err error) {
	if errors.Is(err, ports.ErrNotFound) {
		fail(w, 404, "not_found", err.Error())
		return
	}
	s.metrics.DBErrors.Inc()
	fail(w, 500, "internal_error", err.Error())
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, 400, "invalid_json", err.Error())
		return false
	}
	return true
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, code, message string) {
	write(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
