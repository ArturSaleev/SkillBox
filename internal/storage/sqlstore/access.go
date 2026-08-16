package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
	"github.com/google/uuid"
)

func (s *Store) MarkSkillProposalPublished(ctx context.Context, id string) error {
	stamp := now()
	res, err := s.db.ExecContext(ctx, s.q(`UPDATE skill_proposals SET status='published',updated_at=? WHERE id=? AND status='approved'`), ts(stamp), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ports.ErrNotFound
	}
	return nil
}
func (s *Store) CreateSkillProposal(ctx context.Context, p *domain.SkillProposal) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.Status == "" {
		p.Status = "pending"
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now()
	}
	p.UpdatedAt = p.CreatedAt
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO skill_proposals(id,skill_id,base_version,proposed_snapshot,summary,status,created_by,reviewed_by,review_note,created_at,updated_at,reviewed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`), p.ID, p.SkillID, p.BaseVersion, p.ProposedSnapshot, p.Summary, p.Status, p.CreatedBy, nil, nil, ts(p.CreatedAt), ts(p.UpdatedAt), nil)
	return err
}
func scanProposal(row rowScanner) (*domain.SkillProposal, error) {
	var p domain.SkillProposal
	var createdBy, reviewedBy, note, reviewed sql.NullString
	var created, updated string
	if err := row.Scan(&p.ID, &p.SkillID, &p.BaseVersion, &p.ProposedSnapshot, &p.Summary, &p.Status, &createdBy, &reviewedBy, &note, &created, &updated, &reviewed); err != nil {
		return nil, err
	}
	p.CreatedBy = strPtr(createdBy)
	p.ReviewedBy = strPtr(reviewedBy)
	p.ReviewNote = strPtr(note)
	p.CreatedAt, p.UpdatedAt = parseTime(created), parseTime(updated)
	if reviewed.Valid {
		v := parseTime(reviewed.String)
		p.ReviewedAt = &v
	}
	return &p, nil
}
func proposalSelect() string {
	return `SELECT id,skill_id,base_version,proposed_snapshot,summary,status,created_by,reviewed_by,review_note,created_at,updated_at,reviewed_at FROM skill_proposals`
}
func (s *Store) GetSkillProposal(ctx context.Context, id string) (*domain.SkillProposal, error) {
	p, err := scanProposal(s.db.QueryRowContext(ctx, s.q(proposalSelect()+` WHERE id=?`), id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ports.ErrNotFound
	}
	return p, err
}
func (s *Store) ListSkillProposals(ctx context.Context, skillID *string, status string) ([]domain.SkillProposal, error) {
	q := proposalSelect() + ` WHERE 1=1`
	var args []any
	if skillID != nil {
		q += ` AND skill_id=?`
		args = append(args, *skillID)
	}
	if strings.TrimSpace(status) != "" {
		q += ` AND status=?`
		args = append(args, status)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SkillProposal
	for rows.Next() {
		p, err := scanProposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}
func (s *Store) ReviewSkillProposal(ctx context.Context, id, status string, reviewer, note *string) (*domain.SkillProposal, error) {
	if status != "approved" && status != "rejected" {
		return nil, errors.New("review status must be approved or rejected")
	}
	stamp := now()
	res, err := s.db.ExecContext(ctx, s.q(`UPDATE skill_proposals SET status=?,reviewed_by=?,review_note=?,reviewed_at=?,updated_at=? WHERE id=? AND status='pending'`), status, reviewer, note, ts(stamp), ts(stamp), id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ports.ErrNotFound
	}
	return s.GetSkillProposal(ctx, id)
}

func (s *Store) GetExecutionTrajectory(ctx context.Context, id string) ([]domain.ExecutionEvent, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id,execution_id,position,event_type,event_data,created_at FROM execution_events WHERE execution_id=? ORDER BY position`), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ExecutionEvent
	for rows.Next() {
		var v domain.ExecutionEvent
		var created string
		if err := rows.Scan(&v.ID, &v.ExecutionID, &v.Position, &v.Type, &v.Data, &created); err != nil {
			return nil, err
		}
		v.CreatedAt = parseTime(created)
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) GetExecution(ctx context.Context, id string) (*domain.Execution, error) {
	items, err := s.ListExecutions(ctx, nil)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == id {
			items[i].Trajectory, err = s.GetExecutionTrajectory(ctx, id)
			return &items[i], err
		}
	}
	return nil, ports.ErrNotFound
}
