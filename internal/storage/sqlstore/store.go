package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
	"github.com/google/uuid"
)

type Store struct {
	db      *sql.DB
	dialect string
}

func New(db *sql.DB, dialect string) *Store     { return &Store{db: db, dialect: dialect} }
func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) q(query string) string {
	if s.dialect != "postgres" {
		return query
	}
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func now() time.Time               { return time.Now().UTC() }
func ts(t time.Time) string        { return t.UTC().Format(time.RFC3339Nano) }
func parseTime(v string) time.Time { t, _ := time.Parse(time.RFC3339Nano, v); return t }

func (s *Store) CreateWorkspace(ctx context.Context, w *domain.Workspace) error {
	if w.ID == "" {
		w.ID = uuid.NewString()
	}
	if w.CreatedAt.IsZero() {
		w.CreatedAt = now()
	}
	w.UpdatedAt = w.CreatedAt
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO workspaces(id,slug,name,description,created_at,updated_at) VALUES(?,?,?,?,?,?)`), w.ID, w.Slug, w.Name, w.Description, ts(w.CreatedAt), ts(w.UpdatedAt))
	return err
}

func (s *Store) GetWorkspace(ctx context.Context, idOrSlug string) (*domain.Workspace, error) {
	var w domain.Workspace
	var created, updated string
	err := s.db.QueryRowContext(ctx, s.q(`SELECT id,slug,name,description,created_at,updated_at FROM workspaces WHERE id=? OR slug=?`), idOrSlug, idOrSlug).Scan(&w.ID, &w.Slug, &w.Name, &w.Description, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	w.CreatedAt, w.UpdatedAt = parseTime(created), parseTime(updated)
	return &w, nil
}

func (s *Store) EnsureWorkspace(ctx context.Context, slug, name string) (*domain.Workspace, error) {
	w := domain.Workspace{ID: uuid.NewString(), Slug: slug, Name: name, CreatedAt: now()}
	w.UpdatedAt = w.CreatedAt
	query := `INSERT INTO workspaces(id,slug,name,description,created_at,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(slug) DO NOTHING`
	if s.dialect == "mysql" {
		query = `INSERT IGNORE INTO workspaces(id,slug,name,description,created_at,updated_at) VALUES(?,?,?,?,?,?)`
	}
	if _, err := s.db.ExecContext(ctx, s.q(query), w.ID, w.Slug, w.Name, w.Description, ts(w.CreatedAt), ts(w.UpdatedAt)); err != nil {
		return nil, err
	}
	return s.GetWorkspace(ctx, slug)
}

func (s *Store) ListWorkspaces(ctx context.Context) ([]domain.Workspace, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,slug,name,description,created_at,updated_at FROM workspaces ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Workspace
	for rows.Next() {
		var w domain.Workspace
		var created, updated string
		if err := rows.Scan(&w.ID, &w.Slug, &w.Name, &w.Description, &created, &updated); err != nil {
			return nil, err
		}
		w.CreatedAt, w.UpdatedAt = parseTime(created), parseTime(updated)
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) CreateProject(ctx context.Context, p *domain.Project) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now()
	}
	p.UpdatedAt = p.CreatedAt
	if p.ExternalID == "" {
		p.ExternalID = p.Slug
	}
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO projects(id,workspace_id,external_id,slug,name,description,auto_created,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`), p.ID, p.WorkspaceID, p.ExternalID, p.Slug, p.Name, p.Description, p.AutoCreated, ts(p.CreatedAt), ts(p.UpdatedAt))
	return err
}

func (s *Store) GetProject(ctx context.Context, idOrSlug string, workspaceID *string) (*domain.Project, error) {
	query := `SELECT p.id,p.workspace_id,p.external_id,p.slug,p.name,p.description,p.auto_created,p.created_at,p.updated_at,w.id,w.slug,w.name,w.description,w.created_at,w.updated_at FROM projects p JOIN workspaces w ON w.id=p.workspace_id WHERE (p.id=? OR p.slug=? OR p.external_id=?)`
	args := []any{idOrSlug, idOrSlug, idOrSlug}
	if workspaceID != nil {
		query += ` AND p.workspace_id=?`
		args = append(args, *workspaceID)
	}
	query += ` ORDER BY CASE WHEN p.id=? THEN 0 ELSE 1 END LIMIT 1`
	args = append(args, idOrSlug)
	var p domain.Project
	var workspace domain.Workspace
	var externalID sql.NullString
	var c, u, wc, wu string
	err := s.db.QueryRowContext(ctx, s.q(query), args...).Scan(&p.ID, &p.WorkspaceID, &externalID, &p.Slug, &p.Name, &p.Description, &p.AutoCreated, &c, &u, &workspace.ID, &workspace.Slug, &workspace.Name, &workspace.Description, &wc, &wu)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt, p.UpdatedAt = parseTime(c), parseTime(u)
	p.ExternalID = externalID.String
	workspace.CreatedAt, workspace.UpdatedAt = parseTime(wc), parseTime(wu)
	p.Workspace = &workspace
	return &p, nil
}

func (s *Store) EnsureProject(ctx context.Context, workspaceID, externalID, name string) (*domain.Project, error) {
	p := domain.Project{ID: uuid.NewString(), WorkspaceID: workspaceID, ExternalID: externalID, Slug: externalID, Name: name, AutoCreated: true, CreatedAt: now()}
	p.UpdatedAt = p.CreatedAt
	query := `INSERT INTO projects(id,workspace_id,external_id,slug,name,description,auto_created,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(workspace_id,slug) DO NOTHING`
	if s.dialect == "mysql" {
		query = `INSERT IGNORE INTO projects(id,workspace_id,external_id,slug,name,description,auto_created,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`
	}
	if _, err := s.db.ExecContext(ctx, s.q(query), p.ID, p.WorkspaceID, p.ExternalID, p.Slug, p.Name, p.Description, p.AutoCreated, ts(p.CreatedAt), ts(p.UpdatedAt)); err != nil {
		return nil, err
	}
	return s.GetProject(ctx, externalID, &workspaceID)
}

func (s *Store) ListProjects(ctx context.Context, workspaceID *string) ([]domain.Project, error) {
	query := `SELECT p.id,p.workspace_id,p.external_id,p.slug,p.name,p.description,p.auto_created,p.created_at,p.updated_at,w.id,w.slug,w.name,w.description,w.created_at,w.updated_at FROM projects p JOIN workspaces w ON w.id=p.workspace_id`
	var args []any
	if workspaceID != nil {
		query += ` WHERE p.workspace_id=?`
		args = append(args, *workspaceID)
	}
	query += ` ORDER BY p.name`
	rows, err := s.db.QueryContext(ctx, s.q(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Project, 0)
	for rows.Next() {
		var p domain.Project
		var workspace domain.Workspace
		var externalID sql.NullString
		var c, u, wc, wu string
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &externalID, &p.Slug, &p.Name, &p.Description, &p.AutoCreated, &c, &u, &workspace.ID, &workspace.Slug, &workspace.Name, &workspace.Description, &wc, &wu); err != nil {
			return nil, err
		}
		p.CreatedAt, p.UpdatedAt = parseTime(c), parseTime(u)
		p.ExternalID = externalID.String
		workspace.CreatedAt, workspace.UpdatedAt = parseTime(wc), parseTime(wu)
		p.Workspace = &workspace
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreateSkill(ctx context.Context, skill *domain.Skill, summary string, actor *string) error {
	if skill.ID == "" {
		skill.ID = uuid.NewString()
	}
	if skill.CurrentVersion == 0 {
		skill.CurrentVersion = 1
	}
	if skill.CreatedAt.IsZero() {
		skill.CreatedAt = now()
	}
	skill.UpdatedAt = skill.CreatedAt
	if err := skill.Validate(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.writeSkill(ctx, tx, skill, false); err != nil {
		return err
	}
	if err = s.writeRelations(ctx, tx, skill); err != nil {
		return err
	}
	if err = s.snapshot(ctx, tx, skill, summary, actor); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UpdateSkill(ctx context.Context, skill *domain.Skill, summary string, actor *string) error {
	if err := skill.Validate(); err != nil {
		return err
	}
	existing, err := s.GetSkill(ctx, skill.ID)
	if err != nil {
		return err
	}
	skill.CreatedAt = existing.CreatedAt
	skill.CurrentVersion = existing.CurrentVersion + 1
	skill.UpdatedAt = now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.writeSkill(ctx, tx, skill, true); err != nil {
		return err
	}
	if err = s.clearRelations(ctx, tx, skill.ID); err != nil {
		return err
	}
	if err = s.writeRelations(ctx, tx, skill); err != nil {
		return err
	}
	if err = s.snapshot(ctx, tx, skill, summary, actor); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) writeSkill(ctx context.Context, tx *sql.Tx, sk *domain.Skill, update bool) error {
	criteria, _ := json.Marshal(sk.SuccessCriteria)
	args := []any{sk.WorkspaceID, sk.ProjectID, sk.Slug, sk.Name, sk.Description, sk.Purpose, sk.WhenToUse, sk.WhenNotToUse, sk.Instructions, string(criteria), sk.Scope, sk.Status, sk.Priority, sk.CurrentVersion, ts(sk.UpdatedAt)}
	if update {
		args = append(args, sk.ID)
		res, err := tx.ExecContext(ctx, s.q(`UPDATE skills SET workspace_id=?,project_id=?,slug=?,name=?,description=?,purpose=?,when_to_use=?,when_not_to_use=?,instructions=?,success_criteria=?,scope=?,status=?,priority=?,current_version=?,updated_at=? WHERE id=?`), args...)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return ports.ErrNotFound
		}
		return nil
	}
	args = []any{sk.ID, sk.WorkspaceID, sk.ProjectID, sk.Slug, sk.Name, sk.Description, sk.Purpose, sk.WhenToUse, sk.WhenNotToUse, sk.Instructions, string(criteria), sk.Scope, sk.Status, sk.Priority, sk.CurrentVersion, ts(sk.CreatedAt), ts(sk.UpdatedAt)}
	_, err := tx.ExecContext(ctx, s.q(`INSERT INTO skills(id,workspace_id,project_id,slug,name,description,purpose,when_to_use,when_not_to_use,instructions,success_criteria,scope,status,priority,current_version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), args...)
	return err
}

func (s *Store) writeRelations(ctx context.Context, tx *sql.Tx, sk *domain.Skill) error {
	sets := []struct {
		table  string
		values []string
	}{{"skill_domains", sk.Domains}, {"skill_intents", sk.Intents}, {"skill_object_types", sk.ObjectTypes}, {"skill_tags", sk.Tags}, {"skill_keywords", sk.Keywords}, {"skill_capabilities", sk.Capabilities}, {"skill_compatibility", sk.Compatibility}}
	for _, set := range sets {
		for _, v := range unique(set.values) {
			if strings.TrimSpace(v) == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, s.q(`INSERT INTO `+set.table+`(skill_id,value) VALUES(?,?)`), sk.ID, strings.ToLower(strings.TrimSpace(v))); err != nil {
				return err
			}
		}
	}
	for i := range sk.Steps {
		v := &sk.Steps[i]
		if v.ID == "" {
			v.ID = uuid.NewString()
		}
		v.SkillID = sk.ID
		if v.CreatedAt.IsZero() {
			v.CreatedAt = sk.UpdatedAt
		}
		v.UpdatedAt = sk.UpdatedAt
		_, err := tx.ExecContext(ctx, s.q(`INSERT INTO skill_steps(id,skill_id,position,title,instruction,condition_text,is_required,expected_result,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`), v.ID, sk.ID, v.Position, v.Title, v.Instruction, nullString(v.Condition), v.Required, nullString(v.ExpectedResult), ts(v.CreatedAt), ts(v.UpdatedAt))
		if err != nil {
			return err
		}
	}
	for i := range sk.Tools {
		v := &sk.Tools[i]
		if v.ID == "" {
			v.ID = uuid.NewString()
		}
		v.SkillID = sk.ID
		_, err := tx.ExecContext(ctx, s.q(`INSERT INTO skill_tools(id,skill_id,tool_name,tool_namespace,requirement,purpose,usage_hint) VALUES(?,?,?,?,?,?,?)`), v.ID, sk.ID, v.Name, nullString(v.Namespace), v.Requirement, v.Purpose, nullString(v.UsageHint))
		if err != nil {
			return err
		}
	}
	for i := range sk.Contexts {
		v := &sk.Contexts[i]
		if v.ID == "" {
			v.ID = uuid.NewString()
		}
		v.SkillID = sk.ID
		_, err := tx.ExecContext(ctx, s.q(`INSERT INTO skill_context_requirements(id,skill_id,type,query_text,description,required,priority,max_tokens) VALUES(?,?,?,?,?,?,?,?)`), v.ID, sk.ID, v.Type, v.Query, v.Description, v.Required, v.Priority, v.MaxTokens)
		if err != nil {
			return err
		}
	}
	for i := range sk.Dependencies {
		v := &sk.Dependencies[i]
		if v.ID == "" {
			v.ID = uuid.NewString()
		}
		v.SkillID = sk.ID
		_, err := tx.ExecContext(ctx, s.q(`INSERT INTO skill_dependencies(id,skill_id,depends_on_skill_id,type,position) VALUES(?,?,?,?,?)`), v.ID, sk.ID, v.DependsOnSkillID, v.Type, v.Position)
		if err != nil {
			return err
		}
	}
	for i := range sk.Examples {
		v := &sk.Examples[i]
		if v.ID == "" {
			v.ID = uuid.NewString()
		}
		v.SkillID = sk.ID
		_, err := tx.ExecContext(ctx, s.q(`INSERT INTO skill_examples(id,skill_id,title,input_example,expected_behavior,bad_behavior,priority) VALUES(?,?,?,?,?,?,?)`), v.ID, sk.ID, v.Title, v.InputExample, v.ExpectedBehavior, nullString(v.BadBehavior), v.Priority)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) clearRelations(ctx context.Context, tx *sql.Tx, id string) error {
	for _, table := range []string{"skill_steps", "skill_tools", "skill_context_requirements", "skill_dependencies", "skill_examples", "skill_domains", "skill_intents", "skill_object_types", "skill_tags", "skill_keywords", "skill_capabilities", "skill_compatibility"} {
		if _, err := tx.ExecContext(ctx, s.q(`DELETE FROM `+table+` WHERE skill_id=?`), id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) snapshot(ctx context.Context, tx *sql.Tx, sk *domain.Skill, summary string, actor *string) error {
	raw, err := json.Marshal(sk)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, s.q(`INSERT INTO skill_versions(id,skill_id,version,snapshot,change_summary,created_by,created_at) VALUES(?,?,?,?,?,?,?)`), uuid.NewString(), sk.ID, sk.CurrentVersion, string(raw), summary, actor, ts(now()))
	return err
}

func (s *Store) GetSkill(ctx context.Context, id string) (*domain.Skill, error) {
	row := s.db.QueryRowContext(ctx, s.q(`SELECT id,workspace_id,project_id,slug,name,description,purpose,when_to_use,when_not_to_use,instructions,success_criteria,scope,status,priority,current_version,created_at,updated_at FROM skills WHERE id=?`), id)
	sk, err := scanSkill(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ports.ErrNotFound
		}
		return nil, err
	}
	if err = s.loadRelations(ctx, sk); err != nil {
		return nil, err
	}
	return sk, nil
}

type rowScanner interface{ Scan(...any) error }

func scanSkill(row rowScanner) (*domain.Skill, error) {
	var sk domain.Skill
	var ws, pr sql.NullString
	var criteria, created, updated string
	err := row.Scan(&sk.ID, &ws, &pr, &sk.Slug, &sk.Name, &sk.Description, &sk.Purpose, &sk.WhenToUse, &sk.WhenNotToUse, &sk.Instructions, &criteria, &sk.Scope, &sk.Status, &sk.Priority, &sk.CurrentVersion, &created, &updated)
	if err != nil {
		return nil, err
	}
	if ws.Valid {
		sk.WorkspaceID = &ws.String
	}
	if pr.Valid {
		sk.ProjectID = &pr.String
	}
	_ = json.Unmarshal([]byte(criteria), &sk.SuccessCriteria)
	sk.CreatedAt, sk.UpdatedAt = parseTime(created), parseTime(updated)
	return &sk, nil
}

func (s *Store) ListSkills(ctx context.Context) ([]domain.Skill, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,workspace_id,project_id,slug,name,description,purpose,when_to_use,when_not_to_use,instructions,success_criteria,scope,status,priority,current_version,created_at,updated_at FROM skills`)
	if err != nil {
		return nil, err
	}
	var out []domain.Skill
	for rows.Next() {
		sk, err := scanSkill(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *sk)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	for i := range out {
		if err = s.loadRelations(ctx, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) loadRelations(ctx context.Context, sk *domain.Skill) error {
	sets := []struct {
		table  string
		target *[]string
	}{{"skill_domains", &sk.Domains}, {"skill_intents", &sk.Intents}, {"skill_object_types", &sk.ObjectTypes}, {"skill_tags", &sk.Tags}, {"skill_keywords", &sk.Keywords}, {"skill_capabilities", &sk.Capabilities}, {"skill_compatibility", &sk.Compatibility}}
	for _, set := range sets {
		rows, err := s.db.QueryContext(ctx, s.q(`SELECT value FROM `+set.table+` WHERE skill_id=? ORDER BY value`), sk.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				return err
			}
			*set.target = append(*set.target, v)
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id,position,title,instruction,condition_text,is_required,expected_result,created_at,updated_at FROM skill_steps WHERE skill_id=? ORDER BY position`), sk.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v domain.Step
		var cond, exp sql.NullString
		var c, u string
		if err := rows.Scan(&v.ID, &v.Position, &v.Title, &v.Instruction, &cond, &v.Required, &exp, &c, &u); err != nil {
			rows.Close()
			return err
		}
		v.SkillID = sk.ID
		v.Condition = cond.String
		v.ExpectedResult = exp.String
		v.CreatedAt, v.UpdatedAt = parseTime(c), parseTime(u)
		sk.Steps = append(sk.Steps, v)
	}
	rows.Close()
	rows, err = s.db.QueryContext(ctx, s.q(`SELECT id,tool_name,tool_namespace,requirement,purpose,usage_hint FROM skill_tools WHERE skill_id=? ORDER BY requirement,tool_name`), sk.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v domain.ToolRequirement
		var ns, h sql.NullString
		if err := rows.Scan(&v.ID, &v.Name, &ns, &v.Requirement, &v.Purpose, &h); err != nil {
			rows.Close()
			return err
		}
		v.SkillID = sk.ID
		v.Namespace = ns.String
		v.UsageHint = h.String
		sk.Tools = append(sk.Tools, v)
	}
	rows.Close()
	rows, err = s.db.QueryContext(ctx, s.q(`SELECT id,type,query_text,description,required,priority,max_tokens FROM skill_context_requirements WHERE skill_id=? ORDER BY priority DESC,id`), sk.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v domain.ContextRequirement
		var max sql.NullInt64
		if err := rows.Scan(&v.ID, &v.Type, &v.Query, &v.Description, &v.Required, &v.Priority, &max); err != nil {
			rows.Close()
			return err
		}
		v.SkillID = sk.ID
		if max.Valid {
			x := int(max.Int64)
			v.MaxTokens = &x
		}
		sk.Contexts = append(sk.Contexts, v)
	}
	rows.Close()
	rows, err = s.db.QueryContext(ctx, s.q(`SELECT id,depends_on_skill_id,type,position FROM skill_dependencies WHERE skill_id=? ORDER BY position,id`), sk.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v domain.Dependency
		if err := rows.Scan(&v.ID, &v.DependsOnSkillID, &v.Type, &v.Position); err != nil {
			rows.Close()
			return err
		}
		v.SkillID = sk.ID
		sk.Dependencies = append(sk.Dependencies, v)
	}
	rows.Close()
	rows, err = s.db.QueryContext(ctx, s.q(`SELECT id,title,input_example,expected_behavior,bad_behavior,priority FROM skill_examples WHERE skill_id=? ORDER BY priority DESC,id`), sk.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v domain.Example
		var bad sql.NullString
		if err := rows.Scan(&v.ID, &v.Title, &v.InputExample, &v.ExpectedBehavior, &bad, &v.Priority); err != nil {
			rows.Close()
			return err
		}
		v.SkillID = sk.ID
		v.BadBehavior = bad.String
		sk.Examples = append(sk.Examples, v)
	}
	return rows.Close()
}

func (s *Store) ListVersions(ctx context.Context, id string) ([]domain.SkillVersion, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id,skill_id,version,change_summary,created_by,created_at FROM skill_versions WHERE skill_id=? ORDER BY version DESC`), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SkillVersion
	for rows.Next() {
		var v domain.SkillVersion
		var actor sql.NullString
		var c string
		if err := rows.Scan(&v.ID, &v.SkillID, &v.Version, &v.ChangeSummary, &actor, &c); err != nil {
			return nil, err
		}
		if actor.Valid {
			v.CreatedBy = &actor.String
		}
		v.CreatedAt = parseTime(c)
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) GetVersion(ctx context.Context, id string, version int) (*domain.SkillVersion, error) {
	var v domain.SkillVersion
	var actor sql.NullString
	var c string
	err := s.db.QueryRowContext(ctx, s.q(`SELECT id,skill_id,version,snapshot,change_summary,created_by,created_at FROM skill_versions WHERE skill_id=? AND version=?`), id, version).Scan(&v.ID, &v.SkillID, &v.Version, &v.Snapshot, &v.ChangeSummary, &actor, &c)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if actor.Valid {
		v.CreatedBy = &actor.String
	}
	v.CreatedAt = parseTime(c)
	return &v, nil
}
func (s *Store) RollbackSkill(ctx context.Context, id string, version int, actor *string) (*domain.Skill, error) {
	v, err := s.GetVersion(ctx, id, version)
	if err != nil {
		return nil, err
	}
	var sk domain.Skill
	if err = json.Unmarshal([]byte(v.Snapshot), &sk); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	sk.ID = id
	if err = s.UpdateSkill(ctx, &sk, fmt.Sprintf("rollback to version %d", version), actor); err != nil {
		return nil, err
	}
	return &sk, nil
}

func (s *Store) CreateExecution(ctx context.Context, e *domain.Execution) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.StartedAt.IsZero() {
		e.StartedAt = now()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now()
	}
	var finished any
	if e.FinishedAt != nil {
		finished = ts(*e.FinishedAt)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO skill_executions(id,skill_id,skill_version,workspace_id,project_id,agent_id,model_provider,model_name,task_summary,task_hash,started_at,finished_at,duration_ms,status,success,tool_calls_count,input_tokens,output_tokens,error_type,error_message,feedback,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), e.ID, e.SkillID, e.SkillVersion, e.WorkspaceID, e.ProjectID, e.AgentID, e.ModelProvider, e.ModelName, e.TaskSummary, e.TaskHash, ts(e.StartedAt), finished, e.DurationMS, e.Status, e.Success, e.ToolCallsCount, e.InputTokens, e.OutputTokens, e.ErrorType, e.ErrorMessage, e.Feedback, ts(e.CreatedAt)); err != nil {
		return err
	}
	for i := range e.Trajectory {
		event := &e.Trajectory[i]
		if event.ID == "" {
			event.ID = uuid.NewString()
		}
		event.ExecutionID = e.ID
		if event.CreatedAt.IsZero() {
			event.CreatedAt = e.CreatedAt
		}
		if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO execution_events(id,execution_id,position,event_type,event_data,created_at) VALUES(?,?,?,?,?,?)`), event.ID, e.ID, event.Position, event.Type, event.Data, ts(event.CreatedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) ListExecutions(ctx context.Context, skillID *string) ([]domain.Execution, error) {
	q := `SELECT id,skill_id,skill_version,workspace_id,project_id,agent_id,model_provider,model_name,task_summary,task_hash,started_at,finished_at,duration_ms,status,success,tool_calls_count,input_tokens,output_tokens,error_type,error_message,feedback,created_at FROM skill_executions`
	var args []any
	if skillID != nil {
		q += ` WHERE skill_id=?`
		args = append(args, *skillID)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Execution
	for rows.Next() {
		var e domain.Execution
		var ws, pr, agent, prov, model, hash, finished, errType, errMsg, feedback sql.NullString
		var duration, tools, inTok, outTok sql.NullInt64
		var started, created string
		if err := rows.Scan(&e.ID, &e.SkillID, &e.SkillVersion, &ws, &pr, &agent, &prov, &model, &e.TaskSummary, &hash, &started, &finished, &duration, &e.Status, &e.Success, &tools, &inTok, &outTok, &errType, &errMsg, &feedback, &created); err != nil {
			return nil, err
		}
		e.WorkspaceID = strPtr(ws)
		e.ProjectID = strPtr(pr)
		e.AgentID = strPtr(agent)
		e.ModelProvider = strPtr(prov)
		e.ModelName = strPtr(model)
		e.TaskHash = strPtr(hash)
		e.ErrorType = strPtr(errType)
		e.ErrorMessage = strPtr(errMsg)
		e.Feedback = strPtr(feedback)
		e.StartedAt = parseTime(started)
		e.CreatedAt = parseTime(created)
		if finished.Valid {
			t := parseTime(finished.String)
			e.FinishedAt = &t
		}
		e.DurationMS = int64Ptr(duration)
		e.ToolCallsCount = intPtr(tools)
		e.InputTokens = intPtr(inTok)
		e.OutputTokens = intPtr(outTok)
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *Store) Statistics(ctx context.Context, skillID *string) ([]domain.Statistics, error) {
	execs, err := s.ListExecutions(ctx, skillID)
	if err != nil {
		return nil, err
	}
	stats := map[string]*domain.Statistics{}
	models := map[string]map[string]*domain.ModelStatistic{}
	for _, e := range execs {
		st := stats[e.SkillID]
		if st == nil {
			st = &domain.Statistics{SkillID: e.SkillID}
			stats[e.SkillID] = st
			models[e.SkillID] = map[string]*domain.ModelStatistic{}
		}
		st.Runs++
		if e.Success {
			st.Successes++
		}
		key := value(e.ModelProvider) + "\x00" + value(e.ModelName)
		ms := models[e.SkillID][key]
		if ms == nil {
			ms = &domain.ModelStatistic{Provider: value(e.ModelProvider), Model: value(e.ModelName)}
			models[e.SkillID][key] = ms
		}
		ms.Runs++
		if e.Success {
			ms.Successes++
		}
	}
	out := make([]domain.Statistics, 0, len(stats))
	for id, st := range stats {
		st.SuccessRate = rate(st.Successes, st.Runs)
		for _, ms := range models[id] {
			ms.SuccessRate = rate(ms.Successes, ms.Runs)
			st.ByModel = append(st.ByModel, *ms)
		}
		sort.Slice(st.ByModel, func(i, j int) bool { return st.ByModel[i].Runs > st.ByModel[j].Runs })
		out = append(out, *st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Runs > out[j].Runs })
	return out, nil
}

func unique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		k := strings.ToLower(strings.TrimSpace(v))
		if k != "" && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}
func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func strPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
func intPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	x := int(v.Int64)
	return &x
}
func int64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}
func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func rate(s, n int) float64 {
	if n == 0 {
		return 0
	}
	return float64(s) * 100 / float64(n)
}
