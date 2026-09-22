package codereview

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
)

// History runs a review through the consent boundary and persists its result
// against the exact immutable package hash that was reviewed.
type History struct {
	Reviews ports.SecurityReviewRepository
}

func (h History) ReviewAndRecord(ctx context.Context, reviewer CodeReviewer, request Request) (Result, error) {
	if h.Reviews == nil {
		return Result{}, errors.New("security review repository is required")
	}
	if request.SkillID == "" || request.PackageHash == "" {
		return Result{}, errors.New("skill ID and package hash are required")
	}
	result, err := Review(ctx, reviewer, request)
	if err != nil {
		return Result{}, err
	}
	findings, err := json.Marshal(result.Findings)
	if err != nil {
		return Result{}, err
	}
	review := domain.SecurityReview{
		SkillID:        request.SkillID,
		PackageHash:    request.PackageHash,
		Provider:       result.Provider,
		Model:          result.Model,
		Summary:        result.Summary,
		Findings:       findings,
		Recommendation: result.Recommendation,
	}
	if err = h.Reviews.CreateSecurityReview(ctx, &review); err != nil {
		return Result{}, err
	}
	return result, nil
}
