// Package codereview defines an AI code-review boundary with explicit external
// transmission consent. Providers receive code only through Review.
package codereview

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/aibox/skillbox/internal/skills/securityscan"
)

var ErrExternalConsentRequired = errors.New("external code transmission requires explicit user approval")

type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type Request struct {
	SkillID                      string              `json:"skill_id"`
	PackageHash                  string              `json:"package_hash"`
	PackageName                  string              `json:"package_name"`
	Files                        []File              `json:"files"`
	StaticReport                 securityscan.Report `json:"static_report"`
	ExternalTransmissionApproved bool                `json:"external_transmission_approved"`
}

type Finding struct {
	Path           string `json:"path,omitempty"`
	Line           int    `json:"line,omitempty"`
	Severity       string `json:"severity"`
	Category       string `json:"category"`
	Title          string `json:"title"`
	Explanation    string `json:"explanation"`
	Recommendation string `json:"recommendation"`
}

type Result struct {
	Provider       string    `json:"provider"`
	Model          string    `json:"model"`
	Summary        string    `json:"summary"`
	Findings       []Finding `json:"findings"`
	Recommendation string    `json:"recommendation"`
	RawResponse    string    `json:"raw_response,omitempty"`
}

type Destination struct {
	Endpoint string `json:"endpoint"`
	External bool   `json:"external"`
}

type CodeReviewer interface {
	Name() string
	Destination() Destination
	Review(context.Context, Request) (Result, error)
}

func Review(ctx context.Context, reviewer CodeReviewer, request Request) (Result, error) {
	if reviewer.Destination().External && !request.ExternalTransmissionApproved {
		return Result{}, fmt.Errorf("%w: code would be sent to %s", ErrExternalConsentRequired, reviewer.Destination().Endpoint)
	}
	return reviewer.Review(ctx, request)
}

type Factory func(Config) (CodeReviewer, error)

type Config struct {
	Provider string
	Endpoint string
	Model    string
	APIKey   string
}

type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

func NewRegistry() *Registry { return &Registry{factories: map[string]Factory{}} }

func (r *Registry) Register(name string, factory Factory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == "" || factory == nil {
		return errors.New("provider name and factory are required")
	}
	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("code-review provider %q is already registered", name)
	}
	r.factories[name] = factory
	return nil
}

func (r *Registry) Create(config Config) (CodeReviewer, error) {
	r.mu.RLock()
	factory := r.factories[config.Provider]
	r.mu.RUnlock()
	if factory == nil {
		return nil, fmt.Errorf("unknown code-review provider %q", config.Provider)
	}
	return factory(config)
}

func (r *Registry) Providers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	providers := make([]string, 0, len(r.factories))
	for name := range r.factories {
		providers = append(providers, name)
	}
	sort.Strings(providers)
	return providers
}

func DefaultRegistry() *Registry {
	registry := NewRegistry()
	_ = registry.Register("openai-compatible", func(config Config) (CodeReviewer, error) {
		return NewOpenAICompatible(config.Endpoint, config.Model, config.APIKey, nil)
	})
	return registry
}
