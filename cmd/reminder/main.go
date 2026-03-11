// reminder は翌日の confirmed セッション参加者にリマインダーメールを送るバッチです。
// Fly.io の Scheduled Machines または外部 cron から毎夜実行します。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/ryotaro/yoyaku-multi/internal/mail"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	resendKey := os.Getenv("RESEND_API_KEY")
	fromEmail := os.Getenv("FROM_EMAIL")
	baseURL := os.Getenv("BASE_URL")

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("DB接続失敗: %v", err)
	}
	defer pool.Close()

	mailer := &mail.ResendMailer{
		APIKey:    resendKey,
		FromEmail: fromEmail,
		FromName:  "市川（ALT-FETISH）",
	}

	// 翌日（Asia/Tokyo）の日付を計算
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		jst = time.UTC
	}
	tomorrow := time.Now().In(jst).AddDate(0, 0, 1).Format("2006-01-02")

	log.Printf("リマインダー対象日: %s", tomorrow)

	// 翌日の confirmed セッション参加者を取得
	rows, err := pool.Query(context.Background(),
		`SELECT a.handle, a.email, a.token,
		        s.session_date, s.session_time
		 FROM sessions s
		 JOIN session_participants sp ON sp.session_id = s.id
		 JOIN applicants a ON a.id = sp.applicant_id
		 WHERE s.status = 'confirmed'
		   AND s.session_date = $1`,
		tomorrow,
	)
	if err != nil {
		log.Fatalf("クエリ失敗: %v", err)
	}
	defer rows.Close()

	sent := 0
	failed := 0

	for rows.Next() {
		var (
			handle      string
			email       string
			token       string
			sessionDate time.Time
			sessionTime time.Time
		)
		if err := rows.Scan(&handle, &email, &token, &sessionDate, &sessionTime); err != nil {
			log.Printf("スキャン失敗: %v", err)
			failed++
			continue
		}

		mypageURL := fmt.Sprintf("%s/my/%s", baseURL, token)
		body := mail.ReminderEmail(
			handle,
			sessionDate.Format("2006年1月2日"),
			sessionTime.Format("15:04"),
			mypageURL,
		)

		if err := mailer.Send(email, "【ALT-FETISH】明日のセッションのご確認", body); err != nil {
			log.Printf("メール送信失敗 %s: %v", email, err)
			failed++
		} else {
			log.Printf("送信済み: %s (%s)", handle, email)
			sent++
		}
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("行読み取りエラー: %v", err)
	}

	log.Printf("完了: 送信%d件 / 失敗%d件", sent, failed)
}
