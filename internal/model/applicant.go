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

// NGSetting はNG行為設定
type NGSetting struct {
	ID          uuid.UUID
	ApplicantID uuid.UUID
	ActionKey   string
	IsOK        bool
}

// ApplicantWithCount は参加回数付き参加者
type ApplicantWithCount struct {
	Applicant
	SessionCount int
	UnreadCount  int
	NGSettings   []*NGSetting
}

// NGActionKeys はNG行為のキー一覧（表示順）
var NGActionKeys = []string{
	"finger_mouth",
	"anal",
	"sheath",
	"kiss",
	"fellatio",
}

// NGActionLabels は日本語ラベル
var NGActionLabels = map[string]string{
	"finger_mouth": "指を口に入れる",
	"anal":         "アナルに触れる",
	"sheath":       "シース（陰茎部ラバー）を外す",
	"kiss":         "キス",
	"fellatio":     "フェラチオ",
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

func (r *PostgresApplicantRepo) CreateApplicant(a *Applicant, ngSettings []*NGSetting) error {
	ctx := context.Background()
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO applicants (handle, email, token, status)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		a.Handle, a.Email, a.Token, "active",
	).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return err
	}

	for _, ng := range ngSettings {
		ng.ApplicantID = a.ID
		_, err = tx.Exec(ctx,
			`INSERT INTO ng_settings (applicant_id, action_key, is_ok) VALUES ($1, $2, $3)`,
			ng.ApplicantID, ng.ActionKey, ng.IsOK,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
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

func (r *PostgresApplicantRepo) GetNGSettings(applicantID uuid.UUID) ([]*NGSetting, error) {
	ctx := context.Background()
	rows, err := r.DB.Query(ctx,
		`SELECT id, applicant_id, action_key, is_ok FROM ng_settings WHERE applicant_id = $1`,
		applicantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*NGSetting
	for rows.Next() {
		ng := &NGSetting{}
		if err := rows.Scan(&ng.ID, &ng.ApplicantID, &ng.ActionKey, &ng.IsOK); err != nil {
			return nil, err
		}
		result = append(result, ng)
	}
	return result, rows.Err()
}

// UpdateNGSettings は参加者のNG設定を一括更新する（UPSERT）
func (r *PostgresApplicantRepo) UpdateNGSettings(applicantID uuid.UUID, settings map[string]bool) error {
	ctx := context.Background()
	for _, key := range NGActionKeys {
		isOK := settings[key]
		_, err := r.DB.Exec(ctx,
			`INSERT INTO ng_settings (applicant_id, action_key, is_ok)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (applicant_id, action_key) DO UPDATE SET is_ok = $3`,
			applicantID, key, isOK,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// NGSettingsMap はapplicant_idをキーにしたNGマップを返す（管理者用）
func (r *PostgresApplicantRepo) GetNGSettingsForApplicants(applicantIDs []uuid.UUID) (map[uuid.UUID][]*NGSetting, error) {
	if len(applicantIDs) == 0 {
		return map[uuid.UUID][]*NGSetting{}, nil
	}
	ctx := context.Background()

	// pgx/v5 でスライスをIN句に渡す
	rows, err := r.DB.Query(ctx,
		`SELECT id, applicant_id, action_key, is_ok FROM ng_settings WHERE applicant_id = ANY($1)`,
		applicantIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]*NGSetting)
	for rows.Next() {
		ng := &NGSetting{}
		if err := rows.Scan(&ng.ID, &ng.ApplicantID, &ng.ActionKey, &ng.IsOK); err != nil {
			return nil, err
		}
		result[ng.ApplicantID] = append(result[ng.ApplicantID], ng)
	}
	return result, rows.Err()
}
