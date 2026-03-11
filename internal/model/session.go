package model

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Session はセッション履歴
type Session struct {
	ID          uuid.UUID
	SessionType string
	SessionDate time.Time
	SessionTime time.Time
	CompletedAt *time.Time
	Status      string
	Notes       string
	EventID     *uuid.UUID
	CreatedAt   time.Time
}

// SessionWithParticipants は参加者情報付きセッション
type SessionWithParticipants struct {
	Session
	Participants []*Applicant
}

// PostgresSessionRepo はPostgreSQLによる実装
type PostgresSessionRepo struct {
	DB *pgxpool.Pool
}

func (r *PostgresSessionRepo) ListSessions(statusFilter string) ([]*SessionWithParticipants, error) {
	ctx := context.Background()

	query := `SELECT s.id, s.session_type, s.session_date, s.session_time,
	                 s.completed_at, s.status, COALESCE(s.notes,''), s.event_id, s.created_at
	          FROM sessions s
	          %s
	          ORDER BY s.session_date DESC, s.session_time DESC`

	var rows pgRows
	var err error
	if statusFilter != "" {
		rows, err = r.DB.Query(ctx, fmt.Sprintf(query, "WHERE s.status=$1"), statusFilter)
	} else {
		rows, err = r.DB.Query(ctx, fmt.Sprintf(query, ""))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*SessionWithParticipants
	var sessionIDs []uuid.UUID
	sessionMap := make(map[uuid.UUID]*SessionWithParticipants)

	for rows.Next() {
		sw := &SessionWithParticipants{}
		if err := rows.Scan(
			&sw.ID, &sw.SessionType, &sw.SessionDate, &sw.SessionTime,
			&sw.CompletedAt, &sw.Status, &sw.Notes, &sw.EventID, &sw.CreatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, sw)
		sessionIDs = append(sessionIDs, sw.ID)
		sessionMap[sw.ID] = sw
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if len(sessionIDs) == 0 {
		return sessions, nil
	}

	// 参加者を一括取得
	pRows, err := r.DB.Query(ctx,
		`SELECT sp.session_id, a.id, a.handle, a.email, a.token, a.status, a.created_at
		 FROM session_participants sp
		 JOIN applicants a ON a.id = sp.applicant_id
		 WHERE sp.session_id = ANY($1)`,
		sessionIDs,
	)
	if err != nil {
		return nil, err
	}
	defer pRows.Close()

	for pRows.Next() {
		var sessionID uuid.UUID
		a := &Applicant{}
		if err := pRows.Scan(&sessionID, &a.ID, &a.Handle, &a.Email, &a.Token, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		if sw, ok := sessionMap[sessionID]; ok {
			sw.Participants = append(sw.Participants, a)
		}
	}
	return sessions, pRows.Err()
}

func (r *PostgresSessionRepo) GetSession(id uuid.UUID) (*SessionWithParticipants, error) {
	ctx := context.Background()
	sw := &SessionWithParticipants{}
	err := r.DB.QueryRow(ctx,
		`SELECT id, session_type, session_date, session_time, completed_at, status, COALESCE(notes,''), event_id, created_at
		 FROM sessions WHERE id=$1`,
		id,
	).Scan(&sw.ID, &sw.SessionType, &sw.SessionDate, &sw.SessionTime,
		&sw.CompletedAt, &sw.Status, &sw.Notes, &sw.EventID, &sw.CreatedAt)
	if err != nil {
		return nil, err
	}

	rows, err := r.DB.Query(ctx,
		`SELECT a.id, a.handle, a.email, a.token, a.status, a.created_at
		 FROM session_participants sp
		 JOIN applicants a ON a.id = sp.applicant_id
		 WHERE sp.session_id=$1`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		a := &Applicant{}
		if err := rows.Scan(&a.ID, &a.Handle, &a.Email, &a.Token, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		sw.Participants = append(sw.Participants, a)
	}
	return sw, rows.Err()
}

func (r *PostgresSessionRepo) CreateSession(s *Session, applicantIDs []uuid.UUID) error {
	ctx := context.Background()
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO sessions (session_type, session_date, session_time, status, notes, event_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		s.SessionType, s.SessionDate.Format("2006-01-02"), s.SessionTime.Format("15:04"),
		s.Status, s.Notes, s.EventID,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return err
	}

	for _, aID := range applicantIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO session_participants (session_id, applicant_id) VALUES ($1, $2)`,
			s.ID, aID,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresSessionRepo) UpdateSession(s *Session, applicantIDs []uuid.UUID) error {
	ctx := context.Background()
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// done に変更する場合は completed_at を自動セット
	completedAtClause := ""
	if s.Status == "done" && s.CompletedAt == nil {
		completedAtClause = ", completed_at = NOW()"
	}

	_, err = tx.Exec(ctx,
		fmt.Sprintf(`UPDATE sessions SET session_type=$1, session_date=$2, session_time=$3, status=$4, notes=$5%s WHERE id=$6`,
			completedAtClause),
		s.SessionType, s.SessionDate.Format("2006-01-02"), s.SessionTime.Format("15:04"),
		s.Status, s.Notes, s.ID,
	)
	if err != nil {
		return err
	}

	// 参加者を再設定
	_, err = tx.Exec(ctx, `DELETE FROM session_participants WHERE session_id=$1`, s.ID)
	if err != nil {
		return err
	}

	for _, aID := range applicantIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO session_participants (session_id, applicant_id) VALUES ($1, $2)`,
			s.ID, aID,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresSessionRepo) DeleteSession(id uuid.UUID) error {
	ctx := context.Background()
	_, err := r.DB.Exec(ctx, `DELETE FROM sessions WHERE id=$1`, id)
	return err
}

// MarkEventDone はイベントをdoneにしてsessionsを更新する
func MarkEventDone(db *pgxpool.Pool, eventID uuid.UUID) error {
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `UPDATE events SET status='done' WHERE id=$1`, eventID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE sessions SET status='done', completed_at=NOW() WHERE event_id=$1`,
		eventID,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
