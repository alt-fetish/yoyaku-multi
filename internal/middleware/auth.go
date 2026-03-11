package middleware

import (
	"net/http"

	"github.com/gorilla/sessions"
)

const sessionName = "admin-session"
const sessionKeyAuthenticated = "authenticated"

// Store はセッションストア（main.goで初期化）
var Store *sessions.CookieStore

// RequireAdmin は管理者認証ミドルウェア
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := Store.Get(r, sessionName)
		if err != nil || session.Values[sessionKeyAuthenticated] != true {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SetAuthenticated はセッションに認証フラグをセットする
func SetAuthenticated(w http.ResponseWriter, r *http.Request) error {
	session, err := Store.Get(r, sessionName)
	if err != nil {
		return err
	}
	session.Values[sessionKeyAuthenticated] = true
	return session.Save(r, w)
}

// ClearAuthenticated はセッションを破棄する
func ClearAuthenticated(w http.ResponseWriter, r *http.Request) error {
	session, err := Store.Get(r, sessionName)
	if err != nil {
		return err
	}
	session.Options.MaxAge = -1
	return session.Save(r, w)
}

// IsAuthenticated はリクエストが認証済みかどうかを返す
func IsAuthenticated(r *http.Request) bool {
	session, err := Store.Get(r, sessionName)
	if err != nil {
		return false
	}
	return session.Values[sessionKeyAuthenticated] == true
}
