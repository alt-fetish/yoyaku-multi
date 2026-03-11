package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/ryotaro/yoyaku-multi/internal/mail"
	"github.com/ryotaro/yoyaku-multi/internal/model"
	"github.com/ryotaro/yoyaku-multi/views/register"
)

// RegisterHandler は参加者登録を処理するハンドラ
type RegisterHandler struct {
	Applicants *model.PostgresApplicantRepo
	Mailer     *mail.ResendMailer
	BaseURL    string
}

func (h *RegisterHandler) ShowForm(w http.ResponseWriter, r *http.Request) {
	register.Form("").Render(r.Context(), w)
}

func (h *RegisterHandler) Submit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		register.Form("入力データの解析に失敗しました").Render(r.Context(), w)
		return
	}

	handle := r.FormValue("handle")
	email := r.FormValue("email")

	if handle == "" || email == "" {
		register.Form("ハンドルネームとメールアドレスは必須です").Render(r.Context(), w)
		return
	}

	// 既存チェック
	if _, err := h.Applicants.GetApplicantByEmail(email); err == nil {
		register.Form("このメールアドレスは既に登録されています").Render(r.Context(), w)
		return
	}

	// トークン生成
	token, err := generateToken()
	if err != nil {
		register.Form("トークン生成に失敗しました").Render(r.Context(), w)
		return
	}

	// NG設定の収集
	var ngSettings []*model.NGSetting
	for _, key := range model.NGActionKeys {
		value := r.FormValue("ng_" + key)
		ngSettings = append(ngSettings, &model.NGSetting{
			ActionKey: key,
			IsOK:      value == "ok",
		})
	}

	applicant := &model.Applicant{
		Handle: handle,
		Email:  email,
		Token:  token,
	}

	if err := h.Applicants.CreateApplicant(applicant, ngSettings); err != nil {
		register.Form("登録に失敗しました: " + err.Error()).Render(r.Context(), w)
		return
	}

	mypageURL := fmt.Sprintf("%s/my/%s", h.BaseURL, token)

	// メール送信（エラーは無視して続行）
	_ = h.Mailer.Send(email, "【ALT-FETISH】参加者登録完了", mail.RegistrationEmail(handle, mypageURL))

	register.Done(handle, mypageURL).Render(r.Context(), w)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
