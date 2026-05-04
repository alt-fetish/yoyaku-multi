package handler

import (
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/ryotaro/yoyaku-multi/internal/model"
	"github.com/ryotaro/yoyaku-multi/views/events"
	"github.com/ryotaro/yoyaku-multi/views/my"
)

// EventsHandler は参加者側の開催日・マイページを処理するハンドラ
type EventsHandler struct {
	Events     *model.PostgresEventRepo
	Entries    *model.PostgresEntryRepo
	Applicants *model.PostgresApplicantRepo
	Options    *model.PostgresOptionRepo
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

func (h *EventsHandler) ShowEvent(w http.ResponseWriter, r *http.Request) {
	eventID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なイベントID", http.StatusBadRequest)
		return
	}

	event, err := h.Events.GetEvent(eventID)
	if err != nil {
		http.Error(w, "開催日が見つかりません", http.StatusNotFound)
		return
	}

	optionSet, err := h.Options.GetOptionSet(event.OptionSetID)
	if err != nil {
		http.Error(w, "オプションセットの取得に失敗しました", http.StatusInternalServerError)
		return
	}
	entryCount, err := h.Events.CountActiveEntries(eventID)
	if err != nil {
		entryCount = 0
	}

	applicantToken := r.URL.Query().Get("token")
	if c, err := r.Cookie("applicant_token"); err == nil && applicantToken == "" {
		applicantToken = c.Value
	}

	var (
		entry            *model.EventEntry
		selectedOptionID = make(map[string]bool)
	)
	if applicantToken != "" {
		applicant, err := h.Applicants.GetApplicantByToken(applicantToken)
		if err == nil {
			if existing, err := h.Entries.GetEntry(eventID, applicant.ID); err == nil {
				entry = existing
				selectionMap, err := h.Options.GetSelectionsByEntryIDs([]uuid.UUID{entry.ID})
				if err == nil {
					for _, itemID := range selectionMap[entry.ID] {
						selectedOptionID[itemID.String()] = true
					}
				}
			}
		}
	}

	events.Show(event, optionSet, applicantToken, entry, selectedOptionID, entryCount).Render(r.Context(), w)
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
	if event.Status != "open" {
		http.Error(w, "この開催日は現在申込みできません", http.StatusForbidden)
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
	created, err := h.Entries.CreateEntryIfAvailable(entry)
	if err != nil {
		http.Error(w, "参加表明の登録に失敗しました", http.StatusInternalServerError)
		return
	}
	if !created {
		if r.Header.Get("HX-Request") != "" {
			ew := &model.EventWithCount{Event: *event}
			ew.EntryCount = getEntryCount(h, eventID)
			events.EventCard(ew, applicantToken, false).Render(r.Context(), w)
			return
		}
		http.Error(w, "この開催日は満席です", http.StatusConflict)
		return
	}

	if err := h.Options.ReplaceSelections(entry.ID, parseOptionItemIDs(r)); err != nil {
		http.Error(w, "オプション選択の保存に失敗しました: "+err.Error(), http.StatusBadRequest)
		return
	}

	ew := &model.EventWithCount{Event: *event}
	ew.EntryCount = getEntryCount(h, eventID)
	if r.Header.Get("HX-Request") != "" {
		events.EventCard(ew, applicantToken, created).Render(r.Context(), w)
		return
	}
	http.Redirect(w, r, "/my/"+applicantToken, http.StatusSeeOther)
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

	var optionSetIDs []uuid.UUID
	var entryIDs []uuid.UUID
	for _, entry := range applicantEntries {
		optionSetIDs = append(optionSetIDs, entry.Event.OptionSetID)
		entryIDs = append(entryIDs, entry.ID)
	}

	messages, err := h.Chat.GetMessages(applicant.ID)
	if err != nil {
		messages = nil
	}

	// 管理者からのメッセージを既読に
	_ = h.Chat.MarkAsRead(applicant.ID, "admin")

	optionSets, err := h.Options.GetOptionSetsMap(optionSetIDs)
	if err != nil {
		optionSets = map[uuid.UUID]*model.OptionSet{}
	}
	selections, err := h.Options.GetSelectionsByEntryIDs(entryIDs)
	if err != nil {
		selections = map[uuid.UUID][]uuid.UUID{}
	}

	my.Index(applicant, applicantEntries, stringifyOptionSets(optionSets), stringifySelections(selections), messages).Render(r.Context(), w)
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

func (h *EventsHandler) UpdateEntryOptions(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	applicant, err := h.Applicants.GetApplicantByToken(token)
	if err != nil {
		http.Error(w, "参加者が見つかりません", http.StatusNotFound)
		return
	}

	entryID, err := parseUUID(r.PathValue("entry_id"))
	if err != nil {
		http.Error(w, "無効な申込ID", http.StatusBadRequest)
		return
	}

	var targetEntry *model.EntryWithEvent
	entries, err := h.Entries.ListEntriesByApplicant(applicant.ID)
	if err == nil {
		for _, entry := range entries {
			if entry.ID == entryID {
				targetEntry = entry
				break
			}
		}
	}
	if targetEntry == nil {
		http.Error(w, "申込情報が見つかりません", http.StatusNotFound)
		return
	}

	if err := h.Options.ReplaceSelections(entryID, parseOptionItemIDs(r)); err != nil {
		http.Error(w, "オプションの更新に失敗しました: "+err.Error(), http.StatusBadRequest)
		return
	}

	returnTo := r.FormValue("return_to")
	if returnTo == "" {
		returnTo = "/my/" + token
	}
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func getEntryCount(h *EventsHandler, eventID interface{ String() string }) int {
	id, err := parseUUID(eventID.String())
	if err != nil {
		return 0
	}
	count, err := h.Events.CountActiveEntries(id)
	if err != nil {
		return 0
	}
	return count
}

func parseOptionItemIDs(r *http.Request) []uuid.UUID {
	if err := r.ParseForm(); err != nil {
		return nil
	}

	raw := r.Form["option_item_ids"]
	result := make([]uuid.UUID, 0, len(raw))
	for _, idStr := range raw {
		itemID, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		result = append(result, itemID)
	}
	return result
}

func stringifyOptionSets(sets map[uuid.UUID]*model.OptionSet) map[string]*model.OptionSet {
	result := make(map[string]*model.OptionSet, len(sets))
	for id, set := range sets {
		result[id.String()] = set
	}
	return result
}

func stringifySelections(selections map[uuid.UUID][]uuid.UUID) map[string]map[string]bool {
	result := make(map[string]map[string]bool, len(selections))
	for entryID, itemIDs := range selections {
		key := entryID.String()
		result[key] = make(map[string]bool, len(itemIDs))
		for _, itemID := range itemIDs {
			result[key][itemID.String()] = true
		}
	}
	return result
}

func containsSelection(selected map[string]bool, itemID uuid.UUID) bool {
	return selected[itemID.String()]
}

func selectedOptionLabels(set *model.OptionSet, selectedIDs []uuid.UUID) []string {
	selected := make(map[uuid.UUID]bool, len(selectedIDs))
	for _, id := range selectedIDs {
		selected[id] = true
	}

	var labels []string
	for _, item := range set.Items {
		if selected[item.ID] {
			labels = append(labels, item.Label)
		}
	}
	slices.Sort(labels)
	return labels
}
