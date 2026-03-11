package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/ryotaro/yoyaku-multi/internal/handler"
	"github.com/ryotaro/yoyaku-multi/internal/mail"
	"github.com/ryotaro/yoyaku-multi/internal/middleware"
	"github.com/ryotaro/yoyaku-multi/internal/model"
	admin_view "github.com/ryotaro/yoyaku-multi/views/admin"
)

func main() {
	// 環境変数ロード
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	resendKey := os.Getenv("RESEND_API_KEY")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	sessionSecret := os.Getenv("SESSION_SECRET")
	baseURL := os.Getenv("BASE_URL")
	fromEmail := os.Getenv("FROM_EMAIL")

	if adminPassword == "" {
		log.Fatal("ADMIN_PASSWORD is required")
	}
	if sessionSecret == "" {
		log.Fatal("SESSION_SECRET is required")
	}

	// DB接続
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("DB接続失敗: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("DBのping失敗: %v", err)
	}
	log.Println("DB接続成功")

	// セッションストア初期化
	middleware.Store = sessions.NewCookieStore([]byte(sessionSecret))
	middleware.Store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30, // 30日
		HttpOnly: true,
		Secure:   os.Getenv("ENV") == "production",
		SameSite: http.SameSiteLaxMode,
	}

	// メーラー初期化
	mailer := &mail.ResendMailer{
		APIKey:    resendKey,
		FromEmail: fromEmail,
		FromName:  "市川（ALT-FETISH）",
	}

	// リポジトリ初期化
	applicantRepo := &model.PostgresApplicantRepo{DB: pool}
	eventRepo := &model.PostgresEventRepo{DB: pool}
	entryRepo := &model.PostgresEntryRepo{DB: pool}
	sessionRepo := &model.PostgresSessionRepo{DB: pool}
	chatRepo := &model.PostgresChatRepo{DB: pool}

	// ハンドラ初期化
	registerHandler := &handler.RegisterHandler{
		Applicants: applicantRepo,
		Mailer:     mailer,
		BaseURL:    baseURL,
	}

	eventsHandler := &handler.EventsHandler{
		Events:     eventRepo,
		Entries:    entryRepo,
		Applicants: applicantRepo,
		Chat:       chatRepo,
	}

	adminEventsHandler := &handler.AdminEventsHandler{
		Events:     eventRepo,
		Entries:    entryRepo,
		Applicants: applicantRepo,
		Mailer:     mailer,
		BaseURL:    baseURL,
	}

	adminApplicantsHandler := &handler.AdminApplicantsHandler{
		Applicants: applicantRepo,
		Entries:    entryRepo,
	}

	adminSessionsHandler := &handler.AdminSessionsHandler{
		Sessions:   sessionRepo,
		Applicants: applicantRepo,
	}

	adminChatHandler := &handler.AdminChatHandler{
		Applicants: applicantRepo,
		Chat:       chatRepo,
	}

	mux := http.NewServeMux()

	// 静的ファイル
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// ルートリダイレクト
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/events", http.StatusFound)
	})

	// --- 参加者側 ---
	mux.HandleFunc("GET /register", registerHandler.ShowForm)
	mux.HandleFunc("POST /register", registerHandler.Submit)

	mux.HandleFunc("GET /events", eventsHandler.ListEvents)
	mux.HandleFunc("POST /events/{id}/entry", eventsHandler.ToggleEntry)
	mux.HandleFunc("DELETE /events/{id}/entry", eventsHandler.ToggleEntry)

	mux.HandleFunc("GET /my/{token}", eventsHandler.MyPage)
	mux.HandleFunc("POST /my/{token}/chat", eventsHandler.MyPageChat)
	mux.HandleFunc("GET /my/{token}/chat/messages", eventsHandler.MyPageChatMessages)

	// --- 管理者ログイン ---
	mux.HandleFunc("GET /admin/login", func(w http.ResponseWriter, r *http.Request) {
		if middleware.IsAuthenticated(r) {
			http.Redirect(w, r, "/admin/events", http.StatusFound)
			return
		}
		admin_view.Login("").Render(r.Context(), w)
	})

	mux.HandleFunc("POST /admin/login", func(w http.ResponseWriter, r *http.Request) {
		password := r.FormValue("password")
		if password != adminPassword {
			admin_view.Login("パスワードが違います").Render(r.Context(), w)
			return
		}
		if err := middleware.SetAuthenticated(w, r); err != nil {
			admin_view.Login("セッションの作成に失敗しました").Render(r.Context(), w)
			return
		}
		http.Redirect(w, r, "/admin/events", http.StatusSeeOther)
	})

	mux.HandleFunc("GET /admin/logout", func(w http.ResponseWriter, r *http.Request) {
		_ = middleware.ClearAuthenticated(w, r)
		http.Redirect(w, r, "/admin/login", http.StatusFound)
	})

	// --- 管理者エリア（認証必須） ---
	adminMux := http.NewServeMux()

	// 開催日管理
	adminMux.HandleFunc("GET /admin/events", adminEventsHandler.List)
	adminMux.HandleFunc("GET /admin/events/new", adminEventsHandler.ShowNew)
	adminMux.HandleFunc("POST /admin/events", adminEventsHandler.Create)
	adminMux.HandleFunc("GET /admin/events/upload", adminEventsHandler.ShowUpload)
	adminMux.HandleFunc("POST /admin/events/upload/preview", adminEventsHandler.PreviewCSV)
	adminMux.HandleFunc("POST /admin/events/upload/confirm", adminEventsHandler.ConfirmCSV)
	adminMux.HandleFunc("GET /admin/events/{id}/edit", adminEventsHandler.ShowEdit)
	adminMux.HandleFunc("POST /admin/events/{id}/edit", adminEventsHandler.Update) // _method=PUTを受ける
	adminMux.HandleFunc("DELETE /admin/events/{id}", adminEventsHandler.Delete)
	adminMux.HandleFunc("GET /admin/events/{id}/entries", adminEventsHandler.ShowEntries)
	adminMux.HandleFunc("GET /admin/events/{id}/entries/compare", adminEventsHandler.CompareEntries)
	adminMux.HandleFunc("POST /admin/events/{id}/confirm", adminEventsHandler.Confirm)
	adminMux.HandleFunc("POST /admin/events/{id}/decline", adminEventsHandler.Decline)
	adminMux.HandleFunc("POST /admin/events/{id}/done", adminEventsHandler.MarkDone)

	// 参加者管理
	adminMux.HandleFunc("GET /admin/applicants", adminApplicantsHandler.List)
	adminMux.HandleFunc("GET /admin/applicants/{id}", adminApplicantsHandler.Show)

	// セッション履歴
	adminMux.HandleFunc("GET /admin/sessions", adminSessionsHandler.List)
	adminMux.HandleFunc("GET /admin/sessions/new", adminSessionsHandler.ShowNew)
	adminMux.HandleFunc("POST /admin/sessions", adminSessionsHandler.Create)
	adminMux.HandleFunc("GET /admin/sessions/participant-select", adminSessionsHandler.ParticipantSelect)
	adminMux.HandleFunc("GET /admin/sessions/{id}/edit", adminSessionsHandler.ShowEdit)
	adminMux.HandleFunc("POST /admin/sessions/{id}/edit", adminSessionsHandler.Update)
	adminMux.HandleFunc("DELETE /admin/sessions/{id}", adminSessionsHandler.Delete)

	// チャット
	adminMux.HandleFunc("GET /admin/chat/{applicant_id}", adminChatHandler.Show)
	adminMux.HandleFunc("POST /admin/chat/{applicant_id}", adminChatHandler.PostMessage)
	adminMux.HandleFunc("GET /admin/chat/{applicant_id}/messages", adminChatHandler.GetMessages)

	// 認証ミドルウェアで管理者エリアを保護
	mux.Handle("/admin/", middleware.RequireAdmin(adminMux))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("サーバー起動: http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("サーバー起動失敗: %v", err)
	}
}
