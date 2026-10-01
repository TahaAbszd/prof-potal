package main

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestDirectoryPagingOrderingAndEngagement(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "directory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.AddFaculty(ctx, "دانشکده آزمون"); err != nil {
		t.Fatal(err)
	}
	ranks := []string{"مربی", "استادیار", "دانشیار", "استاد"}
	for i := 0; i < 25; i++ {
		p := Professor{Slug: fmt.Sprintf("professor-%02d", i), Name: fmt.Sprintf("استاد %02d", i), Rank: ranks[i%4], Faculty: "دانشکده آزمون"}
		p.Normalize()
		if err := store.InsertProfessor(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.SearchProfessors(ctx, "", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 25 || first.TotalPages != 2 || len(first.Items) != 20 {
		t.Fatalf("unexpected first page: %+v", first)
	}
	if first.Items[0].Rank != "استاد تمام" {
		t.Fatalf("rank order failed: %+v", first.Items[0])
	}
	second, err := store.SearchProfessors(ctx, "", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 5 {
		t.Fatalf("second page length = %d", len(second.Items))
	}
	recorded, err := store.RecordEngagement(ctx, "visitor-1", "professor-00", "view")
	if err != nil || !recorded {
		t.Fatalf("first view: %v %v", recorded, err)
	}
	recorded, err = store.RecordEngagement(ctx, "visitor-1", "professor-00", "view")
	if err != nil || recorded {
		t.Fatalf("repeat view: %v %v", recorded, err)
	}
	recorded, err = store.RecordEngagement(ctx, "visitor-1", "professor-00", "search")
	if err != nil || !recorded {
		t.Fatalf("search: %v %v", recorded, err)
	}
	ranked, err := store.SearchProfessors(ctx, "", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if ranked.Items[0].Slug != "professor-00" || ranked.Items[0].Views != 1 || ranked.Items[0].Searches != 1 {
		t.Fatalf("popularity order failed: %+v", ranked.Items[0])
	}
	filtered, err := store.SearchProfessors(ctx, "اسTاد 00", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 0 {
		t.Fatalf("unexpected filter total: %d", filtered.Total)
	}
	filtered, err = store.SearchProfessors(ctx, "استاد 00", "دانشکده آزمون", 1)
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || filtered.Items[0].Slug != "professor-00" {
		t.Fatalf("search failed: %+v", filtered)
	}
}
