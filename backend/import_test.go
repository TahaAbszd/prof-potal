package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestOfficialImportPreservesExistingAndIsIdempotent(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "portal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	existing := Professor{Slug: "existing-professor", Name: "استاد صفر", Rank: "استادیار", Faculty: "دانشکده علوم پایه", Office: "اتاق فعلی"}
	existing.Normalize()
	if err := store.InsertProfessor(t.Context(), existing); err != nil {
		t.Fatal(err)
	}
	export := OfficialExport{Source: "https://ostad.hormozgan.ac.ir/ostad/", IndexCount: 100, ParsedCount: 100}
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("%04d", i+1000)
		name := fmt.Sprintf("استاد %d", i)
		if i == 0 {
			name = existing.Name
		}
		profile := Professor{Slug: "official-" + id, Name: name, Rank: "دانشیار", Faculty: "علوم پایه"}
		profile.Normalize()
		export.Records = append(export.Records, OfficialRecord{SourceID: id, SourceURL: export.Source + "resualtfni.aspx?m=" + id, Profile: profile})
	}
	raw, err := json.Marshal(export)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "official.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	report, err := store.ImportOfficialExport(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if report.Created != 99 || report.MatchedExisting != 1 || report.FacultiesAdded != 1 {
		t.Fatalf("unexpected import report: %+v", report)
	}
	current, err := store.GetProfessor(t.Context(), existing.Slug)
	if err != nil || current.Office != "اتاق فعلی" || current.Rank != "استادیار" {
		t.Fatalf("existing profile changed: %+v %v", current, err)
	}
	page, err := store.SearchProfessors(t.Context(), "", "", 1)
	if err != nil || page.Total != 100 {
		t.Fatalf("import count: %+v %v", page, err)
	}
	again, err := store.ImportOfficialExport(t.Context(), path)
	if err != nil || again.AlreadyImported != 100 || again.Created != 0 {
		t.Fatalf("idempotence: %+v %v", again, err)
	}
}

func TestSyncExistingOfficialProfilesKeepsAccountsAndStatistics(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "portal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := t.Context()
	existing := Professor{Slug: "local-professor", Name: "استاد نمونه", Rank: "استادیار", Faculty: "دانشکده قدیمی", Office: "اتاق قدیمی"}
	existing.Normalize()
	if err := store.InsertProfessor(ctx, existing); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateUser(ctx, "sample", "long-password-123", "professor", existing.Slug); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordEngagement(ctx, "visitor", existing.Slug, "view"); err != nil {
		t.Fatal(err)
	}
	export := OfficialExport{Source: "https://ostad.hormozgan.ac.ir/ostad/", IndexCount: 100, ParsedCount: 100}
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("%04d", i+1000)
		profile := Professor{Slug: "official-" + id, Name: fmt.Sprintf("استاد %d", i), Rank: "استاد", Faculty: "علوم پایه"}
		if i == 0 {
			profile.Name = existing.Name
			profile.Rank = "دانشیار"
			profile.Office = "اتاق رسمی"
		}
		export.Records = append(export.Records, OfficialRecord{SourceID: id, SourceURL: export.Source + "resualtfni.aspx?m=" + id, Profile: profile})
	}
	raw, err := json.Marshal(export)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "official.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportOfficialExport(ctx, path); err != nil {
		t.Fatal(err)
	}
	// Simulate a rank stored before the title was renamed.
	legacy, _ := json.Marshal(Professor{Slug: "official-1001", Name: "استاد 1", Rank: "استاد", Faculty: "دانشکده علوم پایه"})
	if _, err := store.db.ExecContext(ctx, `UPDATE professors SET data=? WHERE slug=?`, string(legacy), "official-1001"); err != nil {
		t.Fatal(err)
	}
	report, err := store.SyncExistingOfficialProfiles(ctx, path)
	if err != nil || report.RefreshedExisting != 1 || report.RanksNormalized != 1 {
		t.Fatalf("unexpected sync report: %+v %v", report, err)
	}
	updated, err := store.GetProfessor(ctx, existing.Slug)
	if err != nil || updated.Office != "اتاق رسمی" || updated.Rank != "دانشیار" || updated.Slug != existing.Slug {
		t.Fatalf("existing profile not synced: %+v %v", updated, err)
	}
	var views int
	if err := store.db.QueryRowContext(ctx, `SELECT views FROM professor_stats WHERE slug=?`, existing.Slug).Scan(&views); err != nil || views != 1 {
		t.Fatalf("views lost: %d %v", views, err)
	}
	var accountSlug string
	if err := store.db.QueryRowContext(ctx, `SELECT professor_slug FROM users WHERE username='sample'`).Scan(&accountSlug); err != nil || accountSlug != existing.Slug {
		t.Fatalf("account lost: %s %v", accountSlug, err)
	}
	var stored string
	if err := store.db.QueryRowContext(ctx, `SELECT json_extract(data,'$.rank') FROM professors WHERE slug='official-1001'`).Scan(&stored); err != nil || stored != "استاد تمام" {
		t.Fatalf("rank not saved: %s %v", stored, err)
	}
	err = store.db.QueryRowContext(ctx, `SELECT slug FROM professors WHERE slug='official-1000'`).Scan(&stored)
	if err != sql.ErrNoRows {
		t.Fatalf("matched profile was duplicated: %v", err)
	}
}
