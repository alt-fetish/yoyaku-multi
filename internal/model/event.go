package model

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Event は開催日
type Event struct {
	ID          uuid.UUID
	SessionType string
	EventDate   time.Time
	EventTime   time.Time
	EndTime     *time.Time
	Status      string
	Notes       string
	CreatedAt   time.Time
}

// EventWithCount は参加表明数付き開催日
type EventWithCount struct {
	Event
	EntryCount int
}

// EventEntry は参加表明
type EventEntry struct {
	ID          uuid.UUID
	EventID     uuid.UUID
	ApplicantID uuid.UUID
	Status      string
	CreatedAt   time.Time
}

// EntryWithApplicant は参加者情報付き表明
type EntryWithApplicant struct {
	EventEntry
	Handle     string
	NGSettings []*NGSetting
}

// EntryWithEvent は開催日情報付き表明
type EntryWithEvent struct {
	EventEntry
	Event Event
}

// EventRow はCSVの1行
type EventRow struct {
	Date      string
	StartTime string
	EndTime   string
	Type      string
}

// BulkResult はCSV一括登録の結果
type BulkResult struct {
	Inserted int
	Skipped  []BulkRowResult
	Errors   []BulkRowResult
}

// BulkRowResult は行ごとの結果
type BulkRowResult struct {
	RowNum int
	Row    EventRow
	Reason string
}

// PostgresEventRepo はPostgreSQLによる実装
type PostgresEventRepo struct {
	DB *pgxpool.Pool
}

func (r *PostgresEventRepo) GetEvent(id uuid.UUID) (*Event, error) {
	ctx := context.Background()
	e := &Event{}
	err := r.DB.QueryRow(ctx,
		`SELECT id, session_type, event_date, event_time, end_time, status, COALESCE(notes,''), created_at
		 FROM events WHERE id = $1`,
		id,
	).Scan(&e.ID, &e.SessionType, &e.EventDate, &e.EventTime, &e.EndTime, &e.Status, &e.Notes, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *PostgresEventRepo) ListEvents(status string) ([]*EventWithCount, error) {
	ctx := context.Background()

	query := `SELECT e.id, e.session_type, e.event_date, e.event_time, e.end_time, e.status,
	                 COALESCE(e.notes,''), e.created_at,
	                 COUNT(ee.id) AS entry_count
	          FROM events e
	          LEFT JOIN event_entries ee ON ee.event_id = e.id AND ee.status != 'cancelled'
	          %s
	          GROUP BY e.id
	          ORDER BY e.event_date DESC, e.event_time DESC`

	var rows interface{ Rows() interface{} }
	_ = rows

	var (
		pgRows interface{ Next() bool }
	)
	_ = pgRows

	if status != "" {
		r2, err := r.DB.Query(ctx, fmt.Sprintf(query, "WHERE e.status = $1"), status)
		if err != nil {
			return nil, err
		}
		defer r2.Close()
		return scanEventRows(r2)
	}

	r2, err := r.DB.Query(ctx, fmt.Sprintf(query, ""))
	if err != nil {
		return nil, err
	}
	defer r2.Close()
	return scanEventRows(r2)
}

func (r *PostgresEventRepo) ListPublicEvents() ([]*EventWithCount, error) {
	ctx := context.Background()
	rows, err := r.DB.Query(ctx,
		`SELECT e.id, e.session_type, e.event_date, e.event_time, e.end_time, e.status,
		        COALESCE(e.notes,''), e.created_at,
		        COUNT(ee.id) AS entry_count
		 FROM events e
		 LEFT JOIN event_entries ee ON ee.event_id = e.id AND ee.status != 'cancelled'
		 WHERE e.status = 'open' AND e.event_date >= CURRENT_DATE
		 GROUP BY e.id
		 ORDER BY e.event_date ASC, e.event_time ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEventRows(rows)
}

type pgRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

func scanEventRows(rows pgRows) ([]*EventWithCount, error) {
	var result []*EventWithCount
	for rows.Next() {
		ew := &EventWithCount{}
		err := rows.Scan(
			&ew.ID, &ew.SessionType, &ew.EventDate, &ew.EventTime,
			&ew.EndTime, &ew.Status, &ew.Notes, &ew.CreatedAt,
			&ew.EntryCount,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, ew)
	}
	return result, rows.Err()
}

func (r *PostgresEventRepo) CreateEvent(e *Event) error {
	ctx := context.Background()
	return r.DB.QueryRow(ctx,
		`INSERT INTO events (session_type, event_date, event_time, end_time, status, notes)
		 VALUES ($1, $2, $3, $4, 'open', $5)
		 RETURNING id, created_at`,
		e.SessionType, e.EventDate.Format("2006-01-02"),
		e.EventTime.Format("15:04"), e.EndTime, e.Notes,
	).Scan(&e.ID, &e.CreatedAt)
}

func (r *PostgresEventRepo) UpdateEvent(e *Event) error {
	ctx := context.Background()
	_, err := r.DB.Exec(ctx,
		`UPDATE events SET session_type=$1, event_date=$2, event_time=$3, end_time=$4, notes=$5, status=$6
		 WHERE id=$7`,
		e.SessionType, e.EventDate.Format("2006-01-02"),
		e.EventTime.Format("15:04"), e.EndTime, e.Notes, e.Status, e.ID,
	)
	return err
}

func (r *PostgresEventRepo) DeleteEvent(id uuid.UUID) error {
	ctx := context.Background()
	_, err := r.DB.Exec(ctx, `DELETE FROM events WHERE id=$1 AND status='open'`, id)
	return err
}

func (r *PostgresEventRepo) GetAllApplicantEmails() ([]string, error) {
	ctx := context.Background()
	rows, err := r.DB.Query(ctx, `SELECT email FROM applicants WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}
	return emails, rows.Err()
}

// BulkCreateEvents はCSVからの一括登録
func (r *PostgresEventRepo) BulkCreateEvents(rows []EventRow) BulkResult {
	ctx := context.Background()
	result := BulkResult{}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		result.Errors = append(result.Errors, BulkRowResult{RowNum: 0, Reason: "DB接続エラー"})
		return result
	}
	defer tx.Rollback(ctx)

	for i, row := range rows {
		rowNum := i + 2 // ヘッダー行を1とした場合

		// 日付パース
		eventDate, err := time.Parse("2006-01-02", row.Date)
		if err != nil {
			result.Errors = append(result.Errors, BulkRowResult{RowNum: rowNum, Row: row, Reason: "日付形式エラー"})
			continue
		}

		// 開始時刻パース
		startTime, err := time.Parse("15:04", row.StartTime)
		if err != nil {
			result.Errors = append(result.Errors, BulkRowResult{RowNum: rowNum, Row: row, Reason: "開始時刻形式エラー"})
			continue
		}

		// 終了時刻パース（省略可）
		var endTime *time.Time
		if row.EndTime != "" {
			et, err := time.Parse("15:04", row.EndTime)
			if err != nil {
				result.Errors = append(result.Errors, BulkRowResult{RowNum: rowNum, Row: row, Reason: "終了時刻形式エラー"})
				continue
			}
			endTime = &et
		}

		// typeバリデーション
		if row.Type != "solo" && row.Type != "group" {
			result.Errors = append(result.Errors, BulkRowResult{RowNum: rowNum, Row: row, Reason: "typeはsoloかgroupのみ"})
			continue
		}

		// INSERT（重複時はスキップ）
		var inserted int
		err = tx.QueryRow(ctx,
			`INSERT INTO events (session_type, event_date, event_time, end_time, status)
			 VALUES ($1, $2, $3, $4, 'open')
			 ON CONFLICT (event_date, event_time) DO NOTHING
			 RETURNING 1`,
			row.Type,
			eventDate.Format("2006-01-02"),
			startTime.Format("15:04"),
			endTime,
		).Scan(&inserted)

		if err != nil && err.Error() == "no rows in result set" {
			result.Skipped = append(result.Skipped, BulkRowResult{RowNum: rowNum, Row: row, Reason: "同日時の開催日が既に存在"})
			continue
		}
		if err != nil {
			result.Errors = append(result.Errors, BulkRowResult{RowNum: rowNum, Row: row, Reason: err.Error()})
			continue
		}
		if inserted == 0 {
			result.Skipped = append(result.Skipped, BulkRowResult{RowNum: rowNum, Row: row, Reason: "同日時の開催日が既に存在"})
			continue
		}
		result.Inserted++
	}

	if err := tx.Commit(ctx); err != nil {
		return BulkResult{Errors: []BulkRowResult{{Reason: "コミット失敗: " + err.Error()}}}
	}
	return result
}

// ParseCSV はCSVをパースしてEventRowのスライスを返す
func ParseCSV(r io.Reader) ([]EventRow, []BulkRowResult, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	// ヘッダー確認
	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("CSVの読み込みに失敗しました")
	}
	expectedHeader := []string{"date", "starttime", "endtime", "type"}
	for i, h := range expectedHeader {
		if i >= len(header) || strings.TrimSpace(strings.ToLower(header[i])) != h {
			return nil, nil, fmt.Errorf("ヘッダー形式エラー。期待: date,starttime,endtime,type")
		}
	}

	var rows []EventRow
	var parseErrors []BulkRowResult
	lineNum := 1

	for {
		lineNum++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			parseErrors = append(parseErrors, BulkRowResult{RowNum: lineNum, Reason: "パースエラー: " + err.Error()})
			continue
		}

		// 空行・コメント行スキップ
		if len(record) == 0 || strings.HasPrefix(strings.TrimSpace(record[0]), "#") {
			continue
		}

		if len(record) < 4 {
			parseErrors = append(parseErrors, BulkRowResult{RowNum: lineNum, Reason: "列数不足"})
			continue
		}

		rows = append(rows, EventRow{
			Date:      strings.TrimSpace(record[0]),
			StartTime: strings.TrimSpace(record[1]),
			EndTime:   strings.TrimSpace(record[2]),
			Type:      strings.TrimSpace(record[3]),
		})
	}
	return rows, parseErrors, nil
}

// --- event_entries ---

type PostgresEntryRepo struct {
	DB *pgxpool.Pool
}

func (r *PostgresEntryRepo) GetEntry(eventID, applicantID uuid.UUID) (*EventEntry, error) {
	ctx := context.Background()
	e := &EventEntry{}
	err := r.DB.QueryRow(ctx,
		`SELECT id, event_id, applicant_id, status, created_at
		 FROM event_entries WHERE event_id=$1 AND applicant_id=$2`,
		eventID, applicantID,
	).Scan(&e.ID, &e.EventID, &e.ApplicantID, &e.Status, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *PostgresEntryRepo) CreateEntry(entry *EventEntry) error {
	ctx := context.Background()
	return r.DB.QueryRow(ctx,
		`INSERT INTO event_entries (event_id, applicant_id, status)
		 VALUES ($1, $2, 'pending')
		 RETURNING id, created_at`,
		entry.EventID, entry.ApplicantID,
	).Scan(&entry.ID, &entry.CreatedAt)
}

func (r *PostgresEntryRepo) UpdateEntryStatus(id uuid.UUID, status string) error {
	ctx := context.Background()
	_, err := r.DB.Exec(ctx,
		`UPDATE event_entries SET status=$1 WHERE id=$2`,
		status, id,
	)
	return err
}

func (r *PostgresEntryRepo) DeleteEntry(id uuid.UUID) error {
	ctx := context.Background()
	_, err := r.DB.Exec(ctx, `DELETE FROM event_entries WHERE id=$1`, id)
	return err
}

func (r *PostgresEntryRepo) ListEntriesByEvent(eventID uuid.UUID) ([]*EntryWithApplicant, error) {
	ctx := context.Background()
	rows, err := r.DB.Query(ctx,
		`SELECT ee.id, ee.event_id, ee.applicant_id, ee.status, ee.created_at,
		        a.handle
		 FROM event_entries ee
		 JOIN applicants a ON a.id = ee.applicant_id
		 WHERE ee.event_id = $1
		 ORDER BY ee.created_at ASC`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*EntryWithApplicant
	for rows.Next() {
		ea := &EntryWithApplicant{}
		if err := rows.Scan(
			&ea.ID, &ea.EventID, &ea.ApplicantID, &ea.Status, &ea.CreatedAt,
			&ea.Handle,
		); err != nil {
			return nil, err
		}
		result = append(result, ea)
	}
	return result, rows.Err()
}

func (r *PostgresEntryRepo) ListEntriesByApplicant(applicantID uuid.UUID) ([]*EntryWithEvent, error) {
	ctx := context.Background()
	rows, err := r.DB.Query(ctx,
		`SELECT ee.id, ee.event_id, ee.applicant_id, ee.status, ee.created_at,
		        e.id, e.session_type, e.event_date, e.event_time, e.end_time, e.status, COALESCE(e.notes,''), e.created_at
		 FROM event_entries ee
		 JOIN events e ON e.id = ee.event_id
		 WHERE ee.applicant_id = $1
		 ORDER BY e.event_date DESC`,
		applicantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*EntryWithEvent
	for rows.Next() {
		ew := &EntryWithEvent{}
		if err := rows.Scan(
			&ew.ID, &ew.EventID, &ew.ApplicantID, &ew.Status, &ew.CreatedAt,
			&ew.Event.ID, &ew.Event.SessionType, &ew.Event.EventDate, &ew.Event.EventTime,
			&ew.Event.EndTime, &ew.Event.Status, &ew.Event.Notes, &ew.Event.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, ew)
	}
	return result, rows.Err()
}

// ConfirmEntries は参加者確定処理（トランザクション）
func ConfirmEntries(db *pgxpool.Pool, eventID uuid.UUID, confirmedApplicantIDs []uuid.UUID) (*Session, error) {
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// イベント取得
	e := &Event{}
	err = tx.QueryRow(ctx,
		`SELECT id, session_type, event_date, event_time FROM events WHERE id=$1`,
		eventID,
	).Scan(&e.ID, &e.SessionType, &e.EventDate, &e.EventTime)
	if err != nil {
		return nil, err
	}

	// 確定参加者のステータスを confirmed に
	for _, aID := range confirmedApplicantIDs {
		_, err = tx.Exec(ctx,
			`UPDATE event_entries SET status='confirmed' WHERE event_id=$1 AND applicant_id=$2`,
			eventID, aID,
		)
		if err != nil {
			return nil, err
		}
	}

	// 未選定者を declined に
	_, err = tx.Exec(ctx,
		`UPDATE event_entries SET status='declined'
		 WHERE event_id=$1 AND status='pending'`,
		eventID,
	)
	if err != nil {
		return nil, err
	}

	// イベントステータスを confirmed に
	_, err = tx.Exec(ctx, `UPDATE events SET status='confirmed' WHERE id=$1`, eventID)
	if err != nil {
		return nil, err
	}

	// セッション自動生成
	s := &Session{}
	err = tx.QueryRow(ctx,
		`INSERT INTO sessions (session_type, session_date, session_time, status, event_id)
		 VALUES ($1, $2, $3, 'confirmed', $4)
		 RETURNING id, created_at`,
		e.SessionType, e.EventDate.Format("2006-01-02"), e.EventTime.Format("15:04"), eventID,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return nil, err
	}

	for _, aID := range confirmedApplicantIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO session_participants (session_id, applicant_id) VALUES ($1, $2)`,
			s.ID, aID,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// GetConfirmedApplicants は確定した参加者を返す
func GetConfirmedApplicants(db *pgxpool.Pool, eventID uuid.UUID) ([]*Applicant, error) {
	ctx := context.Background()
	rows, err := db.Query(ctx,
		`SELECT a.id, a.handle, a.email, a.token, a.status, a.created_at
		 FROM applicants a
		 JOIN event_entries ee ON ee.applicant_id = a.id
		 WHERE ee.event_id=$1 AND ee.status='confirmed'`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Applicant
	for rows.Next() {
		a := &Applicant{}
		if err := rows.Scan(&a.ID, &a.Handle, &a.Email, &a.Token, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// GetDeclinedApplicants は見送りになった参加者を返す
func GetDeclinedApplicants(db *pgxpool.Pool, eventID uuid.UUID) ([]*Applicant, error) {
	ctx := context.Background()
	rows, err := db.Query(ctx,
		`SELECT a.id, a.handle, a.email, a.token, a.status, a.created_at
		 FROM applicants a
		 JOIN event_entries ee ON ee.applicant_id = a.id
		 WHERE ee.event_id=$1 AND ee.status='declined'`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Applicant
	for rows.Next() {
		a := &Applicant{}
		if err := rows.Scan(&a.ID, &a.Handle, &a.Email, &a.Token, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
