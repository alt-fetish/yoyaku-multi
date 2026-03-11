package handler

import (
	"net/http"

	"github.com/ryotaro/yoyaku-multi/internal/model"
	admin_chat "github.com/ryotaro/yoyaku-multi/views/admin/chat"
)

// AdminChatHandler は管理者側のチャットを処理する
type AdminChatHandler struct {
	Applicants *model.PostgresApplicantRepo
	Chat       *model.PostgresChatRepo
}

func (h *AdminChatHandler) Show(w http.ResponseWriter, r *http.Request) {
	applicantID, err := parseUUID(r.PathValue("applicant_id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	applicant, err := h.Applicants.GetApplicant(applicantID)
	if err != nil {
		http.Error(w, "参加者が見つかりません", http.StatusNotFound)
		return
	}

	messages, err := h.Chat.GetMessages(applicantID)
	if err != nil {
		messages = nil
	}

	// 参加者からのメッセージを既読に
	_ = h.Chat.MarkAsRead(applicantID, "applicant")

	admin_chat.Show(applicant, messages).Render(r.Context(), w)
}

func (h *AdminChatHandler) PostMessage(w http.ResponseWriter, r *http.Request) {
	applicantID, err := parseUUID(r.PathValue("applicant_id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	body := r.FormValue("body")
	if body == "" {
		http.Error(w, "メッセージが空です", http.StatusBadRequest)
		return
	}

	msg := &model.ChatMessage{
		ApplicantID: applicantID,
		Sender:      "admin",
		Body:        body,
	}
	_ = h.Chat.PostMessage(msg)

	messages, _ := h.Chat.GetMessages(applicantID)
	admin_chat.Messages(messages).Render(r.Context(), w)
}

func (h *AdminChatHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	applicantID, err := parseUUID(r.PathValue("applicant_id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}
	_ = h.Chat.MarkAsRead(applicantID, "applicant")
	messages, _ := h.Chat.GetMessages(applicantID)
	admin_chat.Messages(messages).Render(r.Context(), w)
}
