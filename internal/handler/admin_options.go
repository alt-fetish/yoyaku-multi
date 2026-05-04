package handler

import (
	"net/http"
	"strings"

	"github.com/ryotaro/yoyaku-multi/internal/model"
	admin_options "github.com/ryotaro/yoyaku-multi/views/admin/options"
)

type AdminOptionsHandler struct {
	Options *model.PostgresOptionRepo
}

func (h *AdminOptionsHandler) List(w http.ResponseWriter, r *http.Request) {
	optionSets, err := h.Options.ListOptionSets()
	if err != nil {
		http.Error(w, "オプションセットの取得に失敗しました: "+err.Error(), http.StatusInternalServerError)
		return
	}
	admin_options.Index(optionSets).Render(r.Context(), w)
}

func (h *AdminOptionsHandler) ShowNew(w http.ResponseWriter, r *http.Request) {
	admin_options.New(&admin_options.OptionSetFormData{}, "").Render(r.Context(), w)
}

func (h *AdminOptionsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		admin_options.New(&admin_options.OptionSetFormData{}, "入力データの解析に失敗しました").Render(r.Context(), w)
		return
	}

	formData := &admin_options.OptionSetFormData{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		ItemsText:   r.FormValue("items_text"),
	}
	set := &model.OptionSet{
		Name:        formData.Name,
		Description: formData.Description,
		Items:       parseOptionItems(formData.ItemsText),
	}
	if err := h.Options.CreateOptionSet(set); err != nil {
		admin_options.New(formData, err.Error()).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, "/admin/options", http.StatusSeeOther)
}

func (h *AdminOptionsHandler) ShowEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	set, err := h.Options.GetOptionSet(id)
	if err != nil {
		http.Error(w, "オプションセットが見つかりません", http.StatusNotFound)
		return
	}

	admin_options.Edit(toOptionSetFormData(set), "").Render(r.Context(), w)
}

func (h *AdminOptionsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "入力データの解析に失敗しました", http.StatusBadRequest)
		return
	}

	formData := &admin_options.OptionSetFormData{
		ID:          id.String(),
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		ItemsText:   r.FormValue("items_text"),
	}
	set := &model.OptionSet{
		ID:          id,
		Name:        formData.Name,
		Description: formData.Description,
		Items:       parseOptionItems(formData.ItemsText),
	}
	if err := h.Options.UpdateOptionSet(set); err != nil {
		admin_options.Edit(formData, err.Error()).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, "/admin/options", http.StatusSeeOther)
}

func (h *AdminOptionsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "無効なID", http.StatusBadRequest)
		return
	}

	if err := h.Options.DeleteOptionSet(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/admin/options", http.StatusSeeOther)
}

func parseOptionItems(itemsText string) []*model.OptionItem {
	lines := strings.Split(itemsText, "\n")
	items := make([]*model.OptionItem, 0, len(lines))
	for _, line := range lines {
		label := strings.TrimSpace(line)
		if label == "" {
			continue
		}
		items = append(items, &model.OptionItem{Label: label})
	}
	return items
}

func toOptionSetFormData(set *model.OptionSet) *admin_options.OptionSetFormData {
	labels := make([]string, 0, len(set.Items))
	for _, item := range set.Items {
		labels = append(labels, item.Label)
	}
	return &admin_options.OptionSetFormData{
		ID:          set.ID.String(),
		Name:        set.Name,
		Description: set.Description,
		ItemsText:   strings.Join(labels, "\n"),
	}
}
