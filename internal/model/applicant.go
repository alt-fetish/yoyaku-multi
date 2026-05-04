package model

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Applicant は参加希望者
type Applicant struct {
	ID        uuid.UUID
	Handle    string
	Email     string
	Token     string
	Status    string
	CreatedAt time.Time
}

// ApplicantWithCount は参加回数付き参加者
type ApplicantWithCount struct {
	Applicant
	SessionCount int
	UnreadCount  int
}

// PostgresApplicantRepo はPostgreSQLによる実装
type PostgresApplicantRepo struct {
	DB *pgxpool.Pool
}

func (r *PostgresApplicantRepo) GetApplicant(id uuid.UUID) (*Applicant, error) {
	ctx := context.Background()
	a := &Applicant{}
	err := r.DB.QueryRow(ctx,
		`SELECT id, handle, email, token, status, created_at FROM applicants WHERE id = $1`,
		id,
	).Scan(&a.ID, &a.Handle, &a.Email, &a.Token, &a.Status, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *PostgresApplicantRepo) GetApplicantByToken(token string) (*Applicant, error) {
	ctx := context.Background()
	a := &Applicant{}
	err := r.DB.QueryRow(ctx,
		`SELECT id, handle, email, token, status, created_at FROM applicants WHERE token = $1`,
		token,
	).Scan(&a.ID, &a.Handle, &a.Email, &a.Token, &a.Status, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *PostgresApplicantRepo) GetApplicantByEmail(email string) (*Applicant, error) {
	ctx := context.Background()
	a := &Applicant{}
	err := r.DB.QueryRow(ctx,
		`SELECT id, handle, email, token, status, created_at FROM applicants WHERE email = $1`,
		email,
	).Scan(&a.ID, &a.Handle, &a.Email, &a.Token, &a.Status, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *PostgresApplicantRepo) CreateApplicant(a *Applicant) error {
	ctx := context.Background()
	return r.DB.QueryRow(ctx,
		`INSERT INTO applicants (handle, email, token, status)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		a.Handle, a.Email, a.Token, "active",
	).Scan(&a.ID, &a.CreatedAt)
}

func (r *PostgresApplicantRepo) ListApplicants() ([]*ApplicantWithCount, error) {
	ctx := context.Background()
	rows, err := r.DB.Query(ctx,
		`SELECT a.id, a.handle, a.email, a.token, a.status, a.created_at,
		        COUNT(DISTINCT sp.session_id) AS session_count,
		        COALESCE((
		            SELECT COUNT(*) FROM chat_messages cm
		            WHERE cm.applicant_id = a.id AND cm.sender = 'applicant' AND cm.is_read = FALSE
		        ), 0) AS unread_count
		 FROM applicants a
		 LEFT JOIN session_participants sp ON sp.applicant_id = a.id
		 WHERE a.status = 'active'
		 GROUP BY a.id
		 ORDER BY a.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*ApplicantWithCount
	for rows.Next() {
		aw := &ApplicantWithCount{}
		err := rows.Scan(
			&aw.ID, &aw.Handle, &aw.Email, &aw.Token, &aw.Status, &aw.CreatedAt,
			&aw.SessionCount, &aw.UnreadCount,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, aw)
	}
	return result, rows.Err()
}
