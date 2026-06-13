package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) ListDocuments(ctx context.Context, householdID int64) ([]Document, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, household_id, title, description, url, kind, status, file_name, file_path, content_type, file_size, created_by, created_at, updated_at
		FROM documents
		WHERE household_id = $1 AND status <> 'archived'
		ORDER BY updated_at DESC, created_at DESC
	`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	documents := []Document{}
	for rows.Next() {
		var document Document
		if err := scanDocument(rows, &document); err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	return documents, rows.Err()
}

func (s *Store) CreateDocument(ctx context.Context, householdID, userID int64, input DocumentInput) (Document, error) {
	if err := validateDocumentInput(&input); err != nil {
		return Document{}, err
	}

	var document Document
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO documents (household_id, title, description, url, kind, status, file_name, file_path, content_type, file_size, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, household_id, title, description, url, kind, status, file_name, file_path, content_type, file_size, created_by, created_at, updated_at
	`, householdID, input.Title, input.Description, input.URL, input.Kind, input.Status, input.FileName, input.FilePath, input.ContentType, input.FileSize, userID).Scan(
		&document.ID, &document.HouseholdID, &document.Title, &document.Description, &document.URL, &document.Kind, &document.Status, &document.FileName, &document.FilePath, &document.ContentType, &document.FileSize, &document.CreatedBy, &document.CreatedAt, &document.UpdatedAt,
	)
	return document, err
}

func (s *Store) GetDocument(ctx context.Context, householdID, id int64) (Document, error) {
	var document Document
	err := s.db.QueryRowContext(ctx, `
		SELECT id, household_id, title, description, url, kind, status, file_name, file_path, content_type, file_size, created_by, created_at, updated_at
		FROM documents
		WHERE household_id = $1 AND id = $2
	`, householdID, id).Scan(
		&document.ID, &document.HouseholdID, &document.Title, &document.Description, &document.URL, &document.Kind, &document.Status, &document.FileName, &document.FilePath, &document.ContentType, &document.FileSize, &document.CreatedBy, &document.CreatedAt, &document.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	return document, err
}

func (s *Store) UpdateDocument(ctx context.Context, householdID, id int64, input DocumentInput) (Document, error) {
	existing, err := s.GetDocument(ctx, householdID, id)
	if errors.Is(err, ErrNotFound) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, err
	}
	if input.FilePath == "" {
		input.FileName = existing.FileName
		input.FilePath = existing.FilePath
		input.ContentType = existing.ContentType
		input.FileSize = existing.FileSize
	}
	if err := validateDocumentInput(&input); err != nil {
		return Document{}, err
	}

	var document Document
	err = s.db.QueryRowContext(ctx, `
		UPDATE documents
		SET title = $3,
			description = $4,
			url = $5,
			kind = $6,
			status = $7,
			file_name = $8,
			file_path = $9,
			content_type = $10,
			file_size = $11,
			updated_at = now()
		WHERE household_id = $1 AND id = $2
		RETURNING id, household_id, title, description, url, kind, status, file_name, file_path, content_type, file_size, created_by, created_at, updated_at
	`, householdID, id, input.Title, input.Description, input.URL, input.Kind, input.Status, input.FileName, input.FilePath, input.ContentType, input.FileSize).Scan(
		&document.ID, &document.HouseholdID, &document.Title, &document.Description, &document.URL, &document.Kind, &document.Status, &document.FileName, &document.FilePath, &document.ContentType, &document.FileSize, &document.CreatedBy, &document.CreatedAt, &document.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	return document, err
}

func (s *Store) ArchiveDocument(ctx context.Context, householdID, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE documents
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

func (s *Store) ListRelatedItemsForDocument(ctx context.Context, householdID, documentID int64) ([]RelatedItem, error) {
	if ok, err := s.documentInHousehold(ctx, householdID, documentID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT dl.id, dl.entity_type, dl.entity_id,
			CASE dl.entity_type
				WHEN 'project' THEN p.title
				WHEN 'task' THEN t.title
				WHEN 'asset' THEN a.name
				ELSE ''
			END AS title,
			CASE dl.entity_type
				WHEN 'project' THEN p.status || ' project'
				WHEN 'task' THEN COALESCE(t.status, '') || CASE WHEN t.assigned_to IS NULL THEN '' ELSE ' task' END
				WHEN 'asset' THEN a.kind || ' asset'
				ELSE ''
			END AS subtitle,
			CASE dl.entity_type
				WHEN 'project' THEN '/projects/' || p.id::text
				WHEN 'task' THEN '/tasks/' || t.id::text
				WHEN 'asset' THEN '/assets/' || a.id::text
				ELSE '#'
			END AS url,
			dl.created_at
		FROM document_links dl
		LEFT JOIN projects p ON dl.entity_type = 'project' AND p.household_id = dl.household_id AND p.id = dl.entity_id
		LEFT JOIN tasks t ON dl.entity_type = 'task' AND t.household_id = dl.household_id AND t.id = dl.entity_id
		LEFT JOIN assets a ON dl.entity_type = 'asset' AND a.household_id = dl.household_id AND a.id = dl.entity_id
		WHERE dl.household_id = $1 AND dl.document_id = $2
		ORDER BY dl.created_at DESC
	`, householdID, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []RelatedItem{}
	for rows.Next() {
		var item RelatedItem
		if err := rows.Scan(&item.LinkID, &item.Type, &item.ID, &item.Title, &item.Subtitle, &item.URL, &item.CreatedAt); err != nil {
			return nil, err
		}
		if item.Title != "" {
			items = append(items, item)
		}
	}
	return items, rows.Err()
}

func (s *Store) ListRelatedDocuments(ctx context.Context, householdID int64, input RelatedItemInput) ([]RelatedDocument, error) {
	if err := s.validateRelatedTarget(ctx, householdID, input); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT dl.id, d.id, d.household_id, d.title, d.description, d.url, d.kind, d.status, d.file_name, d.file_path, d.content_type, d.file_size, d.created_by, d.created_at, d.updated_at
		FROM document_links dl
		JOIN documents d ON d.id = dl.document_id AND d.household_id = dl.household_id
		WHERE dl.household_id = $1
			AND dl.entity_type = $2
			AND dl.entity_id = $3
			AND d.status <> 'archived'
		ORDER BY d.updated_at DESC, dl.created_at DESC
	`, householdID, input.EntityType, input.EntityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	documents := []RelatedDocument{}
	for rows.Next() {
		var related RelatedDocument
		if err := rows.Scan(&related.LinkID, &related.Document.ID, &related.Document.HouseholdID, &related.Document.Title, &related.Document.Description, &related.Document.URL, &related.Document.Kind, &related.Document.Status, &related.Document.FileName, &related.Document.FilePath, &related.Document.ContentType, &related.Document.FileSize, &related.Document.CreatedBy, &related.Document.CreatedAt, &related.Document.UpdatedAt); err != nil {
			return nil, err
		}
		documents = append(documents, related)
	}
	return documents, rows.Err()
}

func (s *Store) LinkDocument(ctx context.Context, householdID, userID, documentID int64, input RelatedItemInput) (RelatedItem, error) {
	if ok, err := s.documentInHousehold(ctx, householdID, documentID); err != nil {
		return RelatedItem{}, err
	} else if !ok {
		return RelatedItem{}, ErrNotFound
	}
	if err := s.validateRelatedTarget(ctx, householdID, input); err != nil {
		return RelatedItem{}, err
	}

	var linkID int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO document_links (household_id, document_id, entity_type, entity_id, created_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (household_id, document_id, entity_type, entity_id)
		DO UPDATE SET document_id = EXCLUDED.document_id
		RETURNING id
	`, householdID, documentID, input.EntityType, input.EntityID, userID).Scan(&linkID)
	if err != nil {
		return RelatedItem{}, err
	}

	items, err := s.ListRelatedItemsForDocument(ctx, householdID, documentID)
	if err != nil {
		return RelatedItem{}, err
	}
	for _, item := range items {
		if item.LinkID == linkID {
			return item, nil
		}
	}
	return RelatedItem{LinkID: linkID, Type: input.EntityType, ID: input.EntityID}, nil
}

func (s *Store) UnlinkDocument(ctx context.Context, householdID, linkID int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM document_links
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

func validateDocumentInput(input *DocumentInput) error {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.URL = strings.TrimSpace(input.URL)
	input.Kind = strings.TrimSpace(input.Kind)
	input.Status = strings.TrimSpace(input.Status)
	input.FileName = strings.TrimSpace(input.FileName)
	input.FilePath = strings.TrimSpace(input.FilePath)
	input.ContentType = strings.TrimSpace(input.ContentType)
	if input.Title == "" {
		return errors.New("title is required")
	}
	if input.URL == "" && input.FilePath == "" {
		return errors.New("url or file is required")
	}
	if input.FilePath != "" && input.FileName == "" {
		return errors.New("file name is required")
	}
	if input.FileSize < 0 {
		return errors.New("file size is invalid")
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
	return nil
}

func (s *Store) validateRelatedTarget(ctx context.Context, householdID int64, input RelatedItemInput) error {
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
	case "asset":
		ok, err := s.assetInHousehold(ctx, householdID, input.EntityID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("asset must belong to this household")
		}
	default:
		return errors.New("related item type must be project, task, or asset")
	}
	return nil
}

func (s *Store) documentInHousehold(ctx context.Context, householdID, documentID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM documents
			WHERE household_id = $1 AND id = $2 AND status <> 'archived'
		)
	`, householdID, documentID).Scan(&exists)
	return exists, err
}

func (s *Store) taskInHousehold(ctx context.Context, householdID, taskID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM tasks
			WHERE household_id = $1 AND id = $2 AND status <> 'archived'
		)
	`, householdID, taskID).Scan(&exists)
	return exists, err
}

type documentScanner interface {
	Scan(dest ...any) error
}

func scanDocument(scanner documentScanner, document *Document) error {
	return scanner.Scan(&document.ID, &document.HouseholdID, &document.Title, &document.Description, &document.URL, &document.Kind, &document.Status, &document.FileName, &document.FilePath, &document.ContentType, &document.FileSize, &document.CreatedBy, &document.CreatedAt, &document.UpdatedAt)
}
