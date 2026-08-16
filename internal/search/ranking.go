package search

import (
	"sort"
	"strings"
	"unicode"

	"github.com/aibox/skillbox/internal/domain"
)

type Scorer interface {
	Score(domain.Skill, domain.SearchFilter, *domain.Statistics) int
}

type WeightedScorer struct {
	Project, Workspace, Domain, Intent, ObjectType, Tag, Keyword, Tool, SuccessMax int
}

func DefaultScorer() WeightedScorer {
	return WeightedScorer{Project: 100, Workspace: 50, Domain: 30, Intent: 30, ObjectType: 30, Tag: 10, Keyword: 5, Tool: 10, SuccessMax: 20}
}

func (w WeightedScorer) Score(skill domain.Skill, f domain.SearchFilter, stats *domain.Statistics) int {
	score := skill.Priority
	if f.ProjectID != nil && skill.ProjectID != nil && *f.ProjectID == *skill.ProjectID {
		score += w.Project
	}
	if f.WorkspaceID != nil && skill.WorkspaceID != nil && *f.WorkspaceID == *skill.WorkspaceID {
		score += w.Workspace
	}
	score += matches(skill.Domains, f.Domains)*w.Domain + matches(skill.Intents, f.Intents)*w.Intent + matches(skill.ObjectTypes, f.ObjectTypes)*w.ObjectType + matches(skill.Tags, f.Tags)*w.Tag
	terms := append(tokenize(f.Task), f.Keywords...)
	score += matches(append(append([]string{}, skill.Keywords...), skill.Tags...), terms) * w.Keyword
	if f.RequiredTool != "" {
		for _, t := range skill.Tools {
			if strings.EqualFold(t.Name, f.RequiredTool) {
				score += w.Tool
			}
		}
	}
	if len(f.AvailableTools) > 0 {
		available := map[string]bool{}
		for _, name := range f.AvailableTools {
			available[strings.ToLower(name)] = true
		}
		for _, tool := range skill.Tools {
			if tool.Requirement != "required" {
				continue
			}
			if available[strings.ToLower(tool.Name)] {
				score += 3
			} else {
				score -= 20
			}
		}
	}
	if stats != nil && stats.Runs >= 3 {
		score += int(stats.SuccessRate * float64(w.SuccessMax) / 100)
	}
	return score
}

func Eligible(skill domain.Skill, f domain.SearchFilter) bool {
	status := f.Status
	if status == "" {
		status = string(domain.StatusActive)
	}
	if string(skill.Status) != status {
		return false
	}
	if len(f.Scopes) > 0 && !containsScope(f.Scopes, skill.Scope) {
		return false
	}
	switch skill.Scope {
	case domain.ScopeProject:
		if f.ProjectID == nil || skill.ProjectID == nil || *f.ProjectID != *skill.ProjectID {
			return false
		}
	case domain.ScopeWorkspace:
		if f.WorkspaceID == nil || skill.WorkspaceID == nil || *f.WorkspaceID != *skill.WorkspaceID {
			return false
		}
	}
	if len(f.Domains) > 0 && matches(skill.Domains, f.Domains) == 0 {
		return false
	}
	if len(f.Intents) > 0 && matches(skill.Intents, f.Intents) == 0 {
		return false
	}
	if len(f.ObjectTypes) > 0 && matches(skill.ObjectTypes, f.ObjectTypes) == 0 {
		return false
	}
	if len(f.Tags) > 0 && matches(skill.Tags, f.Tags) == 0 {
		return false
	}
	if f.RequiredTool != "" {
		ok := false
		for _, t := range skill.Tools {
			if strings.EqualFold(t.Name, f.RequiredTool) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func Candidates(skills []domain.Skill, f domain.SearchFilter, scorer Scorer, stats map[string]domain.Statistics) []domain.Candidate {
	var out []domain.Candidate
	for _, sk := range skills {
		if !Eligible(sk, f) {
			continue
		}
		var st *domain.Statistics
		if v, ok := stats[sk.ID]; ok {
			st = &v
		}
		out = append(out, domain.Candidate{ID: sk.ID, Slug: sk.Slug, Name: sk.Name, Description: sk.Description, Purpose: sk.Purpose, Domains: sk.Domains, Intents: sk.Intents, ObjectTypes: sk.ObjectTypes, Score: scorer.Score(sk, f, st), Version: sk.CurrentVersion})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Name < out[j].Name
		}
		return out[i].Score > out[j].Score
	})
	limit := f.Limit
	if limit <= 0 {
		limit = 5
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func matches(a, b []string) int {
	set := map[string]bool{}
	for _, v := range a {
		set[strings.ToLower(v)] = true
	}
	n := 0
	seen := map[string]bool{}
	for _, v := range b {
		k := strings.ToLower(v)
		if set[k] && !seen[k] {
			seen[k] = true
			n++
		}
	}
	return n
}
func tokenize(v string) []string {
	return strings.FieldsFunc(strings.ToLower(v), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' })
}
func containsScope(v []domain.Scope, x domain.Scope) bool {
	for _, item := range v {
		if item == x {
			return true
		}
	}
	return false
}
