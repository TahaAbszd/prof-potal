package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db    *sql.DB
	cache *Cache
}

func (s *Store) SetCache(cache *Cache) { s.cache = cache }

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, query := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA foreign_keys=ON`,
		`CREATE TABLE IF NOT EXISTS professors (slug TEXT PRIMARY KEY, data TEXT NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, role TEXT NOT NULL CHECK(role IN ('admin','professor')), professor_slug TEXT UNIQUE REFERENCES professors(slug))`,
		`CREATE TABLE IF NOT EXISTS sessions (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, expires_at INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS faculties (name TEXT PRIMARY KEY COLLATE NOCASE, created_at INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS professor_stats (slug TEXT PRIMARY KEY REFERENCES professors(slug) ON DELETE CASCADE, views INTEGER NOT NULL DEFAULT 0, searches INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS engagement_events (visitor_hash TEXT NOT NULL, slug TEXT NOT NULL REFERENCES professors(slug) ON DELETE CASCADE, kind TEXT NOT NULL, day TEXT NOT NULL, PRIMARY KEY(visitor_hash,slug,kind,day))`,
		`CREATE TABLE IF NOT EXISTS source_imports (source TEXT NOT NULL, source_id TEXT NOT NULL, slug TEXT NOT NULL REFERENCES professors(slug) ON DELETE CASCADE, imported_at INTEGER NOT NULL, PRIMARY KEY(source,source_id))`,
	} {
		if _, err := db.Exec(query); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Seed(path, adminUsername, adminPassword, facultyPassword string) error {
	ctx := context.Background()
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM professors`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read seed: %w", err)
		}
		var professors []Professor
		if err := json.Unmarshal(content, &professors); err != nil {
			return err
		}
		for _, professor := range professors {
			professor.Normalize()
			if err := professor.Validate(); err != nil {
				return err
			}
			if err := s.InsertProfessor(ctx, professor); err != nil {
				return err
			}
		}
	}
	// Keep existing installations intact while introducing the faculty catalog and statistics.
	profiles, err := s.ListProfessors(ctx)
	if err != nil {
		return err
	}
	for _, professor := range profiles {
		if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO faculties(name,created_at) VALUES(?,?)`, professor.Faculty, time.Now().Unix()); err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO professor_stats(slug) VALUES(?)`, professor.Slug); err != nil {
			return err
		}
	}
	var adminCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&adminCount); err != nil {
		return err
	}
	if adminCount == 0 {
		if adminUsername == "" || adminPassword == "" {
			return errors.New("BOOTSTRAP_ADMIN_USERNAME and BOOTSTRAP_ADMIN_PASSWORD are required on first run")
		}
		if err := s.CreateUser(ctx, adminUsername, adminPassword, "admin", ""); err != nil {
			return err
		}
	}
	// Demo professor accounts are created only for the two profiles imported from the supplied HTML.
	for username, slug := range map[string]string{"golzari": "shahram-golzari-hormozi", "daryanavard": "seyed-hassan-daryanavard"} {
		var existing int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE professor_slug=?`, slug).Scan(&existing); err != nil {
			return err
		}
		if existing == 0 {
			if facultyPassword == "" {
				return errors.New("BOOTSTRAP_FACULTY_PASSWORD is required on first run")
			}
			if err := s.CreateUser(ctx, username, facultyPassword, "professor", slug); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) ListProfessors(ctx context.Context) ([]Professor, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT data FROM professors ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Professor{}
	for rows.Next() {
		var raw string
		var p Professor
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			return nil, err
		}
		p.Normalize()
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s *Store) GetProfessor(ctx context.Context, slug string) (Professor, error) {
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT data FROM professors WHERE slug=?`, slug).Scan(&raw); err != nil {
		return Professor{}, err
	}
	var p Professor
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return Professor{}, err
	}
	p.Normalize()
	return p, nil
}

func (s *Store) InsertProfessor(ctx context.Context, p Professor) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO professors(slug,data,updated_at) VALUES(?,?,?)`, p.Slug, string(raw), time.Now().Unix())
	if err == nil {
		s.cache.bump(ctx, "catalog")
	}
	return err
}

func (s *Store) UpdateProfessor(ctx context.Context, p Professor) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE professors SET data=?, updated_at=? WHERE slug=?`, string(raw), time.Now().Unix(), p.Slug)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	s.cache.bump(ctx, "catalog")
	return nil
}
