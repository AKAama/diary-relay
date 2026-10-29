package draft

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Draft struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	Filename  string    `json:"filename"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS drafts (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  tags TEXT NOT NULL DEFAULT '[]',
  filename TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);`)
	return err
}

func (s *Store) Create(d *Draft) error {
	tags, err := json.Marshal(d.Tags)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO drafts (id, title, content, tags, filename, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Title, d.Content, string(tags), d.Filename,
		d.CreatedAt.UTC().Format(time.RFC3339Nano),
		d.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) Get(id string) (*Draft, error) {
	row := s.db.QueryRow(
		`SELECT id, title, content, tags, filename, created_at, updated_at FROM drafts WHERE id = ?`, id,
	)
	return scanDraft(row)
}

func (s *Store) Update(d *Draft) error {
	tags, err := json.Marshal(d.Tags)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(
		`UPDATE drafts SET title=?, content=?, tags=?, filename=?, updated_at=? WHERE id=?`,
		d.Title, d.Content, string(tags), d.Filename,
		d.UpdatedAt.UTC().Format(time.RFC3339Nano), d.ID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("draft not found: %s", d.ID)
	}
	return nil
}

func (s *Store) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM drafts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("draft not found: %s", id)
	}
	return nil
}


// List returns all drafts ordered by updated_at descending (newest first).
func (s *Store) List() ([]*Draft, error) {
	rows, err := s.db.Query(
		`SELECT id, title, content, tags, filename, created_at, updated_at FROM drafts ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Draft
	for rows.Next() {
		d, err := scanDraft(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []*Draft{}
	}
	return out, nil
}

// FilenamesForPrefix returns filenames in the store that match a prefix
// (e.g. "DailyJournal-1218") — used when allocating -2/-3 suffixes locally.
func (s *Store) FilenamesForPrefix(prefix string) ([]string, error) {
	rows, err := s.db.Query(`SELECT filename FROM drafts WHERE filename LIKE ?`, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanDraft(row scannable) (*Draft, error) {
	var d Draft
	var tagsJSON, created, updated string
	if err := row.Scan(&d.ID, &d.Title, &d.Content, &tagsJSON, &d.Filename, &created, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("draft not found")
		}
		return nil, err
	}
	if err := json.Unmarshal([]byte(tagsJSON), &d.Tags); err != nil {
		d.Tags = nil
	}
	var err error
	d.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		d.CreatedAt, _ = time.Parse(time.RFC3339, created)
	}
	d.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		d.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	}
	return &d, nil
}
