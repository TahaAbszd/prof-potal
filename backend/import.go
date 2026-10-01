package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

const officialSource = "ostad.hormozgan.ac.ir"

var officialID = regexp.MustCompile(`^[0-9]{3,12}$`)

func canonicalOfficialFaculty(name string) string {
	name = strings.TrimSpace(name)
	switch name {
	case "علوم پایه", "فنی و مهندسی", "علوم و فنون دریایی", "کشاورزی و منابع طبیعی", "علوم انسانی", "مدیریت، اقتصاد و حسابداری", "مهندسی شیمی و نفت":
		return "دانشکده " + name
	default:
		return name
	}
}

type OfficialRecord struct {
	SourceID    string    `json:"sourceId"`
	SourceURL   string    `json:"sourceUrl"`
	ImageSource string    `json:"imageSource"`
	Profile     Professor `json:"profile"`
}

type OfficialExport struct {
	Source      string           `json:"source"`
	IndexCount  int              `json:"indexCount"`
	ParsedCount int              `json:"parsedCount"`
	Records     []OfficialRecord `json:"records"`
}

type ImportReport struct {
	SourceCount     int `json:"sourceCount"`
	Created         int `json:"created"`
	MatchedExisting int `json:"matchedExisting"`
	AlreadyImported int `json:"alreadyImported"`
	FacultiesAdded  int `json:"facultiesAdded"`
}

type SyncReport struct {
	RefreshedExisting int `json:"refreshedExisting"`
	RanksNormalized   int `json:"ranksNormalized"`
}

func readOfficialExport(path string) (OfficialExport, error) {
	var export OfficialExport
	raw, err := os.ReadFile(path)
	if err != nil {
		return export, err
	}
	if err := json.Unmarshal(raw, &export); err != nil {
		return export, err
	}
	if export.Source != "https://"+officialSource+"/ostad/" || export.IndexCount < 100 || export.IndexCount != export.ParsedCount || export.ParsedCount != len(export.Records) {
		return export, errors.New("official export is incomplete or has an unexpected source")
	}
	seen := map[string]bool{}
	for index := range export.Records {
		record := &export.Records[index]
		if !officialID.MatchString(record.SourceID) || seen[record.SourceID] || record.SourceURL != export.Source+"resualtfni.aspx?m="+record.SourceID {
			return export, fmt.Errorf("invalid or duplicate source ID: %q", record.SourceID)
		}
		seen[record.SourceID] = true
		record.Profile.Normalize()
		record.Profile.Faculty = canonicalOfficialFaculty(record.Profile.Faculty)
		record.Profile.Rank = strings.TrimSpace(persianLetters.Replace(record.Profile.Rank))
		record.Profile.Views, record.Profile.Searches = 0, 0
		if record.Profile.Slug != "official-"+record.SourceID {
			return export, fmt.Errorf("unexpected profile slug for %s", record.SourceID)
		}
		if err := record.Profile.Validate(); err != nil {
			return export, fmt.Errorf("profile %s: %w", record.SourceID, err)
		}
	}
	return export, nil
}

// ImportOfficialExport is idempotent and preserves existing profile edits and login accounts.
func (s *Store) ImportOfficialExport(ctx context.Context, path string) (ImportReport, error) {
	report := ImportReport{}
	export, err := readOfficialExport(path)
	if err != nil {
		return report, err
	}
	report.SourceCount = export.IndexCount

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT slug,data FROM professors`)
	if err != nil {
		return report, err
	}
	names := map[string][]string{}
	slugs := map[string]bool{}
	for rows.Next() {
		var slug, data string
		if err := rows.Scan(&slug, &data); err != nil {
			rows.Close()
			return report, err
		}
		var professor Professor
		if err := json.Unmarshal([]byte(data), &professor); err != nil {
			rows.Close()
			return report, err
		}
		key := normalized(professor.Name)
		names[key] = append(names[key], slug)
		slugs[slug] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return report, err
	}
	rows.Close()
	now := time.Now().Unix()
	for _, record := range export.Records {
		var mapped string
		err := tx.QueryRowContext(ctx, `SELECT slug FROM source_imports WHERE source=? AND source_id=?`, officialSource, record.SourceID).Scan(&mapped)
		if err == nil {
			report.AlreadyImported++
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return report, err
		}
		matches := names[normalized(record.Profile.Name)]
		if len(matches) > 1 {
			return report, fmt.Errorf("ambiguous existing profile for %s", record.Profile.Name)
		}
		slug := record.Profile.Slug
		if len(matches) == 1 {
			slug = matches[0]
			report.MatchedExisting++
		} else {
			if slugs[slug] {
				return report, fmt.Errorf("slug already belongs to a different professor: %s", slug)
			}
			data, err := json.Marshal(record.Profile)
			if err != nil {
				return report, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO professors(slug,data,updated_at) VALUES(?,?,?)`, slug, string(data), now); err != nil {
				return report, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO professor_stats(slug) VALUES(?)`, slug); err != nil {
				return report, err
			}
			names[normalized(record.Profile.Name)] = []string{slug}
			slugs[slug] = true
			report.Created++
		}
		result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO faculties(name,created_at) VALUES(?,?)`, record.Profile.Faculty, now)
		if err != nil {
			return report, err
		}
		if n, _ := result.RowsAffected(); n > 0 {
			report.FacultiesAdded++
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO source_imports(source,source_id,slug,imported_at) VALUES(?,?,?,?)`, officialSource, record.SourceID, slug, now); err != nil {
			return report, err
		}
	}
	if err := tx.Commit(); err != nil {
		return report, err
	}
	if report.Created > 0 || report.MatchedExisting > 0 || report.FacultiesAdded > 0 {
		s.cache.bump(ctx, "catalog")
		if s.cache != nil && s.cache.disabled.Load() {
			return report, errors.New("profiles imported, but Redis invalidation failed; restart the API or wait for cache expiration")
		}
	}
	return report, nil
}

// SyncExistingOfficialProfiles refreshes profiles matched to an official ID while
// preserving their local URLs, accounts and engagement statistics.
func (s *Store) SyncExistingOfficialProfiles(ctx context.Context, path string) (SyncReport, error) {
	report := SyncReport{}
	export, err := readOfficialExport(path)
	if err != nil {
		return report, err
	}
	byID := make(map[string]Professor, len(export.Records))
	for _, record := range export.Records {
		byID[record.SourceID] = record.Profile
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT source_id,slug FROM source_imports WHERE source=?`, officialSource)
	if err != nil {
		return report, err
	}
	matched := map[string]Professor{}
	for rows.Next() {
		var id, slug string
		if err := rows.Scan(&id, &slug); err != nil {
			rows.Close()
			return report, err
		}
		if !strings.HasPrefix(slug, "official-") {
			profile, ok := byID[id]
			if !ok {
				rows.Close()
				return report, fmt.Errorf("official source ID %s is missing from export", id)
			}
			matched[slug] = profile
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return report, err
	}
	rows.Close()
	profiles, err := tx.QueryContext(ctx, `SELECT slug,data FROM professors`)
	if err != nil {
		return report, err
	}
	type update struct {
		slug string
		data string
	}
	updates := []update{}
	for profiles.Next() {
		var slug, raw string
		if err := profiles.Scan(&slug, &raw); err != nil {
			profiles.Close()
			return report, err
		}
		var current Professor
		if err := json.Unmarshal([]byte(raw), &current); err != nil {
			profiles.Close()
			return report, err
		}
		originalRank := current.Rank
		_, isMatched := matched[slug]
		if official, ok := matched[slug]; ok {
			if normalized(current.Name) != normalized(official.Name) {
				profiles.Close()
				return report, fmt.Errorf("official name does not match local profile %s", slug)
			}
			current = official
			current.Slug = slug
			report.RefreshedExisting++
		}
		current.Normalize()
		if originalRank == "استاد" {
			report.RanksNormalized++
		}
		if current.Rank != originalRank || isMatched {
			if err := current.Validate(); err != nil {
				profiles.Close()
				return report, fmt.Errorf("profile %s: %w", slug, err)
			}
			encoded, err := json.Marshal(current)
			if err != nil {
				profiles.Close()
				return report, err
			}
			updates = append(updates, update{slug: slug, data: string(encoded)})
		}
	}
	if err := profiles.Err(); err != nil {
		profiles.Close()
		return report, err
	}
	profiles.Close()
	now := time.Now().Unix()
	for _, item := range updates {
		if _, err := tx.ExecContext(ctx, `UPDATE professors SET data=?, updated_at=? WHERE slug=?`, item.data, now, item.slug); err != nil {
			return report, err
		}
	}
	if err := tx.Commit(); err != nil {
		return report, err
	}
	if len(updates) > 0 {
		s.cache.bump(ctx, "catalog")
		if s.cache != nil && s.cache.disabled.Load() {
			return report, errors.New("profiles updated, but Redis invalidation failed; restart the API or wait for cache expiration")
		}
	}
	return report, nil
}
