package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) ListContacts(ctx context.Context, householdID int64) ([]Contact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, household_id, name, kind, email, phone, notes, status, created_by, created_at, updated_at
		FROM contacts
		WHERE household_id = $1 AND status <> 'archived'
		ORDER BY name, updated_at DESC
	`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := []Contact{}
	for rows.Next() {
		var contact Contact
		if err := scanContact(rows, &contact); err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}
	return contacts, rows.Err()
}

func (s *Store) CreateContact(ctx context.Context, householdID, userID int64, input ContactInput) (Contact, error) {
	if err := validateContactInput(&input); err != nil {
		return Contact{}, err
	}

	var contact Contact
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO contacts (household_id, name, kind, email, phone, notes, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, household_id, name, kind, email, phone, notes, status, created_by, created_at, updated_at
	`, householdID, input.Name, input.Kind, input.Email, input.Phone, input.Notes, input.Status, userID).Scan(
		&contact.ID, &contact.HouseholdID, &contact.Name, &contact.Kind, &contact.Email, &contact.Phone, &contact.Notes, &contact.Status, &contact.CreatedBy, &contact.CreatedAt, &contact.UpdatedAt,
	)
	return contact, err
}

func (s *Store) GetContact(ctx context.Context, householdID, id int64) (Contact, error) {
	var contact Contact
	err := s.db.QueryRowContext(ctx, `
		SELECT id, household_id, name, kind, email, phone, notes, status, created_by, created_at, updated_at
		FROM contacts
		WHERE household_id = $1 AND id = $2
	`, householdID, id).Scan(
		&contact.ID, &contact.HouseholdID, &contact.Name, &contact.Kind, &contact.Email, &contact.Phone, &contact.Notes, &contact.Status, &contact.CreatedBy, &contact.CreatedAt, &contact.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Contact{}, ErrNotFound
	}
	return contact, err
}

func (s *Store) UpdateContact(ctx context.Context, householdID, id int64, input ContactInput) (Contact, error) {
	if err := validateContactInput(&input); err != nil {
		return Contact{}, err
	}

	var contact Contact
	err := s.db.QueryRowContext(ctx, `
		UPDATE contacts
		SET name = $3,
			kind = $4,
			email = $5,
			phone = $6,
			notes = $7,
			status = $8,
			updated_at = now()
		WHERE household_id = $1 AND id = $2
		RETURNING id, household_id, name, kind, email, phone, notes, status, created_by, created_at, updated_at
	`, householdID, id, input.Name, input.Kind, input.Email, input.Phone, input.Notes, input.Status).Scan(
		&contact.ID, &contact.HouseholdID, &contact.Name, &contact.Kind, &contact.Email, &contact.Phone, &contact.Notes, &contact.Status, &contact.CreatedBy, &contact.CreatedAt, &contact.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Contact{}, ErrNotFound
	}
	return contact, err
}

func (s *Store) ArchiveContact(ctx context.Context, householdID, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE contacts
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

func (s *Store) ListRelatedContacts(ctx context.Context, householdID int64, input RelatedItemInput) ([]RelatedContact, error) {
	if err := s.validateRelatedTarget(ctx, householdID, input); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT cl.id, c.id, c.household_id, c.name, c.kind, c.email, c.phone, c.notes, c.status, c.created_by, c.created_at, c.updated_at
		FROM contact_links cl
		JOIN contacts c ON c.id = cl.contact_id AND c.household_id = cl.household_id
		WHERE cl.household_id = $1
			AND cl.entity_type = $2
			AND cl.entity_id = $3
			AND c.status <> 'archived'
		ORDER BY c.name, cl.created_at DESC
	`, householdID, input.EntityType, input.EntityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := []RelatedContact{}
	for rows.Next() {
		var related RelatedContact
		if err := rows.Scan(&related.LinkID, &related.Contact.ID, &related.Contact.HouseholdID, &related.Contact.Name, &related.Contact.Kind, &related.Contact.Email, &related.Contact.Phone, &related.Contact.Notes, &related.Contact.Status, &related.Contact.CreatedBy, &related.Contact.CreatedAt, &related.Contact.UpdatedAt); err != nil {
			return nil, err
		}
		contacts = append(contacts, related)
	}
	return contacts, rows.Err()
}

func (s *Store) LinkContact(ctx context.Context, householdID, userID, contactID int64, input RelatedItemInput) error {
	if ok, err := s.contactInHousehold(ctx, householdID, contactID); err != nil {
		return err
	} else if !ok {
		return ErrNotFound
	}
	if err := s.validateRelatedTarget(ctx, householdID, input); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO contact_links (household_id, contact_id, entity_type, entity_id, created_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (household_id, contact_id, entity_type, entity_id)
		DO UPDATE SET contact_id = EXCLUDED.contact_id
	`, householdID, contactID, input.EntityType, input.EntityID, userID)
	return err
}

func (s *Store) UnlinkContact(ctx context.Context, householdID, linkID int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM contact_links
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

func validateContactInput(input *ContactInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Kind = strings.TrimSpace(input.Kind)
	input.Email = strings.TrimSpace(input.Email)
	input.Phone = strings.TrimSpace(input.Phone)
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
	return nil
}

func (s *Store) contactInHousehold(ctx context.Context, householdID, contactID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM contacts
			WHERE household_id = $1 AND id = $2 AND status <> 'archived'
		)
	`, householdID, contactID).Scan(&exists)
	return exists, err
}

type contactScanner interface {
	Scan(dest ...any) error
}

func scanContact(scanner contactScanner, contact *Contact) error {
	return scanner.Scan(&contact.ID, &contact.HouseholdID, &contact.Name, &contact.Kind, &contact.Email, &contact.Phone, &contact.Notes, &contact.Status, &contact.CreatedBy, &contact.CreatedAt, &contact.UpdatedAt)
}
