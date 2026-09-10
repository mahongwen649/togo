package channels

import (
	"database/sql"
	"encoding/json"
	"errors"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) Create(channel Channel) (Channel, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Channel{}, err
	}
	defer rollback(tx)

	if err := insertChannel(tx, channel); err != nil {
		return Channel{}, err
	}
	if err := replaceModels(tx, channel.ID, channel.Models, channel.CreatedAt); err != nil {
		return Channel{}, err
	}
	if err := tx.Commit(); err != nil {
		return Channel{}, err
	}
	return channel, nil
}

func (s *SQLStore) Update(channel Channel) (Channel, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Channel{}, err
	}
	defer rollback(tx)

	result, err := tx.Exec(
		`UPDATE channels
		 SET name = ?, base_url = ?, encrypted_api_key = ?, updated_at = ?
		 WHERE id = ? AND user_id = ?`,
		channel.Name,
		channel.BaseURL,
		channel.EncryptedAPIKey,
		channel.UpdatedAt,
		channel.ID,
		channel.UserID,
	)
	if err != nil {
		return Channel{}, err
	}
	if err := requireAffected(result); err != nil {
		return Channel{}, err
	}
	if _, err := tx.Exec(`DELETE FROM channel_models WHERE channel_id = ?`, channel.ID); err != nil {
		return Channel{}, err
	}
	if err := replaceModels(tx, channel.ID, channel.Models, channel.UpdatedAt); err != nil {
		return Channel{}, err
	}
	if err := tx.Commit(); err != nil {
		return Channel{}, err
	}
	return channel, nil
}

func (s *SQLStore) Delete(userID string, id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err := tx.Exec(`DELETE FROM channel_models WHERE channel_id = ?`, id); err != nil {
		return err
	}
	result, err := tx.Exec(`DELETE FROM channels WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if err := requireAffected(result); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLStore) List(userID string) ([]Channel, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, name, base_url, encrypted_api_key, created_at, updated_at
		 FROM channels
		 WHERE user_id = ?
		 ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Channel{}
	for rows.Next() {
		channel, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		models, err := s.modelsForChannel(channel.ID)
		if err != nil {
			return nil, err
		}
		channel.Models = models
		items = append(items, channel)
	}
	return items, rows.Err()
}

func (s *SQLStore) ByID(userID string, id string) (Channel, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, name, base_url, encrypted_api_key, created_at, updated_at
		 FROM channels
		 WHERE id = ? AND user_id = ?`,
		id,
		userID,
	)
	channel, err := scanChannel(row)
	if err != nil {
		return Channel{}, err
	}
	models, err := s.modelsForChannel(channel.ID)
	if err != nil {
		return Channel{}, err
	}
	channel.Models = models
	return channel, nil
}

func (s *SQLStore) modelsForChannel(channelID string) ([]ModelEntry, error) {
	rows, err := s.db.Query(
		`SELECT model_id, capabilities_json
		 FROM channel_models
		 WHERE channel_id = ?
		 ORDER BY created_at ASC`,
		channelID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := []ModelEntry{}
	for rows.Next() {
		var model ModelEntry
		var capabilitiesJSON string
		if err := rows.Scan(&model.ID, &capabilitiesJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(capabilitiesJSON), &model.Capabilities); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

func insertChannel(tx *sql.Tx, channel Channel) error {
	_, err := tx.Exec(
		`INSERT INTO channels (id, user_id, name, base_url, encrypted_api_key, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		channel.ID,
		channel.UserID,
		channel.Name,
		channel.BaseURL,
		channel.EncryptedAPIKey,
		channel.CreatedAt,
		channel.UpdatedAt,
	)
	return err
}

func replaceModels(tx *sql.Tx, channelID string, models []ModelEntry, timestamp int64) error {
	for _, model := range models {
		capabilitiesJSON, err := json.Marshal(model.Capabilities)
		if err != nil {
			return err
		}
		_, err = tx.Exec(
			`INSERT INTO channel_models (id, channel_id, model_id, capabilities_json, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			newID("mdl"),
			channelID,
			model.ID,
			string(capabilitiesJSON),
			timestamp,
			timestamp,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

type channelScanner interface {
	Scan(dest ...any) error
}

func scanChannel(scanner channelScanner) (Channel, error) {
	var channel Channel
	if err := scanner.Scan(
		&channel.ID,
		&channel.UserID,
		&channel.Name,
		&channel.BaseURL,
		&channel.EncryptedAPIKey,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Channel{}, ErrNotFound
		}
		return Channel{}, err
	}
	channel.HasAPIKey = channel.EncryptedAPIKey != ""
	return channel, nil
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}

func requireAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
