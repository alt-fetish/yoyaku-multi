package model

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChatMessage はチャットメッセージ
type ChatMessage struct {
	ID          uuid.UUID
	ApplicantID uuid.UUID
	Sender      string // admin / applicant
	Body        string
	IsRead      bool
	CreatedAt   time.Time
}

// PostgresChatRepo はPostgreSQLによる実装
type PostgresChatRepo struct {
	DB *pgxpool.Pool
}

func (r *PostgresChatRepo) GetMessages(applicantID uuid.UUID) ([]*ChatMessage, error) {
	ctx := context.Background()
	rows, err := r.DB.Query(ctx,
		`SELECT id, applicant_id, sender, body, is_read, created_at
		 FROM chat_messages
		 WHERE applicant_id=$1
		 ORDER BY created_at ASC`,
		applicantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*ChatMessage
	for rows.Next() {
		msg := &ChatMessage{}
		if err := rows.Scan(&msg.ID, &msg.ApplicantID, &msg.Sender, &msg.Body, &msg.IsRead, &msg.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, msg)
	}
	return result, rows.Err()
}

func (r *PostgresChatRepo) PostMessage(msg *ChatMessage) error {
	ctx := context.Background()
	return r.DB.QueryRow(ctx,
		`INSERT INTO chat_messages (applicant_id, sender, body)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		msg.ApplicantID, msg.Sender, msg.Body,
	).Scan(&msg.ID, &msg.CreatedAt)
}

func (r *PostgresChatRepo) MarkAsRead(applicantID uuid.UUID, sender string) error {
	ctx := context.Background()
	_, err := r.DB.Exec(ctx,
		`UPDATE chat_messages SET is_read=TRUE WHERE applicant_id=$1 AND sender=$2 AND is_read=FALSE`,
		applicantID, sender,
	)
	return err
}

func (r *PostgresChatRepo) UnreadCount(applicantID uuid.UUID) (int, error) {
	ctx := context.Background()
	var count int
	err := r.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM chat_messages WHERE applicant_id=$1 AND sender='applicant' AND is_read=FALSE`,
		applicantID,
	).Scan(&count)
	return count, err
}
