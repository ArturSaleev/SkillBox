package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/aibox/skillbox/internal/codereview"
	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
	skillexporter "github.com/aibox/skillbox/internal/skills/exporter"
	skillimporter "github.com/aibox/skillbox/internal/skills/importer"
	skillpackage "github.com/aibox/skillbox/internal/skills/package"
	skillstore "github.com/aibox/skillbox/internal/skills/store"
	"github.com/go-chi/chi/v5"
)

type adminServer struct {
	store       ports.Storage
	skillsRoot  string
	workspaceID string
	reindex     func(context.Context) (skillstore.ReindexResult, error)
	reviewer    codereview.CodeReviewer
}

type adminSkill struct {
	domain.Skill
	Project    *domain.Project `json:"project,omitempty"`
	MCPProject string          `json:"mcp_project"`
}

// AdminHandler exposes the database-wide views used by the embedded Dashboard.
// Service-enabled handlers add narrowly scoped package-management writes.
func AdminHandler(store ports.Storage) http.Handler {
	return adminHandler(adminServer{store: store})
}

func AdminHandlerWithSkills(store ports.Storage, skillsRoot string, reindex func(context.Context) (skillstore.ReindexResult, error)) http.Handler {
	return adminHandler(adminServer{store: store, skillsRoot: skillsRoot, reindex: reindex})
}

func AdminHandlerWithServices(store ports.Storage, skillsRoot, workspaceID string, reindex func(context.Context) (skillstore.ReindexResult, error), reviewer codereview.CodeReviewer) http.Handler {
	return adminHandler(adminServer{store: store, skillsRoot: skillsRoot, workspaceID: workspaceID, reindex: reindex, reviewer: reviewer})
}

func adminHandler(server adminServer) http.Handler {
	router := chi.NewRouter()
	router.Get("/projects", server.listProjects)
	router.Get("/skills", server.listSkills)
	router.Post("/skills", server.createSkill)
	router.Get("/skills/{skillID}", server.getSkill)
	router.Put("/skills/{skillID}", server.updateSkill)
	router.Get("/executions", server.listExecutions)
	router.Get("/statistics", server.listStatistics)
	router.Get("/proposals", server.listProposals)
	router.Get("/security-reviews", server.listSecurityReviews)
	router.Get("/code-review/config", server.codeReviewConfig)
	router.Post("/skills/{skillID}/security-reviews", server.runSecurityReview)
	if server.skillsRoot != "" {
		router.Post("/imports/zip/preview", server.previewZIP)
		router.Post("/imports/zip", server.importZIP)
		router.Post("/imports/git/preview", server.previewGit)
		router.Post("/imports/git", server.importGit)
		router.Get("/skills/{skillID}/export", server.exportSkill)
	}
	return router
}

type adminSkillInput struct {
	domain.Skill
	MCPProject string `json:"mcp_project"`
}

func (s adminServer) createSkill(w http.ResponseWriter, r *http.Request) {
	var input adminSkillInput
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		s.writeBadRequest(w, err)
		return
	}
	skill := input.Skill
	skill.ID = ""
	skill.Status = domain.StatusDraft
	skill.CurrentVersion = 1
	if err := s.applyAdminScope(r.Context(), &skill, input.MCPProject); err != nil {
		s.writeBadRequest(w, err)
		return
	}
	if err := s.store.CreateSkill(r.Context(), &skill, "Dashboard draft", nil); err != nil {
		s.writeError(w, err)
		return
	}
	projects, err := s.store.ListProjects(r.Context(), nil)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, attachProjects([]domain.Skill{skill}, projects)[0])
}

func (s adminServer) updateSkill(w http.ResponseWriter, r *http.Request) {
	var input adminSkillInput
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		s.writeBadRequest(w, err)
		return
	}
	existing, err := s.store.GetSkill(r.Context(), chi.URLParam(r, "skillID"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	if existing.Status != domain.StatusDraft {
		s.writeBadRequest(w, errors.New("only draft Skills can be edited"))
		return
	}
	skill := input.Skill
	skill.ID = existing.ID
	skill.Scope, skill.WorkspaceID, skill.ProjectID = existing.Scope, existing.WorkspaceID, existing.ProjectID
	skill.Status, skill.CurrentVersion = domain.StatusDraft, existing.CurrentVersion
	if err = s.store.UpdateSkill(r.Context(), &skill, "Updated from Dashboard", nil); err != nil {
		s.writeError(w, err)
		return
	}
	projects, err := s.store.ListProjects(r.Context(), nil)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, attachProjects([]domain.Skill{skill}, projects)[0])
}

func (s adminServer) applyAdminScope(ctx context.Context, skill *domain.Skill, projectRoute string) error {
	skill.WorkspaceID, skill.ProjectID = nil, nil
	switch skill.Scope {
	case domain.ScopeGlobal:
		return nil
	case domain.ScopeWorkspace:
		if s.workspaceID == "" {
			return errors.New("default workspace is not configured")
		}
		skill.WorkspaceID = &s.workspaceID
		return nil
	case domain.ScopeProject:
		projectRoute = strings.TrimSpace(projectRoute)
		if projectRoute == "" {
			return errors.New("project is required when availability is limited to one project")
		}
		projects, err := s.store.ListProjects(ctx, nil)
		if err != nil {
			return err
		}
		for i := range projects {
			if projectRouteID(&projects[i]) == projectRoute {
				skill.WorkspaceID, skill.ProjectID = &projects[i].WorkspaceID, &projects[i].ID
				return nil
			}
		}
		return errors.New("selected project was not found")
	default:
		return errors.New("scope must be global, workspace, or project")
	}
}

func (s adminServer) codeReviewConfig(w http.ResponseWriter, _ *http.Request) {
	if s.reviewer == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	destination := s.reviewer.Destination()
	model := ""
	if named, ok := s.reviewer.(interface{ Model() string }); ok {
		model = named.Model()
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"configured": true, "provider": s.reviewer.Name(), "model": model, "endpoint": destination.Endpoint, "external": destination.External,
	})
}

type reviewRequest struct {
	ExternalTransmissionApproved bool `json:"external_transmission_approved"`
}

func (s adminServer) runSecurityReview(w http.ResponseWriter, r *http.Request) {
	if s.reviewer == nil {
		s.writeBadRequest(w, errors.New("AI code review is not configured"))
		return
	}
	var input reviewRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		s.writeBadRequest(w, err)
		return
	}
	skill, err := s.store.GetSkill(r.Context(), chi.URLParam(r, "skillID"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	if skill.PackagePath == "" {
		s.writeBadRequest(w, errors.New("Skill has no filesystem package to review"))
		return
	}
	if err = skillpackage.ValidateRelativePath(skill.PackagePath); err != nil {
		s.writeBadRequest(w, err)
		return
	}
	request, err := codereview.RequestFromPackage(filepath.Join(s.skillsRoot, filepath.FromSlash(skill.PackagePath)), skill)
	if err != nil {
		s.writeError(w, err)
		return
	}
	request.ExternalTransmissionApproved = input.ExternalTransmissionApproved
	repository, ok := s.store.(ports.SecurityReviewRepository)
	if !ok {
		s.writeError(w, errors.New("security review history is not supported by this storage"))
		return
	}
	result, err := (codereview.History{Reviews: repository}).ReviewAndRecord(r.Context(), s.reviewer, request)
	if err != nil {
		if errors.Is(err, codereview.ErrExternalConsentRequired) {
			s.writeBadRequest(w, err)
			return
		}
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, result)
}

func (s adminServer) listSecurityReviews(w http.ResponseWriter, r *http.Request) {
	repository, ok := s.store.(ports.SecurityReviewRepository)
	if !ok {
		s.writeError(w, errors.New("security review history is not supported by this storage"))
		return
	}
	skillID := strings.TrimSpace(r.URL.Query().Get("skill_id"))
	if skillID == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "skill_id is required"})
		return
	}
	reviews, err := repository.ListSecurityReviews(r.Context(), skillID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"reviews": reviews})
}

type gitImportRequest struct {
	URL      string `json:"url"`
	Revision string `json:"revision"`
}

func (s adminServer) previewZIP(w http.ResponseWriter, r *http.Request) {
	path, name, cleanup, err := uploadedZIP(w, r)
	if err != nil {
		s.writeError(w, err)
		return
	}
	defer cleanup()
	preview, err := skillimporter.PreviewZIP(path, name)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, preview)
}

func (s adminServer) importZIP(w http.ResponseWriter, r *http.Request) {
	path, name, cleanup, err := uploadedZIP(w, r)
	if err != nil {
		s.writeError(w, err)
		return
	}
	defer cleanup()
	result, err := skillimporter.New(s.skillsRoot).ZIPWithSource(path, name)
	if err == nil && s.reindex != nil {
		_, err = s.reindex(r.Context())
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, result)
}

func (s adminServer) previewGit(w http.ResponseWriter, r *http.Request) {
	request, err := decodeGitRequest(r)
	if err != nil {
		s.writeError(w, err)
		return
	}
	preview, err := skillimporter.PreviewGit(request.URL, request.Revision)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, preview)
}

func (s adminServer) importGit(w http.ResponseWriter, r *http.Request) {
	request, err := decodeGitRequest(r)
	if err != nil {
		s.writeError(w, err)
		return
	}
	result, err := skillimporter.New(s.skillsRoot).Git(request.URL, request.Revision)
	if err == nil && s.reindex != nil {
		_, err = s.reindex(r.Context())
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, result)
}

func (s adminServer) exportSkill(w http.ResponseWriter, r *http.Request) {
	skill, err := s.store.GetSkill(r.Context(), chi.URLParam(r, "skillID"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	if skill.PackagePath == "" {
		s.writeError(w, errors.New("Skill has no filesystem package"))
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, safeDownloadName(skill.Slug)))
	w.Header().Set("Cache-Control", "no-store")
	if _, err = skillexporter.WriteZIP(filepath.Join(s.skillsRoot, filepath.FromSlash(skill.PackagePath)), w); err != nil {
		return
	}
}

func uploadedZIP(w http.ResponseWriter, r *http.Request) (string, string, func(), error) {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<20)
	file, header, err := r.FormFile("file")
	if err != nil {
		return "", "", func() {}, errors.New("ZIP file is required")
	}
	defer file.Close()
	tmp, err := os.CreateTemp("", "skillbox-upload-*.zip")
	if err != nil {
		return "", "", func() {}, err
	}
	cleanup := func() { _ = os.Remove(tmp.Name()) }
	if _, err = io.Copy(tmp, file); err == nil {
		err = tmp.Close()
	}
	if err != nil {
		cleanup()
		return "", "", func() {}, err
	}
	return tmp.Name(), filepath.Base(header.Filename), cleanup, nil
}

func decodeGitRequest(r *http.Request) (gitImportRequest, error) {
	var request gitImportRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, err
	}
	request.URL = strings.TrimSpace(request.URL)
	if request.URL == "" {
		return request, errors.New("Git repository URL is required")
	}
	return request, nil
}

func safeDownloadName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "skill"
	}
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, value)
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

func (s adminServer) writeBadRequest(w http.ResponseWriter, err error) {
	s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func (s adminServer) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
