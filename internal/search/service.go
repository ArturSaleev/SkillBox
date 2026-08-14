package search

import (
	"context"
	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
)

type Service struct {
	store  ports.Storage
	scorer Scorer
}

func New(store ports.Storage, scorer Scorer) *Service { return &Service{store: store, scorer: scorer} }
func (s *Service) Search(ctx context.Context, f domain.SearchFilter) ([]domain.Candidate, error) {
	skills, err := s.store.ListSkills(ctx)
	if err != nil {
		return nil, err
	}
	allStats, err := s.store.Statistics(ctx, nil)
	if err != nil {
		return nil, err
	}
	stats := map[string]domain.Statistics{}
	for _, st := range allStats {
		stats[st.SkillID] = st
	}
	return Candidates(skills, f, s.scorer, stats), nil
}
