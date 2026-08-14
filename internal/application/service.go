package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aibox/skillbox/internal/compiler"
	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
	"github.com/aibox/skillbox/internal/search"
)

type Service struct {
	Store         ports.Storage
	SearchService *search.Service
	Compiler      *compiler.Compiler
}

func New(store ports.Storage) *Service {
	return &Service{Store: store, SearchService: search.New(store, search.DefaultScorer()), Compiler: compiler.New(store)}
}
func (s *Service) Search(ctx context.Context, f domain.SearchFilter) ([]domain.Candidate, error) {
	return s.SearchService.Search(ctx, f)
}
func (s *Service) Prepare(ctx context.Context, r domain.PrepareRequest) (domain.PreparedSkill, error) {
	if strings.TrimSpace(r.Task) == "" {
		return domain.PreparedSkill{}, errors.New("task is required")
	}
	var sk *domain.Skill
	var err error
	if r.SkillID != nil && *r.SkillID != "" {
		sk, err = s.Store.GetSkill(ctx, *r.SkillID)
	} else {
		candidates, e := s.Search(ctx, domain.SearchFilter{Task: r.Task, WorkspaceID: r.WorkspaceID, ProjectID: r.ProjectID, OwnerUserID: r.OwnerUserID, Domains: r.Domains, Intents: r.Intents, ObjectTypes: r.ObjectTypes, AvailableTools: r.AvailableTools, Limit: 1})
		if e != nil {
			return domain.PreparedSkill{}, e
		}
		if len(candidates) == 0 {
			return domain.PreparedSkill{}, fmt.Errorf("no matching skill: %w", ports.ErrNotFound)
		}
		sk, err = s.Store.GetSkill(ctx, candidates[0].ID)
	}
	if err != nil {
		return domain.PreparedSkill{}, err
	}
	return s.Compiler.Compile(ctx, sk, r)
}
