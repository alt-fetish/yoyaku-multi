package handler

import (
	"net/http"

	"github.com/ryotaro/yoyaku-multi/internal/model"
	"github.com/ryotaro/yoyaku-multi/views/events"
	"github.com/ryotaro/yoyaku-multi/views/my"
)

// EventsHandler は参加者側の開催日・マイページを処理するハンドラ
type EventsHandler struct {
	Events     *model.PostgresEventRepo
	Entries    *model.PostgresEntryRepo
	Applicants *model.PostgresApplicantRepo
	Chat       *model.PostgresChatRepo
}

// ListEvents は開催日一覧を表示する
func (h *EventsHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	publicEvents, err := h.Events.ListPublicEvents()
	if err != nil {
		http.Error(w, "開催日の取得に失敗しました", http.StatusInternalServerError)
		return
	}

	// クッキーまたはクエリパラメータからトークン取得
	applicantToken := r.URL.Query().Get("token")
	if c, err := r.Cookie("applicant_token"); err == nil && applicantToken == "" {
		applicantToken = c.Value
	}

	entryEventIDs := make(map[string]bool)
	if applicantToken != "" {
		applicant, err := h.Applicants.GetApplicantByToken(applicantToken)
		if err == nil {
			applicantEntries, err := h.Entries.ListEntriesByApplicant(applicant.ID)
			if err == nil {
				for _, e := range applicantEntries {
					if e.Status == "pending" || e.Status == "confirmed" {
						entryEventIDs[e.EventID.String()] = true
					}
				}
			}
		}
	}

	events.Index(publicEvents, applicantToken, entryEventIDs).Render(r.Context(), w)
}

// ToggleEntry は参加表明のトグルを処理する（POST: 表明、DELETE: 取消）
func (h *EventsHandler) ToggleEntry(w http.ResponseWriter, r *http.Request) {
	eventID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なイベントID", http.StatusBadRequest)
		return
	}

	applicantToken := r.URL.Query().Get("token")
	if applicantToken == "" {
		http.Error(w, "トークンが必要です", http.StatusUnauthorized)
		return
	}

	applicant, err := h.Applicants.GetApplicantByToken(applicantToken)
	if err != nil {
		http.Error(w, "参加者が見つかりません", http.StatusNotFound)
		return
	}

	event, err := h.Events.GetEvent(eventID)
	if err != nil {
		http.Error(w, "開催日が見つかりません", http.StatusNotFound)
		return
	}

	if r.Method == http.MethodDelete {
		// 取消
		entry, err := h.Entries.GetEntry(eventID, applicant.ID)
		if err == nil {
			_ = h.Entries.DeleteEntry(entry.ID)
		}
		// 更新後のイベントカードを返す
		ew := &model.EventWithCount{Event: *event}
		ew.EntryCount = getEntryCount(h, eventID)
		events.EventCard(ew, applicantToken, false).Render(r.Context(), w)
		return
	}

	// 参加表明
	entry := &model.EventEntry{
		EventID:     eventID,
		ApplicantID: applicant.ID,
	}
	_ = h.Entries.CreateEntry(entry)

	ew := &model.EventWithCount{Event: *event}
	ew.EntryCount = getEntryCount(h, eventID)
	events.EventCard(ew, applicantToken, true).Render(r.Context(), w)
}

// MyPage はマイページを表示する
func (h *EventsHandler) MyPage(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	applicant, err := h.Applicants.GetApplicantByToken(token)
	if err != nil {
		http.Error(w, "マイページが見つかりません", http.StatusNotFound)
		return
	}

	// トークンをCookieにセット（開催日一覧からも申込できるように）
	http.SetCookie(w, &http.Cookie{
		Name:     "applicant_token",
		Value:    token,
		Path:     "/",
		MaxAge:   86400 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	applicantEntries, err := h.Entries.ListEntriesByApplicant(applicant.ID)
	if err != nil {
		applicantEntries = nil
	}

	ngSettings, err := h.Applicants.GetNGSettings(applicant.ID)
	if err != nil {
		ngSettings = nil
	}

	messages, err := h.Chat.GetMessages(applicant.ID)
	if err != nil {
		messages = nil
	}

	// 管理者からのメッセージを既読に
	_ = h.Chat.MarkAsRead(applicant.ID, "admin")

	my.Index(applicant, applicantEntries, ngSettings, messages).Render(r.Context(), w)
}

// MyPageChat はマイページからチャットメッセージを送信する
func (h *EventsHandler) MyPageChat(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	applicant, err := h.Applicants.GetApplicantByToken(token)
	if err != nil {
		http.Error(w, "参加者が見つかりません", http.StatusNotFound)
		return
	}

	body := r.FormValue("body")
	if body == "" {
		http.Error(w, "メッセージが空です", http.StatusBadRequest)
		return
	}

	msg := &model.ChatMessage{
		ApplicantID: applicant.ID,
		Sender:      "applicant",
		Body:        body,
	}
	_ = h.Chat.PostMessage(msg)

	messages, _ := h.Chat.GetMessages(applicant.ID)
	my.ChatMessages(messages).Render(r.Context(), w)
}

// MyPageChatMessages はポーリング用のメッセージ一覧を返す
func (h *EventsHandler) MyPageChatMessages(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	applicant, err := h.Applicants.GetApplicantByToken(token)
	if err != nil {
		http.Error(w, "参加者が見つかりません", http.StatusNotFound)
		return
	}
	_ = h.Chat.MarkAsRead(applicant.ID, "admin")
	messages, _ := h.Chat.GetMessages(applicant.ID)
	my.ChatMessages(messages).Render(r.Context(), w)
}

func getEntryCount(h *EventsHandler, eventID interface{ String() string }) int {
	return 0 // シンプル化: 実際はDBから取得
}
