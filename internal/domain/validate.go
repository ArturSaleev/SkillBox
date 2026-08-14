package domain

import (
	"errors"
	"fmt"
	"strings"
)

func (s *Skill) Validate() error {
	if strings.TrimSpace(s.Slug) == "" || strings.TrimSpace(s.Name) == "" {
		return errors.New("slug and name are required")
	}
	if !validScope(s.Scope) {
		return fmt.Errorf("invalid scope %q", s.Scope)
	}
	if !validStatus(s.Status) {
		return fmt.Errorf("invalid status %q", s.Status)
	}
	if s.Scope == ScopeGlobal && (s.WorkspaceID != nil || s.ProjectID != nil) {
		return errors.New("global skill cannot belong to a workspace or project")
	}
	if s.Scope == ScopeWorkspace && s.WorkspaceID == nil {
		return errors.New("workspace skill requires workspace_id")
	}
	if s.Scope == ScopeProject && (s.WorkspaceID == nil || s.ProjectID == nil) {
		return errors.New("project skill requires workspace_id and project_id")
	}
	if s.Scope == ScopeUser && s.OwnerUserID == nil {
		return errors.New("user skill requires owner_user_id")
	}
	if s.Scope != ScopeUser && s.OwnerUserID != nil {
		return errors.New("owner_user_id is only valid for user skills")
	}
	for _, tool := range s.Tools {
		if tool.Requirement != "required" && tool.Requirement != "optional" {
			return fmt.Errorf("invalid tool requirement %q", tool.Requirement)
		}
	}
	for _, dep := range s.Dependencies {
		switch dep.Type {
		case "requires", "extends", "uses", "fallback":
		default:
			return fmt.Errorf("invalid dependency type %q", dep.Type)
		}
		if dep.DependsOnSkillID == s.ID && s.ID != "" {
			return errors.New("skill cannot depend on itself")
		}
	}
	return nil
}

func validScope(scope Scope) bool {
	switch scope {
	case ScopeGlobal, ScopeWorkspace, ScopeProject, ScopeUser:
		return true
	default:
		return false
	}
}

func validStatus(status SkillStatus) bool {
	switch status {
	case StatusDraft, StatusActive, StatusDeprecated, StatusArchived:
		return true
	default:
		return false
	}
}
