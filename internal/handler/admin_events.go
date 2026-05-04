package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/ryotaro/yoyaku-multi/internal/mail"
	"github.com/ryotaro/yoyaku-multi/internal/model"
	admin_events "github.com/ryotaro/yoyaku-multi/views/admin/events"
)

// AdminEventsHandler は管理者側の開催日管理を処理する
type AdminEventsHandler struct {
	Events     *model.PostgresEventRepo
	Entries    *model.PostgresEntryRepo
	Applicants *model.PostgresApplicantRepo
	Options    *model.PostgresOptionRepo
	Mailer     *mail.ResendMailer
	BaseURL    string
}

func (h *AdminEventsHandler) List(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	eventList, err := h.Events.ListEvents(statusFilter)
	if err != nil {
		http.Error(w, "開催日一覧の取得に失敗しました: "+err.Error(), http.StatusInternalServerError)
		return
	}
	admin_events.Index(eventList, statusFilter).Render(r.Context(), w)
}

func (h *AdminEventsHandler) ShowNew(w http.ResponseWriter, r *http.Request) {
	optionSets, err := h.Options.ListOptionSets()
	if err != nil {
		http.Error(w, "オプションセットの取得に失敗しました", http.StatusInternalServerError)
		return
	}
	admin_events.New("", optionSets).Render(r.Context(), w)
}

func (h *AdminEventsHandler) Create(w http.ResponseWriter, r *http.Request) {
	optionSets, _ := h.Options.ListOptionSets()
	if err := r.ParseForm(); err != nil {
		admin_events.New("入力データの解析に失敗しました", optionSets).Render(r.Context(), w)
		return
	}

	eventDate, err := time.Parse("2006-01-02", r.FormValue("event_date"))
	if err != nil {
		admin_events.New("日付形式が不正です（YYYY-MM-DD）", optionSets).Render(r.Context(), w)
		return
	}

	eventTime, err := time.Parse("15:04", r.FormValue("event_time"))
	if err != nil {
		admin_events.New("時刻形式が不正です（HH:MM）", optionSets).Render(r.Context(), w)
		return
	}

	capacity := strconvInt(r.FormValue("capacity"))
	if capacity <= 0 {
		admin_events.New("定員は1以上の整数で入力してください", optionSets).Render(r.Context(), w)
		return
	}

	optionSetID, err := parseUUID(r.FormValue("option_set_id"))
	if err != nil {
		admin_events.New("オプションセットを選択してください", optionSets).Render(r.Context(), w)
		return
	}

	e := &model.Event{
		SessionType: "group",
		EventDate:   eventDate,
		EventTime:   eventTime,
		Capacity:    capacity,
		OptionSetID: optionSetID,
		Notes:       r.FormValue("notes"),
	}

	if endTimeStr := r.FormValue("end_time"); endTimeStr != "" {
		et, err := time.Parse("15:04", endTimeStr)
		if err == nil {
			e.EndTime = &et
		}
	}

	if err := h.Events.CreateEvent(e); err != nil {
		admin_events.New("登録に失敗しました: "+err.Error(), optionSets).Render(r.Context(), w)
		return
	}

	// 一斉通知メール
	if r.FormValue("send_notification") == "1" {
		emails, err := h.Events.GetAllApplicantEmails()
		if err == nil {
			eventsURL := h.BaseURL + "/events"
			body := mail.NewEventEmail(
				eventDate.Format("2006年1月2日"),
				eventTime.Format("15:04"),
				capacity,
				eventsURL,
			)
			_ = h.Mailer.SendBulk(emails, "【ALT-FETISH】新しい開催日のお知らせ", body)
		}
	}

	http.Redirect(w, r, "/admin/events", http.StatusSeeOther)
}

func (h *AdminEventsHandler) ShowEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	e, err := h.Events.GetEvent(id)
	if err != nil {
		http.Error(w, "開催日が見つかりません", http.StatusNotFound)
		return
	}

	endTimeStr := ""
	if e.EndTime != nil {
		endTimeStr = e.EndTime.Format("15:04")
	}
	optionSets, err := h.Options.ListOptionSets()
	if err != nil {
		http.Error(w, "オプションセットの取得に失敗しました", http.StatusInternalServerError)
		return
	}

	formData := &admin_events.EventFormData{
		ID:          e.ID.String(),
		EventDate:   e.EventDate.Format("2006-01-02"),
		EventTime:   e.EventTime.Format("15:04"),
		EndTime:     endTimeStr,
		Capacity:    strconv.Itoa(e.Capacity),
		OptionSetID: e.OptionSetID.String(),
		Notes:       e.Notes,
	}

	admin_events.Edit(formData, optionSets, "").Render(r.Context(), w)
}

func (h *AdminEventsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "入力データの解析に失敗しました", http.StatusBadRequest)
		return
	}

	e, err := h.Events.GetEvent(id)
	if err != nil {
		http.Error(w, "開催日が見つかりません", http.StatusNotFound)
		return
	}

	if e.Status != "open" {
		http.Error(w, "open状態の開催日のみ編集できます", http.StatusForbidden)
		return
	}

	eventDate, err := time.Parse("2006-01-02", r.FormValue("event_date"))
	if err != nil {
		http.Error(w, "日付形式が不正です", http.StatusBadRequest)
		return
	}
	eventTime, err := time.Parse("15:04", r.FormValue("event_time"))
	if err != nil {
		http.Error(w, "時刻形式が不正です", http.StatusBadRequest)
		return
	}

	capacity := strconvInt(r.FormValue("capacity"))
	if capacity <= 0 {
		http.Error(w, "定員は1以上の整数で入力してください", http.StatusBadRequest)
		return
	}
	optionSetID, err := parseUUID(r.FormValue("option_set_id"))
	if err != nil {
		http.Error(w, "オプションセットを選択してください", http.StatusBadRequest)
		return
	}

	e.SessionType = "group"
	e.EventDate = eventDate
	e.EventTime = eventTime
	e.Capacity = capacity
	e.OptionSetID = optionSetID
	e.Notes = r.FormValue("notes")

	if endTimeStr := r.FormValue("end_time"); endTimeStr != "" {
		et, err := time.Parse("15:04", endTimeStr)
		if err == nil {
			e.EndTime = &et
		}
	} else {
		e.EndTime = nil
	}

	if err := h.Events.UpdateEvent(e); err != nil {
		http.Error(w, "更新に失敗しました: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/events", http.StatusSeeOther)
}

func (h *AdminEventsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	if err := h.Events.DeleteEvent(id); err != nil {
		http.Error(w, "削除に失敗しました: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// htmxのhx-swapでouterHTMLを空にするため200を返す
	w.WriteHeader(http.StatusOK)
}

func (h *AdminEventsHandler) ShowEntries(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	event, err := h.Events.GetEvent(id)
	if err != nil {
		http.Error(w, "開催日が見つかりません", http.StatusNotFound)
		return
	}

	entryList, err := h.Entries.ListEntriesByEvent(id)
	if err != nil {
		http.Error(w, "参加表明一覧の取得に失敗しました", http.StatusInternalServerError)
		return
	}

	optionSet, err := h.Options.GetOptionSet(event.OptionSetID)
	if err != nil {
		http.Error(w, "オプションセットの取得に失敗しました", http.StatusInternalServerError)
		return
	}

	var entryIDs []uuid.UUID
	for _, e := range entryList {
		entryIDs = append(entryIDs, e.ID)
	}
	selectionMap, err := h.Options.GetSelectionsByEntryIDs(entryIDs)
	if err == nil {
		for _, entry := range entryList {
			entry.SelectedOptionItemIDs = selectionMap[entry.ID]
		}
	}

	admin_events.Entries(event, entryList, optionSet).Render(r.Context(), w)
}

func (h *AdminEventsHandler) CompareEntries(w http.ResponseWriter, r *http.Request) {
	eventID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	ids := r.URL.Query()["ids"]
	if len(ids) != 2 {
		admin_events.CompareView(nil, nil).Render(r.Context(), w)
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

	var result []*model.EntryWithApplicant
	for _, idStr := range ids {
		aID, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		entry, err := h.Entries.GetEntry(eventID, aID)
		if err != nil {
			continue
		}
		ewa := &model.EntryWithApplicant{
			EventEntry: *entry,
		}
		selectionMap, err := h.Options.GetSelectionsByEntryIDs([]uuid.UUID{entry.ID})
		if err == nil {
			ewa.SelectedOptionItemIDs = selectionMap[entry.ID]
		}
		applicant, err := h.Applicants.GetApplicant(aID)
		if err == nil {
			ewa.Handle = applicant.Handle
		}
		result = append(result, ewa)
	}

	admin_events.CompareView(result, optionSet).Render(r.Context(), w)
}

func (h *AdminEventsHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	eventID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "入力データの解析に失敗しました", http.StatusBadRequest)
		return
	}

	applicantIDStrs := r.Form["applicant_ids"]
	if len(applicantIDStrs) == 0 {
		http.Error(w, "参加者を選択してください", http.StatusBadRequest)
		return
	}

	var applicantIDs []uuid.UUID
	for _, idStr := range applicantIDStrs {
		aid, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		applicantIDs = append(applicantIDs, aid)
	}

	event, err := h.Events.GetEvent(eventID)
	if err != nil {
		http.Error(w, "開催日が見つかりません", http.StatusNotFound)
		return
	}
	if len(applicantIDs) > event.Capacity {
		http.Error(w, fmt.Sprintf("選択人数が定員を超えています（定員%d名）", event.Capacity), http.StatusBadRequest)
		return
	}

	_, err = model.ConfirmEntries(h.Events.DB, eventID, applicantIDs)
	if err != nil {
		http.Error(w, "確定処理に失敗しました: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 確定メール送信
	for _, aID := range applicantIDs {
		applicant, err := h.Applicants.GetApplicant(aID)
		if err != nil {
			continue
		}
		body := mail.ConfirmationEmail(
			applicant.Handle,
			event.EventDate.Format("2006年1月2日"),
			event.EventTime.Format("15:04"),
			event.Capacity,
		)
		_ = h.Mailer.Send(applicant.Email, "【ALT-FETISH】参加確定のお知らせ", body)
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/events/%s/entries", eventID.String()), http.StatusSeeOther)
}

func (h *AdminEventsHandler) Decline(w http.ResponseWriter, r *http.Request) {
	eventID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	event, err := h.Events.GetEvent(eventID)
	if err != nil {
		http.Error(w, "開催日が見つかりません", http.StatusNotFound)
		return
	}

	declinedApplicants, err := model.GetDeclinedApplicants(h.Events.DB, eventID)
	if err == nil {
		for _, a := range declinedApplicants {
			body := mail.DeclineEmail(a.Handle, event.EventDate.Format("2006年1月2日"))
			_ = h.Mailer.Send(a.Email, "【ALT-FETISH】参加について", body)
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/events/%s/entries", eventID.String()), http.StatusSeeOther)
}

func (h *AdminEventsHandler) MarkDone(w http.ResponseWriter, r *http.Request) {
	eventID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	if err := model.MarkEventDone(h.Events.DB, eventID); err != nil {
		http.Error(w, "更新に失敗しました: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/events", http.StatusSeeOther)
}

// --- CSV Upload ---

func (h *AdminEventsHandler) ShowUpload(w http.ResponseWriter, r *http.Request) {
	optionSets, err := h.Options.ListOptionSets()
	if err != nil {
		http.Error(w, "オプションセットの取得に失敗しました", http.StatusInternalServerError)
		return
	}
	admin_events.Upload("", optionSets, "").Render(r.Context(), w)
}

func (h *AdminEventsHandler) PreviewCSV(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		admin_events.UploadPreview(nil, nil, "", "ファイルの解析に失敗しました").Render(r.Context(), w)
		return
	}
	optionSetID := r.FormValue("option_set_id")

	file, _, err := r.FormFile("csv_file")
	if err != nil {
		admin_events.UploadPreview(nil, nil, optionSetID, "ファイルが選択されていません").Render(r.Context(), w)
		return
	}
	defer file.Close()

	rows, parseErrors, err := model.ParseCSV(file)
	if err != nil {
		admin_events.UploadPreview(nil, nil, optionSetID, err.Error()).Render(r.Context(), w)
		return
	}

	admin_events.UploadPreview(rows, parseErrors, optionSetID, "").Render(r.Context(), w)
}

func (h *AdminEventsHandler) ConfirmCSV(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "フォームの解析に失敗しました", http.StatusBadRequest)
		return
	}

	// hiddenフィールドからrows[]を取得
	var rows []model.EventRow
	optionSetID, err := parseUUID(r.FormValue("option_set_id"))
	if err != nil {
		http.Error(w, "オプションセットを選択してください", http.StatusBadRequest)
		return
	}
	for i := 0; ; i++ {
		date := r.FormValue(fmt.Sprintf("rows[%d][date]", i))
		if date == "" {
			break
		}
		rows = append(rows, model.EventRow{
			Date:      date,
			StartTime: r.FormValue(fmt.Sprintf("rows[%d][starttime]", i)),
			EndTime:   r.FormValue(fmt.Sprintf("rows[%d][endtime]", i)),
			Capacity:  r.FormValue(fmt.Sprintf("rows[%d][capacity]", i)),
		})
	}

	result := h.Events.BulkCreateEvents(rows, optionSetID)

	// 結果ページを表示
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer
	admin_events.UploadResult(result).Render(r.Context(), &buf)
	// レイアウトで包んで返す
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="UTF-8"><link rel="stylesheet" href="/static/style.css"></head><body class="admin-wrapper"><div style="padding:32px">%s</div></body></html>`, buf.String())
}

// parseUUID はパスパラメータからUUIDをパースする
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// strconvInt は文字列を整数にパースするヘルパー
func strconvInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
