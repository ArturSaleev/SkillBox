package compiler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
)

var ErrDependencyCycle = errors.New("skill dependency cycle")

type Compiler struct{ store ports.Storage }

func New(store ports.Storage) *Compiler { return &Compiler{store: store} }

func (c *Compiler) Compile(ctx context.Context, root *domain.Skill, request domain.PrepareRequest) (domain.PreparedSkill, error) {
	ordered, err := c.resolve(ctx, root)
	if err != nil {
		return domain.PreparedSkill{}, err
	}
	compiled := domain.CompiledSkill{}
	available := set(request.AvailableTools)
	for _, skill := range ordered {
		if strings.TrimSpace(skill.Instructions) != "" {
			if compiled.Instructions != "" {
				compiled.Instructions += "\n"
			}
			compiled.Instructions += skill.Instructions
		}
		for _, step := range skill.Steps {
			compiled.Steps = append(compiled.Steps, domain.CompiledStep{Title: step.Title, Instruction: step.Instruction, Required: step.Required})
		}
		for _, tool := range skill.Tools {
			if tool.Requirement == "required" {
				compiled.RequiredTools = appendUniqueTool(compiled.RequiredTools, tool)
				if !available[strings.ToLower(tool.Name)] {
					compiled.MissingTools = appendUnique(compiled.MissingTools, tool.Name)
				}
			} else {
				compiled.OptionalTools = appendUniqueTool(compiled.OptionalTools, tool)
			}
		}
		compiled.ContextRequirements = appendUniqueContext(compiled.ContextRequirements, skill.Contexts...)
		compiled.SuccessCriteria = appendUnique(compiled.SuccessCriteria, skill.SuccessCriteria...)
	}
	if request.Model.ContextWindow <= 8192 && len(root.Examples) > 0 {
		examples := append([]domain.Example(nil), root.Examples...)
		sort.Slice(examples, func(i, j int) bool { return examples[i].Priority > examples[j].Priority })
		compiled.Examples = []domain.Example{examples[0]}
	}
	budget := request.MaxSkillTokens
	if budget <= 0 {
		budget = 800
	}
	if request.Model.ContextWindow > 0 && budget > request.Model.ContextWindow/3 {
		budget = request.Model.ContextWindow / 3
	}
	compact(&compiled, budget)
	return domain.PreparedSkill{SkillID: root.ID, Version: root.CurrentVersion, Name: root.Name, CompiledSkill: compiled, EstimatedTokens: estimate(compiled)}, nil
}

func (c *Compiler) resolve(ctx context.Context, root *domain.Skill) ([]*domain.Skill, error) {
	var out []*domain.Skill
	state := map[string]int{}
	var visit func(*domain.Skill) error
	visit = func(sk *domain.Skill) error {
		if state[sk.ID] == 1 {
			return fmt.Errorf("%w: %s", ErrDependencyCycle, sk.ID)
		}
		if state[sk.ID] == 2 {
			return nil
		}
		state[sk.ID] = 1
		deps := append([]domain.Dependency(nil), sk.Dependencies...)
		sort.SliceStable(deps, func(i, j int) bool { return deps[i].Position < deps[j].Position })
		for _, dep := range deps {
			if dep.Type == "fallback" {
				continue
			}
			child, err := c.store.GetSkill(ctx, dep.DependsOnSkillID)
			if err != nil {
				return fmt.Errorf("dependency %s: %w", dep.DependsOnSkillID, err)
			}
			if err = visit(child); err != nil {
				return err
			}
		}
		state[sk.ID] = 2
		out = append(out, sk)
		return nil
	}
	return out, visit(root)
}

func compact(c *domain.CompiledSkill, budget int) {
	for estimate(*c) > budget && len(c.Examples) > 0 {
		c.Examples = c.Examples[:len(c.Examples)-1]
	}
	for estimate(*c) > budget {
		idx := -1
		for i := len(c.ContextRequirements) - 1; i >= 0; i-- {
			if !c.ContextRequirements[i].Required {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		c.ContextRequirements = append(c.ContextRequirements[:idx], c.ContextRequirements[idx+1:]...)
	}
	for estimate(*c) > budget {
		idx := -1
		for i := len(c.Steps) - 1; i >= 0; i-- {
			if !c.Steps[i].Required {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		c.Steps = append(c.Steps[:idx], c.Steps[idx+1:]...)
	}
	if estimate(*c) > budget {
		c.Instructions = truncate(c.Instructions, max(80, budget*4/3))
	}
}
func estimate(c domain.CompiledSkill) int {
	n := utf8.RuneCountInString(c.Instructions)
	for _, v := range c.Steps {
		n += utf8.RuneCountInString(v.Title) + utf8.RuneCountInString(v.Instruction)
	}
	for _, v := range c.RequiredTools {
		n += utf8.RuneCountInString(v.Name) + utf8.RuneCountInString(v.Purpose)
	}
	for _, v := range c.OptionalTools {
		n += utf8.RuneCountInString(v.Name) + utf8.RuneCountInString(v.Purpose)
	}
	for _, v := range c.ContextRequirements {
		n += utf8.RuneCountInString(v.Type) + utf8.RuneCountInString(v.Query) + utf8.RuneCountInString(v.Description)
	}
	for _, v := range c.SuccessCriteria {
		n += utf8.RuneCountInString(v)
	}
	for _, v := range c.Examples {
		n += utf8.RuneCountInString(v.InputExample) + utf8.RuneCountInString(v.ExpectedBehavior)
	}
	return (n+3)/4 + 20
}
func truncate(v string, n int) string {
	r := []rune(v)
	if len(r) <= n {
		return v
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}
func set(v []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range v {
		m[strings.ToLower(x)] = true
	}
	return m
}
func appendUnique(dst []string, values ...string) []string {
	m := set(dst)
	for _, v := range values {
		k := strings.ToLower(v)
		if v != "" && !m[k] {
			m[k] = true
			dst = append(dst, v)
		}
	}
	return dst
}
func appendUniqueTool(dst []domain.ToolRequirement, v domain.ToolRequirement) []domain.ToolRequirement {
	for _, x := range dst {
		if strings.EqualFold(x.Name, v.Name) && strings.EqualFold(x.Namespace, v.Namespace) {
			return dst
		}
	}
	return append(dst, v)
}
func appendUniqueContext(dst []domain.ContextRequirement, values ...domain.ContextRequirement) []domain.ContextRequirement {
	for _, v := range values {
		found := false
		for _, x := range dst {
			if x.Type == v.Type && x.Query == v.Query {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, v)
		}
	}
	sort.SliceStable(dst, func(i, j int) bool { return dst[i].Priority > dst[j].Priority })
	return dst
}
