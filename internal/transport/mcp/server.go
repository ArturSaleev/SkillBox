package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/aibox/skillbox/internal/application"
	"github.com/aibox/skillbox/internal/auth"
	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/metrics"
	"github.com/aibox/skillbox/internal/ports"
	"github.com/go-chi/chi/v5"
)

type AccessResolver interface {
	Resolve(*http.Request) (*domain.MCPAccess, error)
}
type ConnectionResolver struct{ store ports.Storage }

func NewConnectionResolver(store ports.Storage) *ConnectionResolver {
	return &ConnectionResolver{store: store}
}
func (r *ConnectionResolver) Resolve(req *http.Request) (*domain.MCPAccess, error) {
	key := strings.TrimSpace(req.Header.Get("X-SkillBox-Key"))
	if key == "" {
		key = strings.TrimSpace(req.Header.Get("X-API-Key"))
	}
	if key == "" {
		if h := req.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
			key = strings.TrimSpace(h[7:])
		}
	}
	if key == "" {
		return nil, errors.New("MCP connection credential is required")
	}
	connection, err := r.store.ResolveMCPConnection(req.Context(), auth.HashAPIKey(key))
	if err != nil {
		return nil, errors.New("invalid MCP connection credential")
	}
	if hint := chi.URLParam(req, "connection"); hint != "" && hint != connection.ID && hint != connection.Slug {
		return nil, errors.New("credential does not match MCP connection path")
	}
	profile, err := r.store.GetMCPProfile(req.Context(), connection.ProfileID)
	if err != nil || !profile.Enabled {
		return nil, errors.New("MCP profile is unavailable")
	}
	_ = r.store.TouchMCPConnection(req.Context(), connection.ID)
	return &domain.MCPAccess{Connection: *connection, Profile: *profile}, nil
}

type Server struct {
	app      *application.Service
	metrics  *metrics.Metrics
	logger   *slog.Logger
	resolver AccessResolver
}

func New(app *application.Service, m *metrics.Metrics, logger *slog.Logger, resolver AccessResolver) *Server {
	return &Server{app: app, metrics: m, logger: logger, resolver: resolver}
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}
type response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
type toolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", 405)
		return
	}
	access, err := s.resolver.Resolve(r)
	if err != nil {
		s.reply(w, response{JSONRPC: "2.0", Error: &rpcError{Code: -32001, Message: err.Error()}})
		return
	}
	var req request
	if err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req); err != nil {
		s.reply(w, response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
		return
	}
	start := time.Now()
	status := "ok"
	defer func() {
		s.metrics.MCPRequests.WithLabelValues(access.Profile.Slug, req.Method, status).Inc()
		s.logger.InfoContext(r.Context(), "mcp request", "connection_id", access.Connection.ID, "profile", access.Profile.Slug, "method", req.Method, "status", status, "duration_ms", time.Since(start).Milliseconds())
	}()
	if req.Method == "notifications/initialized" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	result, err := s.handle(r, *access, req)
	if err != nil {
		status = "error"
		code := -32603
		if errors.Is(err, ports.ErrNotFound) {
			code = -32004
		}
		if errors.Is(err, errForbidden) {
			code = -32003
		}
		s.reply(w, response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: code, Message: err.Error()}})
		return
	}
	s.reply(w, response{JSONRPC: "2.0", ID: req.ID, Result: result})
}

func (s *Server) handle(r *http.Request, access domain.MCPAccess, req request) (any, error) {
	switch req.Method {
	case "initialize":
		return map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{"listChanged": false}}, "serverInfo": map[string]string{"name": "SkillBox/" + access.Profile.Slug, "version": "0.2.0"}}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": allowedDefinitions(access.Profile)}, nil
	case "tools/call":
		var call toolCall
		if err := json.Unmarshal(req.Params, &call); err != nil {
			return nil, err
		}
		if !allows(access.Profile, call.Name) {
			return nil, fmt.Errorf("%w: tool %s", errForbidden, call.Name)
		}
		return s.callTool(r, access, call)
	default:
		return nil, fmt.Errorf("method not found: %s", req.Method)
	}
}

var errForbidden = errors.New("forbidden")

func allows(profile domain.MCPProfile, tool string) bool {
	permission, ok := domain.ToolPermission(tool)
	return ok && profile.AllowsTool(tool) && profile.Allows(permission)
}

func (s *Server) callTool(r *http.Request, access domain.MCPAccess, call toolCall) (any, error) {
	ctx := r.Context()
	var result any
	switch call.Name {
	case domain.ToolSearchSkills:
		var v domain.SearchFilter
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		application.ApplyScope(access, &v)
		if !access.Profile.Allows(domain.PermissionSkillRead) {
			v.Status = string(domain.StatusActive)
		}
		start := time.Now()
		items, err := s.app.Search(ctx, v)
		s.metrics.SearchDuration.Observe(time.Since(start).Seconds())
		if err != nil {
			return nil, err
		}
		result = map[string]any{"skills": items}
	case domain.ToolGetSkill:
		var v idInput
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		item, err := s.app.GetVisibleSkill(ctx, access, v.SkillID)
		if err != nil {
			return nil, err
		}
		result = item
	case domain.ToolPrepareSkill:
		var v domain.PrepareRequest
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		application.ApplyPrepareScope(access, &v)
		if v.SkillID != nil {
			skill, err := s.app.GetVisibleSkill(ctx, access, *v.SkillID)
			if err != nil {
				return nil, err
			}
			if !access.Profile.Allows(domain.PermissionSkillRead) && skill.Status != domain.StatusActive {
				return nil, ports.ErrNotFound
			}
		}
		start := time.Now()
		item, err := s.app.Prepare(ctx, v)
		s.metrics.CompileDuration.Observe(time.Since(start).Seconds())
		if err != nil {
			return nil, err
		}
		result = item
	case domain.ToolReportSkillResult:
		var v domain.Execution
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		if _, err := s.app.GetVisibleSkill(ctx, access, v.SkillID); err != nil {
			return nil, err
		}
		v.WorkspaceID = access.Connection.WorkspaceID
		v.ProjectID = access.Connection.ProjectID
		agent := access.Connection.ID
		v.AgentID = &agent
		if err := s.app.Store.CreateExecution(ctx, &v); err != nil {
			return nil, err
		}
		s.metrics.Executions.Inc()
		if v.Success {
			s.metrics.ExecutionSuccess.Inc()
		}
		result = map[string]any{"execution_id": v.ID, "accepted": true}
	case domain.ToolCreateSkillDraft:
		var v domain.Skill
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		item, err := s.app.CreateDraft(ctx, access, &v)
		if err != nil {
			return nil, err
		}
		result = item
	case domain.ToolUpdateSkillDraft, domain.ToolCreateSkillVersion:
		var v struct {
			Skill         domain.Skill `json:"skill"`
			ChangeSummary string       `json:"change_summary"`
		}
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		item, err := s.app.UpdateDraft(ctx, access, &v.Skill, v.ChangeSummary)
		if err != nil {
			return nil, err
		}
		result = item
	case domain.ToolValidateSkill:
		var v idInput
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		issues, err := s.app.ValidateVisibleSkill(ctx, access, v.SkillID)
		if err != nil {
			return nil, err
		}
		result = map[string]any{"valid": len(issues) == 0, "issues": issues}
	case domain.ToolCreateSkillProposal:
		var v struct {
			SkillID string `json:"skill_id"`
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		item, err := s.app.CreateProposal(ctx, access, v.SkillID, v.Summary)
		if err != nil {
			return nil, err
		}
		result = item
	case domain.ToolGetSkillStatistics:
		var v idInput
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		if _, err := s.app.GetVisibleSkill(ctx, access, v.SkillID); err != nil {
			return nil, err
		}
		items, err := s.app.Store.Statistics(ctx, &v.SkillID)
		if err != nil {
			return nil, err
		}
		result = map[string]any{"statistics": items}
	case domain.ToolListRecentExecutions, domain.ToolGetSkillSuccesses, domain.ToolGetSkillFailures:
		var v struct {
			SkillID *string `json:"skill_id"`
			Limit   int     `json:"limit"`
		}
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		status := ""
		if call.Name == domain.ToolGetSkillSuccesses {
			status = "success"
		}
		if call.Name == domain.ToolGetSkillFailures {
			status = "failed"
		}
		items, err := s.app.ScopedExecutions(ctx, access, v.SkillID, status, v.Limit)
		if err != nil {
			return nil, err
		}
		result = map[string]any{"executions": items}
	case domain.ToolGetExecution:
		var v struct {
			ExecutionID string `json:"execution_id"`
		}
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		item, err := s.app.Store.GetExecution(ctx, v.ExecutionID)
		if err != nil {
			return nil, err
		}
		if _, err = s.app.GetVisibleSkill(ctx, access, item.SkillID); err != nil {
			return nil, err
		}
		item.Trajectory = nil
		result = item
	case domain.ToolGetExecutionTrajectory:
		var v struct {
			ExecutionID string `json:"execution_id"`
		}
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		item, err := s.app.Store.GetExecution(ctx, v.ExecutionID)
		if err != nil {
			return nil, err
		}
		if _, err = s.app.GetVisibleSkill(ctx, access, item.SkillID); err != nil {
			return nil, err
		}
		result = map[string]any{"execution_id": v.ExecutionID, "trajectory": item.Trajectory}
	case domain.ToolGetSkillProposal:
		var v struct {
			ProposalID string `json:"proposal_id"`
		}
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		p, err := s.app.Store.GetSkillProposal(ctx, v.ProposalID)
		if err != nil {
			return nil, err
		}
		if _, err = s.app.GetVisibleSkill(ctx, access, p.SkillID); err != nil {
			return nil, err
		}
		result = p
	case domain.ToolListSkillProposals:
		var v struct {
			SkillID *string `json:"skill_id"`
			Status  string  `json:"status"`
		}
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		items, err := s.app.Store.ListSkillProposals(ctx, v.SkillID, v.Status)
		if err != nil {
			return nil, err
		}
		visible := items[:0]
		for _, p := range items {
			if _, e := s.app.GetVisibleSkill(ctx, access, p.SkillID); e == nil {
				visible = append(visible, p)
			}
		}
		result = map[string]any{"proposals": visible}
	case domain.ToolApproveSkillProposal, domain.ToolRejectSkillProposal:
		var v struct {
			ProposalID string  `json:"proposal_id"`
			Note       *string `json:"note"`
		}
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		p, err := s.app.Store.GetSkillProposal(ctx, v.ProposalID)
		if err != nil {
			return nil, err
		}
		if _, err = s.app.GetVisibleSkill(ctx, access, p.SkillID); err != nil {
			return nil, err
		}
		status := "approved"
		if call.Name == domain.ToolRejectSkillProposal {
			status = "rejected"
		}
		actor := access.Connection.ID
		result, err = s.app.Store.ReviewSkillProposal(ctx, v.ProposalID, status, &actor, v.Note)
		if err != nil {
			return nil, err
		}
	case domain.ToolPublishSkill:
		var v struct {
			ProposalID string `json:"proposal_id"`
		}
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		item, err := s.app.PublishProposal(ctx, access, v.ProposalID)
		if err != nil {
			return nil, err
		}
		result = item
	case domain.ToolRollbackSkillVersion:
		var v struct {
			SkillID string `json:"skill_id"`
			Version int    `json:"version"`
		}
		if err := json.Unmarshal(call.Arguments, &v); err != nil {
			return nil, err
		}
		if _, err := s.app.GetVisibleSkill(ctx, access, v.SkillID); err != nil {
			return nil, err
		}
		actor := access.Connection.ID
		item, err := s.app.Store.RollbackSkill(ctx, v.SkillID, v.Version, &actor)
		if err != nil {
			return nil, err
		}
		result = item
	default:
		return nil, fmt.Errorf("tool not implemented: %s", call.Name)
	}
	raw, _ := json.Marshal(result)
	return map[string]any{"content": []map[string]string{{"type": "text", "text": string(raw)}}, "structuredContent": result, "isError": false}, nil
}

type idInput struct {
	SkillID string `json:"skill_id"`
}
type toolDefinition struct {
	Name, Description, Permission string
	Schema                        map[string]any
}

var definitions = map[string]toolDefinition{
	domain.ToolSearchSkills:           {domain.ToolSearchSkills, "Find compact Skill candidates.", domain.PermissionSkillSearch, obj(map[string]any{"task": str(), "domains": stringsSchema(), "intents": stringsSchema(), "object_types": stringsSchema(), "status": str(), "limit": integer()}, "task")},
	domain.ToolGetSkill:               {domain.ToolGetSkill, "Get one Skill by ID.", domain.PermissionSkillRead, obj(map[string]any{"skill_id": str()}, "skill_id")},
	domain.ToolPrepareSkill:           {domain.ToolPrepareSkill, "Compile the best Skill for this task.", domain.PermissionSkillPrepare, obj(map[string]any{"task": str(), "skill_id": str(), "domains": stringsSchema(), "intents": stringsSchema(), "available_tools": stringsSchema(), "model": map[string]any{"type": "object"}, "max_skill_tokens": integer()}, "task")},
	domain.ToolReportSkillResult:      {domain.ToolReportSkillResult, "Report Skill execution telemetry.", domain.PermissionExecutionReport, obj(map[string]any{"skill_id": str(), "skill_version": integer(), "task_summary": str(), "status": str(), "success": map[string]any{"type": "boolean"}, "trajectory": map[string]any{"type": "array"}}, "skill_id", "skill_version", "task_summary", "status", "success")},
	domain.ToolCreateSkillDraft:       {domain.ToolCreateSkillDraft, "Create a scoped draft Skill.", domain.PermissionSkillCreate, obj(map[string]any{"slug": str(), "name": str(), "description": str(), "purpose": str(), "when_to_use": str(), "instructions": str(), "success_criteria": stringsSchema(), "domains": stringsSchema(), "intents": stringsSchema(), "steps": map[string]any{"type": "array"}, "tools": map[string]any{"type": "array"}, "context_requirements": map[string]any{"type": "array"}}, "slug", "name")},
	domain.ToolUpdateSkillDraft:       {domain.ToolUpdateSkillDraft, "Update an existing draft.", domain.PermissionSkillUpdate, obj(map[string]any{"skill": map[string]any{"type": "object"}, "change_summary": str()}, "skill")},
	domain.ToolCreateSkillVersion:     {domain.ToolCreateSkillVersion, "Create a new immutable draft version.", domain.PermissionSkillVersionCreate, obj(map[string]any{"skill": map[string]any{"type": "object"}, "change_summary": str()}, "skill")},
	domain.ToolValidateSkill:          {domain.ToolValidateSkill, "Validate a Skill and dependency graph.", domain.PermissionSkillValidate, obj(map[string]any{"skill_id": str()}, "skill_id")},
	domain.ToolCreateSkillProposal:    {domain.ToolCreateSkillProposal, "Submit a draft for review.", domain.PermissionSkillPropose, obj(map[string]any{"skill_id": str(), "summary": str()}, "skill_id")},
	domain.ToolGetSkillStatistics:     {domain.ToolGetSkillStatistics, "Get Skill success statistics by model.", domain.PermissionStatisticsRead, obj(map[string]any{"skill_id": str()}, "skill_id")},
	domain.ToolListRecentExecutions:   {domain.ToolListRecentExecutions, "List recent scoped executions.", domain.PermissionExecutionRead, obj(map[string]any{"skill_id": str(), "limit": integer()})},
	domain.ToolGetExecution:           {domain.ToolGetExecution, "Get one scoped execution.", domain.PermissionExecutionRead, obj(map[string]any{"execution_id": str()}, "execution_id")},
	domain.ToolGetExecutionTrajectory: {domain.ToolGetExecutionTrajectory, "Get the execution trajectory.", domain.PermissionExecutionTrajectoryRead, obj(map[string]any{"execution_id": str()}, "execution_id")},
	domain.ToolGetSkillSuccesses:      {domain.ToolGetSkillSuccesses, "List successful executions.", domain.PermissionExecutionRead, obj(map[string]any{"skill_id": str(), "limit": integer()})},
	domain.ToolGetSkillFailures:       {domain.ToolGetSkillFailures, "List failed executions.", domain.PermissionExecutionRead, obj(map[string]any{"skill_id": str(), "limit": integer()})},
	domain.ToolGetSkillProposal:       {domain.ToolGetSkillProposal, "Get one proposal.", domain.PermissionSkillRead, obj(map[string]any{"proposal_id": str()}, "proposal_id")},
	domain.ToolListSkillProposals:     {domain.ToolListSkillProposals, "List scoped proposals.", domain.PermissionSkillRead, obj(map[string]any{"skill_id": str(), "status": str()})},
	domain.ToolApproveSkillProposal:   {domain.ToolApproveSkillProposal, "Approve a pending proposal.", domain.PermissionSkillPublish, obj(map[string]any{"proposal_id": str(), "note": str()}, "proposal_id")},
	domain.ToolRejectSkillProposal:    {domain.ToolRejectSkillProposal, "Reject a pending proposal.", domain.PermissionSkillPublish, obj(map[string]any{"proposal_id": str(), "note": str()}, "proposal_id")},
	domain.ToolPublishSkill:           {domain.ToolPublishSkill, "Publish an approved proposal.", domain.PermissionSkillPublish, obj(map[string]any{"proposal_id": str()}, "proposal_id")},
	domain.ToolRollbackSkillVersion:   {domain.ToolRollbackSkillVersion, "Rollback a Skill as a new version.", domain.PermissionSkillRollback, obj(map[string]any{"skill_id": str(), "version": integer()}, "skill_id", "version")},
}

func allowedDefinitions(profile domain.MCPProfile) []map[string]any {
	out := make([]map[string]any, 0, len(profile.Tools))
	for _, name := range profile.Tools {
		d, ok := definitions[name]
		if !ok || !profile.Allows(d.Permission) {
			continue
		}
		out = append(out, map[string]any{"name": d.Name, "description": d.Description, "inputSchema": d.Schema})
	}
	return out
}
func obj(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func str() map[string]any     { return map[string]any{"type": "string"} }
func integer() map[string]any { return map[string]any{"type": "integer"} }
func stringsSchema() map[string]any {
	return map[string]any{"type": "array", "items": map[string]string{"type": "string"}}
}
func (s *Server) reply(w http.ResponseWriter, v response) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
