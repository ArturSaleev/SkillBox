package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
)

type adminStoreStub struct {
	ports.Storage
	projects []domain.Project
	skills   []domain.Skill
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
