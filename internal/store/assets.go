package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Store) ListAssets(ctx context.Context, householdID int64) ([]Asset, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, household_id, name, kind, serial_number, warranty_expires_at, purchase_date, purchase_cost::float8, vendor, model, maintenance_cadence, maintenance_next_due_at, maintenance_last_completed_at, notes, status, created_by, created_at, updated_at
		FROM assets
		WHERE household_id = $1 AND status <> 'archived'
		ORDER BY name, updated_at DESC
	`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assets := []Asset{}
	for rows.Next() {
		var asset Asset
		if err := scanAsset(rows, &asset); err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (s *Store) CreateAsset(ctx context.Context, householdID, userID int64, input AssetInput) (Asset, error) {
	if err := validateAssetInput(&input); err != nil {
		return Asset{}, err
	}

	var asset Asset
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO assets (household_id, name, kind, serial_number, warranty_expires_at, purchase_date, purchase_cost, vendor, model, maintenance_cadence, maintenance_next_due_at, notes, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, household_id, name, kind, serial_number, warranty_expires_at, purchase_date, purchase_cost::float8, vendor, model, maintenance_cadence, maintenance_next_due_at, maintenance_last_completed_at, notes, status, created_by, created_at, updated_at
	`, householdID, input.Name, input.Kind, input.SerialNumber, input.WarrantyExpiresAt, input.PurchaseDate, input.PurchaseCost, input.Vendor, input.Model, input.MaintenanceCadence, input.MaintenanceNextDueAt, input.Notes, input.Status, userID).Scan(
		&asset.ID, &asset.HouseholdID, &asset.Name, &asset.Kind, &asset.SerialNumber, &asset.WarrantyExpiresAt, &asset.PurchaseDate, &asset.PurchaseCost, &asset.Vendor, &asset.Model, &asset.MaintenanceCadence, &asset.MaintenanceNextDueAt, &asset.MaintenanceLastCompletedAt, &asset.Notes, &asset.Status, &asset.CreatedBy, &asset.CreatedAt, &asset.UpdatedAt,
	)
	return asset, err
}

func (s *Store) GetAsset(ctx context.Context, householdID, id int64) (Asset, error) {
	var asset Asset
	err := s.db.QueryRowContext(ctx, `
		SELECT id, household_id, name, kind, serial_number, warranty_expires_at, purchase_date, purchase_cost::float8, vendor, model, maintenance_cadence, maintenance_next_due_at, maintenance_last_completed_at, notes, status, created_by, created_at, updated_at
		FROM assets
		WHERE household_id = $1 AND id = $2
	`, householdID, id).Scan(
		&asset.ID, &asset.HouseholdID, &asset.Name, &asset.Kind, &asset.SerialNumber, &asset.WarrantyExpiresAt, &asset.PurchaseDate, &asset.PurchaseCost, &asset.Vendor, &asset.Model, &asset.MaintenanceCadence, &asset.MaintenanceNextDueAt, &asset.MaintenanceLastCompletedAt, &asset.Notes, &asset.Status, &asset.CreatedBy, &asset.CreatedAt, &asset.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	return asset, err
}

func (s *Store) UpdateAsset(ctx context.Context, householdID, id int64, input AssetInput) (Asset, error) {
	if err := validateAssetInput(&input); err != nil {
		return Asset{}, err
	}

	var asset Asset
	err := s.db.QueryRowContext(ctx, `
		UPDATE assets
		SET name = $3,
			kind = $4,
			serial_number = $5,
			warranty_expires_at = $6,
			purchase_date = $7,
			purchase_cost = $8,
			vendor = $9,
			model = $10,
			maintenance_cadence = $11,
			maintenance_next_due_at = $12,
			notes = $13,
			status = $14,
			updated_at = now()
		WHERE household_id = $1 AND id = $2
		RETURNING id, household_id, name, kind, serial_number, warranty_expires_at, purchase_date, purchase_cost::float8, vendor, model, maintenance_cadence, maintenance_next_due_at, maintenance_last_completed_at, notes, status, created_by, created_at, updated_at
	`, householdID, id, input.Name, input.Kind, input.SerialNumber, input.WarrantyExpiresAt, input.PurchaseDate, input.PurchaseCost, input.Vendor, input.Model, input.MaintenanceCadence, input.MaintenanceNextDueAt, input.Notes, input.Status).Scan(
		&asset.ID, &asset.HouseholdID, &asset.Name, &asset.Kind, &asset.SerialNumber, &asset.WarrantyExpiresAt, &asset.PurchaseDate, &asset.PurchaseCost, &asset.Vendor, &asset.Model, &asset.MaintenanceCadence, &asset.MaintenanceNextDueAt, &asset.MaintenanceLastCompletedAt, &asset.Notes, &asset.Status, &asset.CreatedBy, &asset.CreatedAt, &asset.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	return asset, err
}

func (s *Store) ArchiveAsset(ctx context.Context, householdID, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE assets
		SET status = 'archived', updated_at = now()
		WHERE household_id = $1 AND id = $2
	`, householdID, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListRelatedAssets(ctx context.Context, householdID int64, input RelatedItemInput) ([]RelatedAsset, error) {
	if err := s.validateAssetLinkTarget(ctx, householdID, input); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT al.id, a.id, a.household_id, a.name, a.kind, a.serial_number, a.warranty_expires_at, a.purchase_date, a.purchase_cost::float8, a.vendor, a.model, a.maintenance_cadence, a.maintenance_next_due_at, a.maintenance_last_completed_at, a.notes, a.status, a.created_by, a.created_at, a.updated_at
		FROM asset_links al
		JOIN assets a ON a.id = al.asset_id AND a.household_id = al.household_id
		WHERE al.household_id = $1
			AND al.entity_type = $2
			AND al.entity_id = $3
			AND a.status <> 'archived'
		ORDER BY a.name, al.created_at DESC
	`, householdID, input.EntityType, input.EntityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assets := []RelatedAsset{}
	for rows.Next() {
		var related RelatedAsset
		if err := rows.Scan(&related.LinkID, &related.Asset.ID, &related.Asset.HouseholdID, &related.Asset.Name, &related.Asset.Kind, &related.Asset.SerialNumber, &related.Asset.WarrantyExpiresAt, &related.Asset.PurchaseDate, &related.Asset.PurchaseCost, &related.Asset.Vendor, &related.Asset.Model, &related.Asset.MaintenanceCadence, &related.Asset.MaintenanceNextDueAt, &related.Asset.MaintenanceLastCompletedAt, &related.Asset.Notes, &related.Asset.Status, &related.Asset.CreatedBy, &related.Asset.CreatedAt, &related.Asset.UpdatedAt); err != nil {
			return nil, err
		}
		assets = append(assets, related)
	}
	return assets, rows.Err()
}

func (s *Store) LinkAsset(ctx context.Context, householdID, userID, assetID int64, input RelatedItemInput) error {
	if ok, err := s.assetInHousehold(ctx, householdID, assetID); err != nil {
		return err
	} else if !ok {
		return ErrNotFound
	}
	if err := s.validateAssetLinkTarget(ctx, householdID, input); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO asset_links (household_id, asset_id, entity_type, entity_id, created_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (household_id, asset_id, entity_type, entity_id)
		DO UPDATE SET asset_id = EXCLUDED.asset_id
	`, householdID, assetID, input.EntityType, input.EntityID, userID)
	return err
}

func (s *Store) UnlinkAsset(ctx context.Context, householdID, linkID int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM asset_links
		WHERE household_id = $1 AND id = $2
	`, householdID, linkID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListAssetMaintenanceItems(ctx context.Context, householdID, assetID int64) ([]AssetMaintenanceItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, household_id, asset_id, title, notes, cadence, status, next_due_at, last_completed_at, created_by, created_at, updated_at
		FROM asset_maintenance_items
		WHERE household_id = $1 AND asset_id = $2 AND status <> 'archived'
		ORDER BY COALESCE(next_due_at, '9999-12-31'::timestamptz), title
	`, householdID, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AssetMaintenanceItem{}
	for rows.Next() {
		var item AssetMaintenanceItem
		if err := scanAssetMaintenanceItem(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateAssetMaintenanceItem(ctx context.Context, householdID, userID, assetID int64, input AssetMaintenanceInput) (AssetMaintenanceItem, error) {
	if ok, err := s.assetInHousehold(ctx, householdID, assetID); err != nil {
		return AssetMaintenanceItem{}, err
	} else if !ok {
		return AssetMaintenanceItem{}, ErrNotFound
	}
	if err := validateAssetMaintenanceInput(&input); err != nil {
		return AssetMaintenanceItem{}, err
	}

	var item AssetMaintenanceItem
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO asset_maintenance_items (household_id, asset_id, title, notes, cadence, status, next_due_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, household_id, asset_id, title, notes, cadence, status, next_due_at, last_completed_at, created_by, created_at, updated_at
	`, householdID, assetID, input.Title, input.Notes, input.Cadence, input.Status, input.NextDueAt, userID).Scan(
		&item.ID, &item.HouseholdID, &item.AssetID, &item.Title, &item.Notes, &item.Cadence, &item.Status, &item.NextDueAt, &item.LastCompletedAt, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func (s *Store) GetAssetMaintenanceItem(ctx context.Context, householdID, id int64) (AssetMaintenanceItem, error) {
	var item AssetMaintenanceItem
	err := s.db.QueryRowContext(ctx, `
		SELECT id, household_id, asset_id, title, notes, cadence, status, next_due_at, last_completed_at, created_by, created_at, updated_at
		FROM asset_maintenance_items
		WHERE household_id = $1 AND id = $2
	`, householdID, id).Scan(
		&item.ID, &item.HouseholdID, &item.AssetID, &item.Title, &item.Notes, &item.Cadence, &item.Status, &item.NextDueAt, &item.LastCompletedAt, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return AssetMaintenanceItem{}, ErrNotFound
	}
	return item, err
}

func (s *Store) UpdateAssetMaintenanceItem(ctx context.Context, householdID, id int64, input AssetMaintenanceInput) (AssetMaintenanceItem, error) {
	if err := validateAssetMaintenanceInput(&input); err != nil {
		return AssetMaintenanceItem{}, err
	}

	var item AssetMaintenanceItem
	err := s.db.QueryRowContext(ctx, `
		UPDATE asset_maintenance_items
		SET title = $3,
			notes = $4,
			cadence = $5,
			status = $6,
			next_due_at = $7,
			updated_at = now()
		WHERE household_id = $1 AND id = $2
		RETURNING id, household_id, asset_id, title, notes, cadence, status, next_due_at, last_completed_at, created_by, created_at, updated_at
	`, householdID, id, input.Title, input.Notes, input.Cadence, input.Status, input.NextDueAt).Scan(
		&item.ID, &item.HouseholdID, &item.AssetID, &item.Title, &item.Notes, &item.Cadence, &item.Status, &item.NextDueAt, &item.LastCompletedAt, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return AssetMaintenanceItem{}, ErrNotFound
	}
	return item, err
}

func (s *Store) ArchiveAssetMaintenanceItem(ctx context.Context, householdID, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE asset_maintenance_items
		SET status = 'archived', updated_at = now()
		WHERE household_id = $1 AND id = $2
	`, householdID, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GenerateAssetMaintenanceItemTask(ctx context.Context, householdID, userID, id int64) (Task, error) {
	item, err := s.GetAssetMaintenanceItem(ctx, householdID, id)
	if err != nil {
		return Task{}, err
	}
	if item.Status == "archived" {
		return Task{}, errors.New("maintenance item is archived")
	}
	asset, err := s.GetAsset(ctx, householdID, item.AssetID)
	if err != nil {
		return Task{}, err
	}
	if asset.Status == "archived" {
		return Task{}, errors.New("asset is archived")
	}
	if item.NextDueAt != nil {
		task, err := s.getOpenAssetMaintenanceTaskForDue(ctx, item.ID, *item.NextDueAt)
		if err == nil {
			return task, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Task{}, err
		}
	}

	input := TaskInput{
		AssetID:                &asset.ID,
		AssetMaintenanceItemID: &item.ID,
		Title:                  item.Title,
		Notes:                  assetMaintenanceTaskNotes(asset, item),
		Priority:               "normal",
		DueAt:                  item.NextDueAt,
	}
	task, err := s.CreateTask(ctx, householdID, userID, input)
	if err != nil {
		return Task{}, err
	}
	if err := s.LinkAsset(ctx, householdID, userID, asset.ID, RelatedItemInput{EntityType: "task", EntityID: task.ID}); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Store) getOpenAssetMaintenanceTaskForDue(ctx context.Context, itemID int64, dueAt time.Time) (Task, error) {
	var task Task
	err := s.db.QueryRowContext(ctx, `
		SELECT t.id, t.household_id, t.project_id, t.project_folder_id, t.routine_id, t.asset_id, t.asset_maintenance_item_id, t.assigned_to, COALESCE(assigned.name, ''), t.title, t.notes, t.status, t.priority, t.due_at, t.completed_at, t.created_by, t.created_at, t.updated_at
		FROM tasks t
		LEFT JOIN users assigned ON assigned.id = t.assigned_to
		WHERE t.asset_maintenance_item_id = $1
			AND t.status = 'open'
			AND t.due_at IS NOT DISTINCT FROM $2
		LIMIT 1
	`, itemID, dueAt).Scan(
		&task.ID, &task.HouseholdID, &task.ProjectID, &task.ProjectFolderID, &task.RoutineID, &task.AssetID, &task.AssetMaintenanceItemID, &task.AssignedTo, &task.AssignedName, &task.Title, &task.Notes, &task.Status, &task.Priority, &task.DueAt, &task.CompletedAt, &task.CreatedBy, &task.CreatedAt, &task.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return task, err
}

func (s *Store) GenerateDueAssetMaintenanceTasks(ctx context.Context, now time.Time) (int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH inserted AS (
			INSERT INTO tasks (household_id, asset_id, asset_maintenance_item_id, title, notes, priority, due_at, created_by)
			SELECT i.household_id, i.asset_id, i.id, i.title,
				CASE
					WHEN i.notes = '' THEN 'Asset maintenance for ' || a.name
					ELSE 'Asset maintenance for ' || a.name || E'\n\n' || i.notes
				END,
				'normal', i.next_due_at, i.created_by
			FROM asset_maintenance_items i
			JOIN assets a ON a.id = i.asset_id AND a.household_id = i.household_id
			WHERE i.status = 'active'
				AND a.status = 'active'
				AND i.next_due_at IS NOT NULL
				AND i.next_due_at <= $1
				AND NOT EXISTS (
					SELECT 1
					FROM tasks t
					WHERE t.asset_maintenance_item_id = i.id
						AND t.status = 'open'
						AND t.due_at IS NOT DISTINCT FROM i.next_due_at
				)
			ON CONFLICT DO NOTHING
			RETURNING id, household_id, asset_id, created_by
		), linked AS (
			INSERT INTO asset_links (household_id, asset_id, entity_type, entity_id, created_by)
			SELECT household_id, asset_id, 'task', id, created_by
			FROM inserted
			ON CONFLICT DO NOTHING
		)
		SELECT id FROM inserted
	`, now)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var count int64
	for rows.Next() {
		count++
	}
	return count, rows.Err()
}

func (s *Store) CompleteAssetMaintenance(ctx context.Context, householdID, id int64) error {
	item, err := s.GetAssetMaintenanceItem(ctx, householdID, id)
	if err != nil {
		return err
	}

	nextDue, err := nextDueForCadence(item.NextDueAt, item.Cadence)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE asset_maintenance_items
		SET last_completed_at = now(),
			next_due_at = $3,
			updated_at = now()
		WHERE household_id = $1 AND id = $2
	`, householdID, id, nextDue)
	return err
}

func assetMaintenanceTaskNotes(asset Asset, item AssetMaintenanceItem) string {
	note := fmt.Sprintf("Asset maintenance for %s", asset.Name)
	if item.Notes != "" {
		note += "\n\n" + item.Notes
	}
	return note
}

func validateAssetInput(input *AssetInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Kind = strings.TrimSpace(input.Kind)
	input.SerialNumber = strings.TrimSpace(input.SerialNumber)
	input.Vendor = strings.TrimSpace(input.Vendor)
	input.Model = strings.TrimSpace(input.Model)
	input.MaintenanceCadence = strings.TrimSpace(input.MaintenanceCadence)
	input.Notes = strings.TrimSpace(input.Notes)
	input.Status = strings.TrimSpace(input.Status)
	if input.Name == "" {
		return errors.New("name is required")
	}
	if input.Kind == "" {
		input.Kind = "general"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Status != "active" && input.Status != "archived" {
		return errors.New("status must be active or archived")
	}
	if input.PurchaseCost != nil && *input.PurchaseCost < 0 {
		return errors.New("purchase cost must be zero or greater")
	}
	if input.MaintenanceCadence == "" {
		input.MaintenanceNextDueAt = nil
	} else if !validCadence(input.MaintenanceCadence) {
		input.MaintenanceCadence = ""
		input.MaintenanceNextDueAt = nil
	}
	return nil
}

func validateAssetMaintenanceInput(input *AssetMaintenanceInput) error {
	input.Title = strings.TrimSpace(input.Title)
	input.Notes = strings.TrimSpace(input.Notes)
	input.Cadence = strings.TrimSpace(input.Cadence)
	input.Status = strings.TrimSpace(input.Status)
	if input.Title == "" {
		return errors.New("title is required")
	}
	if input.Cadence == "" {
		input.Cadence = "monthly"
	}
	if !validCadence(input.Cadence) {
		return errors.New("cadence must be daily, weekly, monthly, quarterly, or yearly")
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Status != "active" && input.Status != "archived" {
		return errors.New("status must be active or archived")
	}
	return nil
}

func (s *Store) validateAssetLinkTarget(ctx context.Context, householdID int64, input RelatedItemInput) error {
	if input.EntityID <= 0 {
		return errors.New("related item is required")
	}
	switch input.EntityType {
	case "project":
		ok, err := s.projectInHousehold(ctx, householdID, input.EntityID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("project must belong to this household")
		}
	case "task":
		ok, err := s.taskInHousehold(ctx, householdID, input.EntityID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("task must belong to this household")
		}
	default:
		return errors.New("related item type must be project or task")
	}
	return nil
}

func (s *Store) assetInHousehold(ctx context.Context, householdID, assetID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM assets
			WHERE household_id = $1 AND id = $2 AND status <> 'archived'
		)
	`, householdID, assetID).Scan(&exists)
	return exists, err
}

func (s *Store) assetMaintenanceItemInHousehold(ctx context.Context, householdID, id int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM asset_maintenance_items
			WHERE household_id = $1 AND id = $2 AND status <> 'archived'
		)
	`, householdID, id).Scan(&exists)
	return exists, err
}

type assetScanner interface {
	Scan(dest ...any) error
}

func scanAsset(scanner assetScanner, asset *Asset) error {
	return scanner.Scan(&asset.ID, &asset.HouseholdID, &asset.Name, &asset.Kind, &asset.SerialNumber, &asset.WarrantyExpiresAt, &asset.PurchaseDate, &asset.PurchaseCost, &asset.Vendor, &asset.Model, &asset.MaintenanceCadence, &asset.MaintenanceNextDueAt, &asset.MaintenanceLastCompletedAt, &asset.Notes, &asset.Status, &asset.CreatedBy, &asset.CreatedAt, &asset.UpdatedAt)
}

func scanAssetMaintenanceItem(scanner assetScanner, item *AssetMaintenanceItem) error {
	return scanner.Scan(&item.ID, &item.HouseholdID, &item.AssetID, &item.Title, &item.Notes, &item.Cadence, &item.Status, &item.NextDueAt, &item.LastCompletedAt, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt)
}
