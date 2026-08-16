package domain

import "testing"

func TestSkillValidationEnforcesScopeAndPortableEnums(t *testing.T) {
	t.Parallel()
	sk := Skill{Slug: "x", Name: "X", Scope: ScopeGlobal, Status: StatusActive}
	if err := sk.Validate(); err != nil {
		t.Fatal(err)
	}
	ws := "w"
	sk.WorkspaceID = &ws
	if err := sk.Validate(); err == nil {
		t.Fatal("expected global scope ownership error")
	}
	sk.Scope = Scope("tenant")
	sk.WorkspaceID = nil
	if err := sk.Validate(); err == nil {
		t.Fatal("expected invalid scope error")
	}
}
