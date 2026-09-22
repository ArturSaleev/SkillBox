package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aibox/skillbox/internal/codereview"
	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
	skillpackage "github.com/aibox/skillbox/internal/skills/package"
)

type adminStoreStub struct {
	ports.Storage
	projects []domain.Project
	skills   []domain.Skill
}

type securityAdminStoreStub struct {
	adminStoreStub
	reviews []domain.SecurityReview
}

type createAdminStoreStub struct {
	adminStoreStub
	created domain.Skill
	updated domain.Skill
}

func (s *createAdminStoreStub) UpdateSkill(_ context.Context, skill *domain.Skill, _ string, _ *string) error {
	s.updated = *skill
	return nil
}

func (s *createAdminStoreStub) CreateSkill(_ context.Context, skill *domain.Skill, _ string, _ *string) error {
	s.created = *skill
	return nil
}

type reviewStub struct{ external bool }

func (r reviewStub) Name() string { return "test-reviewer" }
func (r reviewStub) Destination() codereview.Destination {
	return codereview.Destination{Endpoint: "https://review.example.test/v1", External: r.external}
}
func (reviewStub) Review(_ context.Context, request codereview.Request) (codereview.Result, error) {
	return codereview.Result{Provider: "test-reviewer", Model: "test-model", Summary: request.PackageName + " reviewed"}, nil
}

func (s securityAdminStoreStub) CreateSecurityReview(context.Context, *domain.SecurityReview) error {
	return nil
}

func (s securityAdminStoreStub) ListSecurityReviews(_ context.Context, skillID string) ([]domain.SecurityReview, error) {
	if skillID == "missing" {
		return nil, ports.ErrNotFound
	}
	return s.reviews, nil
}

func (s adminStoreStub) ListProjects(context.Context, *string) ([]domain.Project, error) {
	return s.projects, nil
}

func (s adminStoreStub) ListSkills(context.Context) ([]domain.Skill, error) {
	return s.skills, nil
}

func (s adminStoreStub) GetSkill(_ context.Context, id string) (*domain.Skill, error) {
	for i := range s.skills {
		if s.skills[i].ID == id {
			return &s.skills[i], nil
		}
	}
	return nil, ports.ErrNotFound
}

func TestAdminHandlerListsSkillsAcrossProjects(t *testing.T) {
	oneID, twoID := "project-one-id", "project-two-id"
	store := adminStoreStub{
		projects: []domain.Project{
			{ID: oneID, ExternalID: "project-one", Slug: "project-one", Name: "Project One"},
			{ID: twoID, ExternalID: "project-two", Slug: "project-two", Name: "Project Two"},
		},
		skills: []domain.Skill{
			{ID: "skill-one", ProjectID: &oneID, Name: "First", UpdatedAt: time.Unix(1, 0)},
			{ID: "skill-two", ProjectID: &twoID, Name: "Second", UpdatedAt: time.Unix(2, 0)},
		},
	}
	recorder := httptest.NewRecorder()
	AdminHandler(store).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/skills", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Skills []adminSkill `json:"skills"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Skills) != 2 {
		t.Fatalf("skills=%d want=2", len(response.Skills))
	}
	if response.Skills[0].ID != "skill-two" || response.Skills[0].MCPProject != "project-two" || response.Skills[0].Project == nil {
		t.Fatalf("unexpected first skill: %+v", response.Skills[0])
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("Cache-Control=%q", cache)
	}
}

func TestAdminHandlerListsSecurityReviewHistory(t *testing.T) {
	store := securityAdminStoreStub{reviews: []domain.SecurityReview{{
		ID: "review-id", SkillID: "skill-id", PackageHash: "hash", Provider: "local", Model: "model",
	}}}
	recorder := httptest.NewRecorder()
	AdminHandler(store).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/security-reviews?skill_id=skill-id", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Reviews []domain.SecurityReview `json:"reviews"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Reviews) != 1 || response.Reviews[0].ID != "review-id" {
		t.Fatalf("unexpected reviews: %#v", response.Reviews)
	}
}

func TestAdminHandlerRequiresSkillForSecurityReviewHistory(t *testing.T) {
	recorder := httptest.NewRecorder()
	AdminHandler(securityAdminStoreStub{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/security-reviews", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminHandlerGetsSkillWithProjectRoutingMetadata(t *testing.T) {
	projectID := "project-id"
	store := adminStoreStub{
		projects: []domain.Project{{ID: projectID, ExternalID: "external-project", Slug: "project", Name: "Project"}},
		skills:   []domain.Skill{{ID: "skill-id", ProjectID: &projectID, Name: "Skill"}},
	}
	recorder := httptest.NewRecorder()
	AdminHandler(store).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/skills/skill-id", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response adminSkill
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.MCPProject != "external-project" || response.Project == nil || response.Project.ID != projectID {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestAdminCreatesGlobalSkillWithoutProject(t *testing.T) {
	store := &createAdminStoreStub{}
	body := `{"slug":"portable-skill","name":"Portable Skill","description":"Reusable procedure","purpose":"Help agents","when_to_use":"When needed","instructions":"Follow the procedure","success_criteria":["Done"],"scope":"global","status":"active","priority":0,"domains":[],"intents":[],"object_types":[],"tags":[],"keywords":[],"capabilities":[],"compatibility":[],"steps":[],"tools":[],"context_requirements":[],"dependencies":[],"examples":[],"mcp_project":""}`
	recorder := httptest.NewRecorder()
	AdminHandlerWithServices(store, t.TempDir(), "workspace-id", nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/skills", strings.NewReader(body)))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if store.created.Scope != domain.ScopeGlobal || store.created.ProjectID != nil || store.created.WorkspaceID != nil || store.created.Status != domain.StatusDraft {
		t.Fatalf("unexpected created Skill: %+v", store.created)
	}
}

func TestAdminRequiresProjectOnlyForProjectScope(t *testing.T) {
	store := &createAdminStoreStub{}
	body := `{"slug":"project-skill","name":"Project Skill","description":"Reusable procedure","purpose":"Help agents","when_to_use":"When needed","instructions":"Follow the procedure","success_criteria":["Done"],"scope":"project","status":"draft","priority":0,"domains":[],"intents":[],"object_types":[],"tags":[],"keywords":[],"capabilities":[],"compatibility":[],"steps":[],"tools":[],"context_requirements":[],"dependencies":[],"examples":[],"mcp_project":""}`
	recorder := httptest.NewRecorder()
	AdminHandlerWithServices(store, t.TempDir(), "workspace-id", nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/skills", strings.NewReader(body)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminUpdatesGlobalDraftWithoutProject(t *testing.T) {
	store := &createAdminStoreStub{adminStoreStub: adminStoreStub{skills: []domain.Skill{{ID: "skill-id", Slug: "portable-skill", Scope: domain.ScopeGlobal, Status: domain.StatusDraft, CurrentVersion: 1}}}}
	body := `{"id":"skill-id","slug":"portable-skill","name":"Updated Skill","description":"Reusable procedure","purpose":"Help agents","when_to_use":"When needed","instructions":"Follow the updated procedure","success_criteria":["Done"],"scope":"project","status":"active","priority":0,"domains":[],"intents":[],"object_types":[],"tags":[],"keywords":[],"capabilities":[],"compatibility":[],"steps":[],"tools":[],"context_requirements":[],"dependencies":[],"examples":[],"mcp_project":""}`
	recorder := httptest.NewRecorder()
	AdminHandlerWithServices(store, t.TempDir(), "workspace-id", nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/skills/skill-id", strings.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if store.updated.Scope != domain.ScopeGlobal || store.updated.ProjectID != nil || store.updated.Status != domain.StatusDraft || store.updated.Name != "Updated Skill" {
		t.Fatalf("unexpected updated Skill: %+v", store.updated)
	}
}

func TestAdminSecurityReviewRequiresExternalConsent(t *testing.T) {
	root := t.TempDir()
	packageRoot := filepath.Join(root, "reviewable")
	if err := skillpackage.WriteSkillMD(packageRoot, map[string]any{"name": "Reviewable", "description": "Review target"}, "Inspect carefully."); err != nil {
		t.Fatal(err)
	}
	pkg, err := skillpackage.Load(packageRoot)
	if err != nil {
		t.Fatal(err)
	}
	store := securityAdminStoreStub{adminStoreStub: adminStoreStub{skills: []domain.Skill{{ID: "skill-id", Name: "Reviewable", PackagePath: "reviewable", PackageHash: pkg.Hash}}}}
	handler := AdminHandlerWithServices(store, root, "workspace-id", nil, reviewStub{external: true})
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, httptest.NewRequest(http.MethodPost, "/skills/skill-id/security-reviews", strings.NewReader(`{"external_transmission_approved":false}`)))
	if denied.Code != http.StatusBadRequest {
		t.Fatalf("denied status=%d body=%s", denied.Code, denied.Body.String())
	}
	approved := httptest.NewRecorder()
	handler.ServeHTTP(approved, httptest.NewRequest(http.MethodPost, "/skills/skill-id/security-reviews", strings.NewReader(`{"external_transmission_approved":true}`)))
	if approved.Code != http.StatusCreated {
		t.Fatalf("approved status=%d body=%s", approved.Code, approved.Body.String())
	}
}
