// Package store provides a local SQLite cache for Power BI data.
// It uses modernc.org/sqlite (pure Go, no CGO) so the Windows
// cross-compile pipeline keeps working.
package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite cache database.
type Store struct {
	db *sql.DB
}

// Open creates/opens the cache DB. The DSN's _txlock=immediate makes
// database/sql transactions issue BEGIN IMMEDIATE, so write
// transactions (MAX(id)+1 inserts, cache replaces) are atomic under
// concurrent access on modernc.org/sqlite.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?_txlock=immediate")
	if err != nil {
		return nil, err
	}
	// modernc/sqlite handles concurrent access poorly without these.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// OpenDefault opens the cache under the user config dir
// (e.g. %AppData%/search-grid-template), creating the directory if
// needed. Falls back to the CWD only when no config dir is available.
func OpenDefault() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Open(filepath.Join(".", "docs-cache.db"))
	}
	dir = filepath.Join(dir, "search-grid-template")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return Open(filepath.Join(dir, "docs-cache.db"))
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS docs (
		id       INTEGER PRIMARY KEY,
		title    TEXT NOT NULL,
		author   TEXT,
		category TEXT,
		updated  TEXT,
		status   TEXT,
		synced_at TEXT NOT NULL,
		pending  INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		return err
	}
	// Caches created before the pending flag lack the column; add it.
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM pragma_table_info('docs') WHERE name = 'pending'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`ALTER TABLE docs ADD COLUMN pending INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	// meta holds the issued-id high-water mark (next_id) so deleting the
	// highest row no longer recycles its id. Older caches start empty and
	// fall back to MAX(id)+1 on the first insert.
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`); err != nil {
		return err
	}
	return nil
}

// ReplaceDocs atomically replaces the cached docs with the given set
// and stamps the sync time. Rows still flagged pending (their Power BI
// push has not succeeded yet) are kept so a sync never loses them;
// they are re-pushed on the next sync instead.
func (s *Store) ReplaceDocs(docs []Doc) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM docs WHERE pending = 0"); err != nil {
		return err
	}
	// INSERT OR REPLACE: a kept pending row may also come back in the
	// fresh set once its re-push has succeeded.
	stmt, err := tx.Prepare("INSERT OR REPLACE INTO docs (id,title,author,category,updated,status,synced_at) VALUES (?,?,?,?,?,?,datetime('now'))")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, d := range docs {
		if _, err := stmt.Exec(d.ID, d.Title, d.Author, d.Category, d.Updated, d.Status); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListDocs returns all cached docs ordered by id.
func (s *Store) ListDocs() ([]Doc, error) {
	rows, err := s.db.Query("SELECT id,title,author,category,updated,status FROM docs ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	docs := []Doc{}
	for rows.Next() {
		var d Doc
		if err := rows.Scan(&d.ID, &d.Title, &d.Author, &d.Category, &d.Updated, &d.Status); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// PendingDocs returns the rows whose Power BI push has not succeeded
// yet (pending = 1), oldest id first.
func (s *Store) PendingDocs() ([]Doc, error) {
	rows, err := s.db.Query("SELECT id,title,author,category,updated,status FROM docs WHERE pending = 1 ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	docs := []Doc{}
	for rows.Next() {
		var d Doc
		if err := rows.Scan(&d.ID, &d.Title, &d.Author, &d.Category, &d.Updated, &d.Status); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// MarkPending flags one row as awaiting a Power BI re-push.
func (s *Store) MarkPending(id int64, pending bool) error {
	v := 0
	if pending {
		v = 1
	}
	_, err := s.db.Exec("UPDATE docs SET pending = ? WHERE id = ?", v, id)
	return err
}

// ClearPending clears the pending flag of the given ids after their
// re-push has succeeded.
func (s *Store) ClearPending(ids []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare("UPDATE docs SET pending = 0 WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LastSync returns the most recent sync timestamp (empty if never).
func (s *Store) LastSync() (string, error) {
	var t sql.NullString
	err := s.db.QueryRow("SELECT max(synced_at) FROM docs").Scan(&t)
	if err != nil {
		return "", err
	}
	return t.String, nil
}

// Doc mirrors the Power BI Docs table (and frontend Doc type).
type Doc struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Author   string `json:"author"`
	Category string `json:"category"`
	Updated  string `json:"updated"`
	Status   string `json:"status"`
}

// DeleteDoc removes a row by id from the local cache.
func (s *Store) DeleteDoc(id int64) error {
	_, err := s.db.Exec("DELETE FROM docs WHERE id = ?", id)
	return err
}

// Close closes the database.

func (s *Store) Close() error { return s.db.Close() }

// InsertDoc inserts one doc into the local cache with a new id.
// Ids are monotonic per cache: the next id is max(issued-id high-water
// mark from meta, MAX(id)+1), so deleting the highest row (the push
// dataset delete path re-uploads the survivors) never recycles an id
// that Power BI has already seen. Read + INSERT + counter bump share one
// transaction, and the connection's BEGIN IMMEDIATE (_txlock=immediate)
// keeps concurrent writers from grabbing the same id.
func (s *Store) InsertDoc(d *Doc) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var maxID sql.NullInt64
	if err := tx.QueryRow(`SELECT MAX(id) FROM docs`).Scan(&maxID); err != nil {
		return err
	}
	next := int64(1)
	if maxID.Valid && maxID.Int64 >= next {
		next = maxID.Int64 + 1
	}
	// A stale/copied cache may still hold a higher high-water mark than the
	// rows it has; the counter wins so ids never go backwards.
	var issued sql.NullString
	if err := tx.QueryRow(`SELECT value FROM meta WHERE key = 'next_id'`).Scan(&issued); err != nil && err != sql.ErrNoRows {
		return err
	}
	if issued.Valid {
		if v, perr := strconv.ParseInt(issued.String, 10, 64); perr == nil && v > next {
			next = v
		}
	}

	d.ID = next
	if _, err := tx.Exec(
		`INSERT INTO docs (id, title, author, category, updated, status, synced_at) VALUES (?,?,?,?,?,?,datetime('now'))`,
		d.ID, d.Title, d.Author, d.Category, d.Updated, d.Status); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO meta (key, value) VALUES ('next_id', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		strconv.FormatInt(next+1, 10)); err != nil {
		return err
	}
	return tx.Commit()
}
