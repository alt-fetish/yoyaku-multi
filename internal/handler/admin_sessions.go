package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/ryotaro/yoyaku-multi/internal/model"
	admin_sessions "github.com/ryotaro/yoyaku-multi/views/admin/sessions"
)

// AdminSessionsHandler は管理者側のセッション履歴CRUDを処理する
type AdminSessionsHandler struct {
	Sessions   *model.PostgresSessionRepo
	Applicants *model.PostgresApplicantRepo
}

func (h *AdminSessionsHandler) List(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	sessions, err := h.Sessions.ListSessions(statusFilter)
	if err != nil {
		http.Error(w, "セッション一覧の取得に失敗しました: "+err.Error(), http.StatusInternalServerError)
		return
	}
	admin_sessions.Index(sessions, statusFilter).Render(r.Context(), w)
}

func (h *AdminSessionsHandler) ShowNew(w http.ResponseWriter, r *http.Request) {
	applicants, err := h.Applicants.ListApplicants()
	if err != nil {
		applicants = nil
	}
	admin_sessions.New(applicants, "").Render(r.Context(), w)
}

func (h *AdminSessionsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		applicants, _ := h.Applicants.ListApplicants()
		admin_sessions.New(applicants, "入力データの解析に失敗しました").Render(r.Context(), w)
		return
	}

	sessionDate, err := time.Parse("2006-01-02", r.FormValue("session_date"))
	if err != nil {
		applicants, _ := h.Applicants.ListApplicants()
		admin_sessions.New(applicants, "日付形式が不正です").Render(r.Context(), w)
		return
	}

	sessionTime, err := time.Parse("15:04", r.FormValue("session_time"))
	if err != nil {
		applicants, _ := h.Applicants.ListApplicants()
		admin_sessions.New(applicants, "時刻形式が不正です").Render(r.Context(), w)
		return
	}

	sessionType := r.FormValue("session_type")
	status := r.FormValue("status")
	if status == "" {
		status = "confirmed"
	}

	s := &model.Session{
		SessionType: sessionType,
		SessionDate: sessionDate,
		SessionTime: sessionTime,
		Status:      status,
		Notes:       r.FormValue("notes"),
	}

	var applicantIDs []uuid.UUID
	for _, idStr := range r.Form["applicant_ids"] {
		aid, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		applicantIDs = append(applicantIDs, aid)
	}

	if err := h.Sessions.CreateSession(s, applicantIDs); err != nil {
		applicants, _ := h.Applicants.ListApplicants()
		admin_sessions.New(applicants, "登録に失敗しました: "+err.Error()).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, "/admin/sessions", http.StatusSeeOther)
}

func (h *AdminSessionsHandler) ShowEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	s, err := h.Sessions.GetSession(id)
	if err != nil {
		http.Error(w, "セッションが見つかりません", http.StatusNotFound)
		return
	}

	applicants, err := h.Applicants.ListApplicants()
	if err != nil {
		applicants = nil
	}

	admin_sessions.Edit(s, applicants, "").Render(r.Context(), w)
}

func (h *AdminSessionsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "入力データの解析に失敗しました", http.StatusBadRequest)
		return
	}

	sessionDate, err := time.Parse("2006-01-02", r.FormValue("session_date"))
	if err != nil {
		http.Error(w, "日付形式が不正です", http.StatusBadRequest)
		return
	}

	sessionTime, err := time.Parse("15:04", r.FormValue("session_time"))
	if err != nil {
		http.Error(w, "時刻形式が不正です", http.StatusBadRequest)
		return
	}

	s := &model.Session{
		ID:          id,
		SessionType: r.FormValue("session_type"),
		SessionDate: sessionDate,
		SessionTime: sessionTime,
		Status:      r.FormValue("status"),
		Notes:       r.FormValue("notes"),
	}

	var applicantIDs []uuid.UUID
	for _, idStr := range r.Form["applicant_ids"] {
		aid, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		applicantIDs = append(applicantIDs, aid)
	}

	if err := h.Sessions.UpdateSession(s, applicantIDs); err != nil {
		http.Error(w, "更新に失敗しました: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/sessions", http.StatusSeeOther)
}

func (h *AdminSessionsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	if err := h.Sessions.DeleteSession(id); err != nil {
		http.Error(w, "削除に失敗しました: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *AdminSessionsHandler) ParticipantSelect(w http.ResponseWriter, r *http.Request) {
	sessionType := r.URL.Query().Get("session_type")
	if sessionType == "" {
		sessionType = "solo"
	}
	applicants, err := h.Applicants.ListApplicants()
	if err != nil {
		applicants = nil
	}
	admin_sessions.ParticipantSelect(applicants, sessionType, nil).Render(r.Context(), w)
}
