package dashboard

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
	"github.com/go-chi/chi/v5"
)

type adminServer struct {
	store ports.Storage
}

type adminSkill struct {
	domain.Skill
	Project    *domain.Project `json:"project,omitempty"`
	MCPProject string          `json:"mcp_project"`
}

// AdminHandler exposes the read-only, database-wide views used by the embedded
// Dashboard. Skill mutations still go through the project-scoped Teacher MCP.
func AdminHandler(store ports.Storage) http.Handler {
	server := adminServer{store: store}
	router := chi.NewRouter()
	router.Get("/projects", server.listProjects)
	router.Get("/skills", server.listSkills)
	router.Get("/skills/{skillID}", server.getSkill)
	router.Get("/executions", server.listExecutions)
	router.Get("/statistics", server.listStatistics)
	router.Get("/proposals", server.listProposals)
	return router
}

func (s adminServer) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.store.ListProjects(r.Context(), nil)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (s adminServer) listSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := s.store.ListSkills(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	projects, err := s.store.ListProjects(r.Context(), nil)
	if err != nil {
		s.writeError(w, err)
		return
	}
	items := attachProjects(skills, projects)
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	s.writeJSON(w, http.StatusOK, map[string]any{"skills": items})
}

func (s adminServer) getSkill(w http.ResponseWriter, r *http.Request) {
	skill, err := s.store.GetSkill(r.Context(), chi.URLParam(r, "skillID"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	projects, err := s.store.ListProjects(r.Context(), nil)
	if err != nil {
		s.writeError(w, err)
		return
	}
	items := attachProjects([]domain.Skill{*skill}, projects)
	s.writeJSON(w, http.StatusOK, items[0])
}

func (s adminServer) listExecutions(w http.ResponseWriter, r *http.Request) {
	skillID := optionalQuery(r, "skill_id")
	executions, err := s.store.ListExecutions(r.Context(), skillID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	limit := queryLimit(r, 100, 500)
	if len(executions) > limit {
		executions = executions[:limit]
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"executions": executions})
}

func (s adminServer) listStatistics(w http.ResponseWriter, r *http.Request) {
	statistics, err := s.store.Statistics(r.Context(), optionalQuery(r, "skill_id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"statistics": statistics})
}

func (s adminServer) listProposals(w http.ResponseWriter, r *http.Request) {
	proposals, err := s.store.ListSkillProposals(r.Context(), optionalQuery(r, "skill_id"), strings.TrimSpace(r.URL.Query().Get("status")))
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"proposals": proposals})
}

func attachProjects(skills []domain.Skill, projects []domain.Project) []adminSkill {
	projectByID := make(map[string]*domain.Project, len(projects))
	fallback := ""
	for i := range projects {
		project := &projects[i]
		projectByID[project.ID] = project
		if fallback == "" {
			fallback = projectRouteID(project)
		}
	}
	items := make([]adminSkill, 0, len(skills))
	for _, skill := range skills {
		item := adminSkill{Skill: skill, MCPProject: fallback}
		if skill.ProjectID != nil {
			item.Project = projectByID[*skill.ProjectID]
			if item.Project != nil {
				item.MCPProject = projectRouteID(item.Project)
			}
		}
		items = append(items, item)
	}
	return items
}

func projectRouteID(project *domain.Project) string {
	if project.ExternalID != "" {
		return project.ExternalID
	}
	return project.Slug
}

func optionalQuery(r *http.Request, name string) *string {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return nil
	}
	return &value
}

func queryLimit(r *http.Request, fallback, maximum int) int {
	value, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func (s adminServer) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, ports.ErrNotFound) {
		status = http.StatusNotFound
	}
	s.writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s adminServer) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
