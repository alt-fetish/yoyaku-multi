package model

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OptionSet struct {
	ID          uuid.UUID
	Name        string
	Description string
	Items       []*OptionItem
	CreatedAt   time.Time
}

type OptionItem struct {
	ID          uuid.UUID
	OptionSetID uuid.UUID
	Label       string
	SortOrder   int
	CreatedAt   time.Time
}

type PostgresOptionRepo struct {
	DB *pgxpool.Pool
}

func (r *PostgresOptionRepo) ListOptionSets() ([]*OptionSet, error) {
	ctx := context.Background()
	rows, err := r.DB.Query(ctx,
		`SELECT id, name, COALESCE(description,''), created_at
		 FROM option_sets
		 ORDER BY created_at ASC, name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sets []*OptionSet
	var ids []uuid.UUID
	setMap := make(map[uuid.UUID]*OptionSet)
	for rows.Next() {
		set := &OptionSet{}
		if err := rows.Scan(&set.ID, &set.Name, &set.Description, &set.CreatedAt); err != nil {
			return nil, err
		}
		sets = append(sets, set)
		ids = append(ids, set.ID)
		setMap[set.ID] = set
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return sets, nil
	}

	itemRows, err := r.DB.Query(ctx,
		`SELECT id, option_set_id, label, sort_order, created_at
		 FROM option_items
		 WHERE option_set_id = ANY($1)
		 ORDER BY option_set_id, sort_order ASC, created_at ASC`,
		ids,
	)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	for itemRows.Next() {
		item := &OptionItem{}
		if err := itemRows.Scan(&item.ID, &item.OptionSetID, &item.Label, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, err
		}
		if set, ok := setMap[item.OptionSetID]; ok {
			set.Items = append(set.Items, item)
		}
	}
	return sets, itemRows.Err()
}

func (r *PostgresOptionRepo) GetOptionSet(id uuid.UUID) (*OptionSet, error) {
	ctx := context.Background()
	set := &OptionSet{}
	if err := r.DB.QueryRow(ctx,
		`SELECT id, name, COALESCE(description,''), created_at
		 FROM option_sets
		 WHERE id = $1`,
		id,
	).Scan(&set.ID, &set.Name, &set.Description, &set.CreatedAt); err != nil {
		return nil, err
	}

	rows, err := r.DB.Query(ctx,
		`SELECT id, option_set_id, label, sort_order, created_at
		 FROM option_items
		 WHERE option_set_id = $1
		 ORDER BY sort_order ASC, created_at ASC`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		item := &OptionItem{}
		if err := rows.Scan(&item.ID, &item.OptionSetID, &item.Label, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, err
		}
		set.Items = append(set.Items, item)
	}
	return set, rows.Err()
}

func (r *PostgresOptionRepo) GetOptionSetsMap(ids []uuid.UUID) (map[uuid.UUID]*OptionSet, error) {
	result := make(map[uuid.UUID]*OptionSet)
	if len(ids) == 0 {
		return result, nil
	}

	allSets, err := r.ListOptionSets()
	if err != nil {
		return nil, err
	}

	for _, set := range allSets {
		if slices.Contains(ids, set.ID) {
			result[set.ID] = set
		}
	}
	return result, nil
}

func (r *PostgresOptionRepo) CreateOptionSet(set *OptionSet) error {
	items, err := sanitizeOptionItems(set.Items)
	if err != nil {
		return err
	}

	ctx := context.Background()
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx,
		`INSERT INTO option_sets (name, description)
		 VALUES ($1, $2)
		 RETURNING id, created_at`,
		strings.TrimSpace(set.Name), strings.TrimSpace(set.Description),
	).Scan(&set.ID, &set.CreatedAt); err != nil {
		return err
	}

	for i, item := range items {
		if err := tx.QueryRow(ctx,
			`INSERT INTO option_items (option_set_id, label, sort_order)
			 VALUES ($1, $2, $3)
			 RETURNING id, created_at`,
			set.ID, item.Label, i,
		).Scan(&item.ID, &item.CreatedAt); err != nil {
			return err
		}
		item.OptionSetID = set.ID
		item.SortOrder = i
	}
	set.Items = items

	return tx.Commit(ctx)
}

func (r *PostgresOptionRepo) UpdateOptionSet(set *OptionSet) error {
	items, err := sanitizeOptionItems(set.Items)
	if err != nil {
		return err
	}

	ctx := context.Background()
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE option_sets SET name = $1, description = $2 WHERE id = $3`,
		strings.TrimSpace(set.Name), strings.TrimSpace(set.Description), set.ID,
	); err != nil {
		return err
	}

	rows, err := tx.Query(ctx,
		`SELECT id, option_set_id, label, sort_order, created_at
		 FROM option_items
		 WHERE option_set_id = $1
		 ORDER BY sort_order ASC, created_at ASC`,
		set.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var existing []*OptionItem
	for rows.Next() {
		item := &OptionItem{}
		if err := rows.Scan(&item.ID, &item.OptionSetID, &item.Label, &item.SortOrder, &item.CreatedAt); err != nil {
			return err
		}
		existing = append(existing, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i, item := range items {
		if i < len(existing) {
			item.ID = existing[i].ID
			item.OptionSetID = set.ID
			item.SortOrder = i
			item.CreatedAt = existing[i].CreatedAt
			if _, err := tx.Exec(ctx,
				`UPDATE option_items SET label = $1, sort_order = $2 WHERE id = $3`,
				item.Label, i, existing[i].ID,
			); err != nil {
				return err
			}
			continue
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO option_items (option_set_id, label, sort_order)
			 VALUES ($1, $2, $3)
			 RETURNING id, created_at`,
			set.ID, item.Label, i,
		).Scan(&item.ID, &item.CreatedAt); err != nil {
			return err
		}
		item.OptionSetID = set.ID
		item.SortOrder = i
	}

	for _, item := range existing[len(items):] {
		if _, err := tx.Exec(ctx, `DELETE FROM option_items WHERE id = $1`, item.ID); err != nil {
			return err
		}
	}

	set.Items = items
	return tx.Commit(ctx)
}

func (r *PostgresOptionRepo) DeleteOptionSet(id uuid.UUID) error {
	ctx := context.Background()
	var usageCount int
	if err := r.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM events WHERE option_set_id = $1`,
		id,
	).Scan(&usageCount); err != nil {
		return err
	}
	if usageCount > 0 {
		return fmt.Errorf("このオプションセットを使用中の開催イベントがあるため削除できません")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM option_sets WHERE id = $1`, id)
	return err
}

func (r *PostgresOptionRepo) GetSelectionsByEntryIDs(entryIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	result := make(map[uuid.UUID][]uuid.UUID)
	if len(entryIDs) == 0 {
		return result, nil
	}

	ctx := context.Background()
	rows, err := r.DB.Query(ctx,
		`SELECT event_entry_id, option_item_id
		 FROM event_entry_option_selections
		 WHERE event_entry_id = ANY($1)`,
		entryIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var entryID, itemID uuid.UUID
		if err := rows.Scan(&entryID, &itemID); err != nil {
			return nil, err
		}
		result[entryID] = append(result[entryID], itemID)
	}
	return result, rows.Err()
}

func (r *PostgresOptionRepo) ReplaceSelections(entryID uuid.UUID, optionItemIDs []uuid.UUID) error {
	ctx := context.Background()
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	validRows, err := tx.Query(ctx,
		`SELECT oi.id
		 FROM option_items oi
		 JOIN events e ON e.option_set_id = oi.option_set_id
		 JOIN event_entries ee ON ee.event_id = e.id
		 WHERE ee.id = $1`,
		entryID,
	)
	if err != nil {
		return err
	}
	defer validRows.Close()

	validIDs := make(map[uuid.UUID]bool)
	for validRows.Next() {
		var itemID uuid.UUID
		if err := validRows.Scan(&itemID); err != nil {
			return err
		}
		validIDs[itemID] = true
	}
	if err := validRows.Err(); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM event_entry_option_selections WHERE event_entry_id = $1`,
		entryID,
	); err != nil {
		return err
	}

	seen := make(map[uuid.UUID]bool)
	for _, itemID := range optionItemIDs {
		if seen[itemID] {
			continue
		}
		seen[itemID] = true
		if !validIDs[itemID] {
			return fmt.Errorf("無効なオプションが選択されました")
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO event_entry_option_selections (event_entry_id, option_item_id)
			 VALUES ($1, $2)`,
			entryID, itemID,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func sanitizeOptionItems(items []*OptionItem) ([]*OptionItem, error) {
	var result []*OptionItem
	for _, item := range items {
		label := strings.TrimSpace(item.Label)
		if label == "" {
			continue
		}
		result = append(result, &OptionItem{Label: label})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("オプションは1件以上必要です")
	}
	return result, nil
}
