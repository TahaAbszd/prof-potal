package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestRoleAccessAndProfileUpdate(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "portal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Seed("../data/professors.json", "admin", "admin-test-password-123", "faculty-test-password-123"); err != nil {
		t.Fatal(err)
	}
	router := Router(store, Config{PublicOrigin: "http://localhost:3000"})
	request := func(method, path string, body any, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
		var payload []byte
		if body != nil {
			payload, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(payload))
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	login := func(username, password, role string) *http.Cookie {
		response := request(http.MethodPost, "/api/auth/login", map[string]string{"username": username, "password": password, "role": role}, nil, "http://localhost:3000")
		if response.Code != http.StatusOK {
			t.Fatalf("login %s: %d %s", username, response.Code, response.Body.String())
		}
		cookies := response.Result().Cookies()
		if len(cookies) != 1 || !cookies[0].HttpOnly {
			t.Fatal("secure session cookie missing")
		}
		return cookies[0]
	}
	if got := request(http.MethodGet, "/api/auth/me", nil, nil, "").Code; got != http.StatusUnauthorized {
		t.Fatalf("unauthenticated me: %d", got)
	}
	profCookie := login("golzari", "faculty-test-password-123", "professor")
	if got := request(http.MethodPost, "/api/admin/faculties", map[string]string{"name": "دانشکده جدید"}, profCookie, "http://localhost:3000").Code; got != http.StatusForbidden {
		t.Fatalf("professor faculty admin access: %d", got)
	}
	if got := request(http.MethodPost, "/api/admin/professors/shahram-golzari-hormozi/account", map[string]string{"username": "forbidden", "password": "forbidden-password-123"}, profCookie, "http://localhost:3000").Code; got != http.StatusForbidden {
		t.Fatalf("professor account admin access: %d", got)
	}
	if got := request(http.MethodPost, "/api/admin/professors", map[string]string{}, profCookie, "http://localhost:3000").Code; got != http.StatusForbidden {
		t.Fatalf("professor admin access: %d", got)
	}
	other, err := store.GetProfessor(t.Context(), "seyed-hassan-daryanavard")
	if err != nil {
		t.Fatal(err)
	}
	other.Office = "نباید ذخیره شود"
	if got := request(http.MethodPut, "/api/professors/seyed-hassan-daryanavard", other, profCookie, "http://localhost:3000").Code; got != http.StatusForbidden {
		t.Fatalf("cross-profile update: %d", got)
	}
	self, err := store.GetProfessor(t.Context(), "shahram-golzari-hormozi")
	if err != nil {
		t.Fatal(err)
	}
	self.Office = "اتاق ۱۲"
	if got := request(http.MethodPut, "/api/professors/shahram-golzari-hormozi", self, profCookie, "http://evil.example").Code; got != http.StatusForbidden {
		t.Fatalf("cross-origin update: %d", got)
	}
	if got := request(http.MethodPut, "/api/professors/shahram-golzari-hormozi", self, profCookie, "http://localhost:3000").Code; got != http.StatusOK {
		t.Fatalf("own profile update: %d", got)
	}
	saved, err := store.GetProfessor(t.Context(), self.Slug)
	if err != nil || saved.Office != "اتاق ۱۲" {
		t.Fatal("profile update not persisted")
	}
	adminCookie := login("admin", "admin-test-password-123", "admin")
	imported := Professor{Slug: "imported-professor", Name: "استاد واردشده", Rank: "استادیار", Faculty: self.Faculty}
	imported.Normalize()
	if err := store.InsertProfessor(t.Context(), imported); err != nil {
		t.Fatal(err)
	}
	if got := request(http.MethodGet, "/api/admin/professors/imported-professor/account", nil, adminCookie, "").Code; got != http.StatusOK {
		t.Fatalf("account status: %d", got)
	}
	if got := request(http.MethodPost, "/api/admin/professors/imported-professor/account", map[string]string{"username": "imported", "password": "imported-password-123"}, adminCookie, "http://localhost:3000").Code; got != http.StatusCreated {
		t.Fatalf("create imported account: %d", got)
	}
	login("imported", "imported-password-123", "professor")
	if got := request(http.MethodPost, "/api/admin/faculties", map[string]string{"name": "علوم پایه"}, adminCookie, "http://localhost:3000").Code; got != http.StatusCreated {
		t.Fatalf("admin add faculty: %d", got)
	}
	if got := request(http.MethodPut, "/api/professors/seyed-hassan-daryanavard", other, adminCookie, "http://localhost:3000").Code; got != http.StatusOK {
		t.Fatalf("admin update: %d", got)
	}
	newProfile := Professor{Slug: "new-professor", Name: "استاد جدید", Rank: "استادیار", Faculty: "علوم پایه"}
	newProfile.Normalize()
	createBody := map[string]any{"profile": newProfile, "username": "newprof", "password": "new-professor-password-123"}
	if got := request(http.MethodPost, "/api/admin/professors", createBody, adminCookie, "http://localhost:3000").Code; got != http.StatusCreated {
		t.Fatalf("admin create: %d", got)
	}
	newCookie := login("newprof", "new-professor-password-123", "professor")
	if got := request(http.MethodGet, "/api/auth/me", nil, newCookie, "").Code; got != http.StatusOK {
		t.Fatalf("new professor session: %d", got)
	}
	change := map[string]string{"currentPassword": "new-professor-password-123", "newPassword": "replacement-password-123"}
	if got := request(http.MethodPost, "/api/auth/change-password", change, newCookie, "http://localhost:3000").Code; got != http.StatusNoContent {
		t.Fatalf("change password: %d", got)
	}
	if got := request(http.MethodGet, "/api/auth/me", nil, newCookie, "").Code; got != http.StatusUnauthorized {
		t.Fatalf("old session still active: %d", got)
	}
	login("newprof", "replacement-password-123", "professor")
}

func TestPublicDirectoryAndViewDeduplication(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "portal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Seed("../data/professors.json", "admin", "admin-test-password-123", "faculty-test-password-123"); err != nil {
		t.Fatal(err)
	}
	router := Router(store, Config{PublicOrigin: "http://localhost:3000"})
	request := func(cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/professors/shahram-golzari-hormozi/view", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	first := request(nil)
	if first.Code != http.StatusOK || len(first.Result().Cookies()) != 1 {
		t.Fatalf("first view: %d %s", first.Code, first.Body.String())
	}
	second := request(first.Result().Cookies()[0])
	if second.Code != http.StatusOK || second.Body.String() != `{"recorded":false}` {
		t.Fatalf("duplicate view: %d %s", second.Code, second.Body.String())
	}
	page, err := store.SearchProfessors(t.Context(), "شهرام", "", 1)
	if err != nil || page.Total != 1 || page.Items[0].Views != 1 {
		t.Fatalf("directory statistics: %+v %v", page, err)
	}
}

func TestProfessorValidation(t *testing.T) {
	p := Professor{Slug: "valid-slug", Name: "استاد", Rank: "استادیار", Faculty: "فنی", Downloads: []Download{{Title: "فایل", URL: "javascript:alert(1)"}}}
	if err := p.Validate(); err == nil {
		t.Fatal("unsafe download URL accepted")
	}
	p.Downloads = nil
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
}
