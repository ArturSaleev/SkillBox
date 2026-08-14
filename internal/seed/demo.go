package seed

import (
	"context"

	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
)

func Demo(ctx context.Context, store ports.Storage) error {
	existing, err := store.ListSkills(ctx)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, s := range existing {
		seen[s.Slug] = true
	}
	for _, skill := range demoSkills() {
		if seen[skill.Slug] {
			continue
		}
		if err := store.CreateSkill(ctx, &skill, "demo seed", nil); err != nil {
			return err
		}
	}
	return nil
}
func demoSkills() []domain.Skill {
	return []domain.Skill{
		demo("laravel-create-api-endpoint", "Laravel API Endpoint", "Create a Laravel REST endpoint that follows the project's existing contract.", []string{"laravel", "backend"}, []string{"create"}, []string{"api_endpoint"}, []string{"routes", "controller", "api"}),
		demo("laravel-create-migration", "Laravel Migration", "Create a reversible Laravel database migration.", []string{"laravel", "mysql", "postgresql"}, []string{"create", "migrate"}, []string{"migration"}, []string{"schema", "rollback"}),
		demo("golang-create-http-handler", "Go HTTP Handler", "Add an HTTP handler using the service's established transport and application boundaries.", []string{"golang", "backend"}, []string{"create"}, []string{"api_endpoint"}, []string{"handler", "http", "test"}),
		demo("postgresql-analyze-query", "PostgreSQL Query Analysis", "Analyze a PostgreSQL query and propose evidence-based improvements.", []string{"postgresql"}, []string{"analyze", "optimize"}, []string{"database_query"}, []string{"explain", "index"}),
		demo("dockerize-application", "Dockerize Application", "Build a reproducible production container without leaking build secrets.", []string{"docker", "devops"}, []string{"configure", "deploy"}, []string{"docker_container"}, []string{"dockerfile", "healthcheck"}),
		demo("mcp-create-tool", "MCP Tool", "Add a compact MCP tool with a strict input schema and end-to-end protocol test.", []string{"mcp", "backend"}, []string{"create"}, []string{"mcp_tool"}, []string{"tools/list", "tools/call", "json-rpc"}),
	}
}
func demo(slug, name, purpose string, domains, intents, objects, keywords []string) domain.Skill {
	return domain.Skill{Slug: slug, Name: name, Description: purpose, Purpose: purpose, WhenToUse: "Use when the task metadata matches this procedure.", WhenNotToUse: "Do not use for unrelated domains or objects.", Instructions: "Inspect the real project contract before editing. Preserve established boundaries and keep the change scoped. Verify the result with the narrowest relevant tests and an operational check.", SuccessCriteria: []string{"The requested behavior matches the existing project contract", "Relevant automated tests pass", "No unrelated behavior is changed"}, Scope: domain.ScopeGlobal, Status: domain.StatusActive, Priority: 10, Domains: domains, Intents: intents, ObjectTypes: objects, Keywords: keywords, Tags: keywords, Steps: []domain.Step{{Position: 1, Title: "Inspect", Instruction: "Find the owning routes, types, and nearest working example.", Required: true}, {Position: 2, Title: "Implement", Instruction: "Apply the smallest cohesive change that satisfies the confirmed contract.", Required: true}, {Position: 3, Title: "Verify", Instruction: "Run focused tests and inspect the runtime-facing result.", Required: true}}, Tools: []domain.ToolRequirement{{Name: "search_files", Requirement: "required", Purpose: "Locate the owning contract"}, {Name: "read_file", Requirement: "required", Purpose: "Inspect source of truth"}, {Name: "write_file", Requirement: "required", Purpose: "Implement the change"}, {Name: "run_command", Requirement: "optional", Purpose: "Format and verify"}}, Contexts: []domain.ContextRequirement{{Type: "project_structure", Query: "Relevant project tree", Required: true, Priority: 100}, {Type: "source_code", Query: "Owning source and closest working example", Required: true, Priority: 100}, {Type: "documentation", Query: "Project conventions", Required: false, Priority: 20}}, Examples: []domain.Example{{Title: "Scoped execution", InputExample: "Add the requested capability in the current project.", ExpectedBehavior: "Inspect first, change the owning layer, then run focused verification.", Priority: 10}}}
}
