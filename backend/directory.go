package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

const directoryPageSize = 20

type DirectoryPage struct {
	Items      []Professor `json:"items"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	TotalPages int         `json:"totalPages"`
}

var persianLetters = strings.NewReplacer("ي", "ی", "ك", "ک", "‌", "", "ـ", "")

func normalized(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(persianLetters.Replace(value)), " "))
}

func rankScore(rank string) int {
	switch normalized(rank) {
	case "استاد", "استاد تمام":
		return 4
	case "دانشیار":
		return 3
	case "استادیار":
		return 2
	case "مربی":
		return 1
	default:
		return 0
	}
}

func (s *Store) SearchProfessors(ctx context.Context, query, faculty string, page int) (DirectoryPage, error) {
	result := DirectoryPage{Items: []Professor{}, PageSize: directoryPageSize}
	rows, err := s.db.QueryContext(ctx, `SELECT p.data, COALESCE(st.views,0), COALESCE(st.searches,0) FROM professors p LEFT JOIN professor_stats st ON st.slug=p.slug`)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	needle := normalized(query)
	faculty = normalized(faculty)
	all := []Professor{}
	for rows.Next() {
		var raw string
		var professor Professor
		if err := rows.Scan(&raw, &professor.Views, &professor.Searches); err != nil {
			return result, err
		}
		views, searches := professor.Views, professor.Searches
		if err := json.Unmarshal([]byte(raw), &professor); err != nil {
			return result, err
		}
		professor.Views, professor.Searches = views, searches
		professor.Normalize()
		if faculty != "" && normalized(professor.Faculty) != faculty {
			continue
		}
		if needle != "" && !strings.Contains(normalized(strings.Join([]string{professor.Name, professor.Department, professor.Field, professor.Faculty, professor.Rank}, " ")), needle) {
			continue
		}
		all = append(all, professor)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	nameOrder := collate.New(language.Make("fa"), collate.IgnoreCase)
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		popA, popB := a.Views+a.Searches, b.Views+b.Searches
		if popA != popB {
			return popA > popB
		}
		if a.Views != b.Views {
			return a.Views > b.Views
		}
		if rankScore(a.Rank) != rankScore(b.Rank) {
			return rankScore(a.Rank) > rankScore(b.Rank)
		}
		if byName := nameOrder.CompareString(a.Name, b.Name); byName != 0 {
			return byName < 0
		}
		return a.Slug < b.Slug
	})
	result.Total = len(all)
	result.TotalPages = (len(all) + directoryPageSize - 1) / directoryPageSize
	if page < 1 {
		page = 1
	}
	if result.TotalPages > 0 && page > result.TotalPages {
		page = result.TotalPages
	}
	result.Page = page
	start := (page - 1) * directoryPageSize
	if start < len(all) {
		end := start + directoryPageSize
		if end > len(all) {
			end = len(all)
		}
		result.Items = all[start:end]
	}
	return result, nil
}

func (s *Store) ListFaculties(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM faculties`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		items = append(items, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	order := collate.New(language.Make("fa"), collate.IgnoreCase)
	sort.Slice(items, func(i, j int) bool { return order.CompareString(items[i], items[j]) < 0 })
	return items, nil
}

func (s *Store) FacultyExists(ctx context.Context, name string) (bool, error) {
	items, err := s.ListFaculties(ctx)
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if normalized(item) == normalized(name) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) AddFaculty(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 3 || len([]rune(name)) > 120 {
		return errors.New("نام دانشکده باید بین ۳ تا ۱۲۰ نویسه باشد")
	}
	exists, err := s.FacultyExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("این دانشکده قبلاً ثبت شده است")
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO faculties(name,created_at) VALUES(?,?)`, name, time.Now().Unix())
	if err == nil {
		s.cache.bump(ctx, "catalog")
	}
	return err
}

// Count at most one view and one precise search per visitor, professor and day.
func (s *Store) RecordEngagement(ctx context.Context, visitorToken, slug, kind string) (bool, error) {
	if kind != "view" && kind != "search" {
		return false, errors.New("invalid engagement kind")
	}
	if _, err := s.GetProfessor(ctx, slug); err != nil {
		return false, err
	}
	hash := sha256.Sum256([]byte(visitorToken))
	day := time.Now().UTC().Format("2006-01-02")
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO engagement_events(visitor_hash,slug,kind,day) VALUES(?,?,?,?)`, hex.EncodeToString(hash[:]), slug, kind, day)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	if n > 0 {
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO professor_stats(slug) VALUES(?)`, slug); err != nil {
			return false, err
		}
		column := "views"
		if kind == "search" {
			column = "searches"
		}
		if _, err = tx.ExecContext(ctx, `UPDATE professor_stats SET `+column+`=`+column+`+1 WHERE slug=?`, slug); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if n > 0 {
		s.cache.bump(ctx, "ranking")
	}
	return n > 0, nil
}
