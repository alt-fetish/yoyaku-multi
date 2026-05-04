package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/ryotaro/yoyaku-multi/internal/model"
	admin_applicants "github.com/ryotaro/yoyaku-multi/views/admin/applicants"
)

// AdminApplicantsHandler は管理者側の参加者管理を処理する
type AdminApplicantsHandler struct {
	Applicants *model.PostgresApplicantRepo
	Entries    *model.PostgresEntryRepo
	Options    *model.PostgresOptionRepo
}

func (h *AdminApplicantsHandler) List(w http.ResponseWriter, r *http.Request) {
	applicants, err := h.Applicants.ListApplicants()
	if err != nil {
		http.Error(w, "参加者一覧の取得に失敗しました: "+err.Error(), http.StatusInternalServerError)
		return
	}
	admin_applicants.Index(applicants).Render(r.Context(), w)
}

func (h *AdminApplicantsHandler) Show(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	applicant, err := h.Applicants.GetApplicant(id)
	if err != nil {
		http.Error(w, "参加者が見つかりません", http.StatusNotFound)
		return
	}

	entries, err := h.Entries.ListEntriesByApplicant(id)
	if err != nil {
		entries = nil
	}

	var optionSetIDs []uuid.UUID
	var entryIDs []uuid.UUID
	for _, entry := range entries {
		optionSetIDs = append(optionSetIDs, entry.Event.OptionSetID)
		entryIDs = append(entryIDs, entry.ID)
	}

	optionSets, err := h.Options.GetOptionSetsMap(optionSetIDs)
	if err != nil {
		optionSets = map[uuid.UUID]*model.OptionSet{}
	}
	selections, err := h.Options.GetSelectionsByEntryIDs(entryIDs)
	if err != nil {
		selections = map[uuid.UUID][]uuid.UUID{}
	}

	admin_applicants.Show(applicant, entries, optionSets, selections).Render(r.Context(), w)
}
