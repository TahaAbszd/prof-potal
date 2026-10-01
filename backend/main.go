package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type Config struct {
	Port, DatabasePath, SeedDataPath, FontPath, ChromeBin, PublicOrigin, RedisURL, RedisPrefix string
	SecureCookies                                                                              bool
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func readConfig() Config {
	_ = godotenv.Load(".env")
	origin := envOr("PUBLIC_ORIGIN", "http://localhost:3000")
	return Config{
		Port:          envOr("PORT", "8080"),
		DatabasePath:  envOr("DATABASE_PATH", "./data/portal.db"),
		SeedDataPath:  envOr("SEED_DATA_PATH", "../data/professors.json"),
		FontPath:      envOr("RESUME_FONT_PATH", "../public/fonts/IRANSansWeb.woff2"),
		ChromeBin:     envOr("CHROME_BIN", ""),
		PublicOrigin:  strings.TrimRight(origin, "/"),
		RedisURL:      envOr("REDIS_URL", "redis://127.0.0.1:6379/0"),
		RedisPrefix:   envOr("REDIS_PREFIX", "hormozgan:portal"),
		SecureCookies: strings.HasPrefix(origin, "https://") || os.Getenv("COOKIE_SECURE") == "true",
	}
}

func decodeJSON(c *gin.Context, destination any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("body must contain one JSON object")
	}
	return nil
}

func sameOrigin(cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin != "" && origin != cfg.PublicOrigin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "درخواست از مبدأ نامعتبر است"})
			return
		}
		c.Next()
	}
}

func setSessionCookie(c *gin.Context, cfg Config, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(cookieName, token, maxAge, "/", "", cfg.SecureCookies, true)
}

func visitorToken(c *gin.Context, cfg Config) (string, error) {
	if token, err := c.Cookie("hormozgan_visitor"); err == nil && len(token) == 32 {
		if _, err := hex.DecodeString(token); err == nil {
			return token, nil
		}
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("hormozgan_visitor", token, 365*24*3600, "/", "", cfg.SecureCookies, true)
	return token, nil
}

func Router(store *Store, cfg Config) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), sameOrigin(cfg))
	if err := router.SetTrustedProxies(nil); err != nil {
		panic(err)
	}
	router.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	})
	limit := &limiter{Entries: map[string]attempt{}}
	api := router.Group("/api")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "cache": store.cache.Status(c.Request.Context())})
	})
	api.POST("/auth/login", func(c *gin.Context) {
		key := c.ClientIP()
		if !limit.Allowed(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "تلاش‌های ناموفق زیاد بوده است. کمی بعد دوباره امتحان کنید"})
			return
		}
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := decodeJSON(c, &input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "درخواست نامعتبر است"})
			return
		}
		user, err := store.Authenticate(c.Request.Context(), input.Username, input.Password)
		if err != nil {
			limit.Failed(key)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "نام کاربری یا گذرواژه نادرست است"})
			return
		}
		if input.Role != "" && input.Role != user.Role {
			limit.Failed(key)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "این حساب برای نقش انتخاب‌شده تعریف نشده است"})
			return
		}
		limit.Reset(key)
		token, err := store.NewSession(c.Request.Context(), user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در ایجاد نشست"})
			return
		}
		setSessionCookie(c, cfg, token, int(sessionLifetime.Seconds()))
		c.JSON(http.StatusOK, user)
	})
	api.POST("/auth/logout", func(c *gin.Context) {
		token, _ := c.Cookie(cookieName)
		store.DeleteSession(c.Request.Context(), token)
		setSessionCookie(c, cfg, "", -1)
		c.Status(http.StatusNoContent)
	})
	auth := api.Group("", store.RequireUser())
	auth.GET("/auth/me", func(c *gin.Context) { c.JSON(http.StatusOK, c.MustGet("user")) })
	auth.POST("/auth/change-password", func(c *gin.Context) {
		var input struct {
			CurrentPassword string `json:"currentPassword"`
			NewPassword     string `json:"newPassword"`
		}
		if err := decodeJSON(c, &input); err != nil || !validPassword(input.NewPassword) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "گذرواژه جدید باید ۱۲ تا ۷۲ نویسه باشد"})
			return
		}
		user := c.MustGet("user").(User)
		if err := store.ChangeOwnPassword(c.Request.Context(), user.ID, input.CurrentPassword, input.NewPassword); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "گذرواژه فعلی نادرست است"})
			return
		}
		setSessionCookie(c, cfg, "", -1)
		c.Status(http.StatusNoContent)
	})

	api.GET("/professors", func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		items, err := store.CachedSearchProfessors(c.Request.Context(), c.Query("q"), c.Query("faculty"), page)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در دریافت اساتید"})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, items)
	})
	api.GET("/faculties", func(c *gin.Context) {
		items, err := store.CachedListFaculties(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در دریافت دانشکده‌ها"})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, items)
	})
	api.POST("/professors/:slug/view", func(c *gin.Context) {
		token, err := visitorToken(c, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در ثبت بازدید"})
			return
		}
		recorded, err := store.RecordEngagement(c.Request.Context(), token, c.Param("slug"), "view")
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "استاد یافت نشد"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در ثبت بازدید"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"recorded": recorded})
	})
	api.POST("/search-events", func(c *gin.Context) {
		var input struct {
			Query string `json:"query"`
		}
		if err := decodeJSON(c, &input); err != nil || len([]rune(normalized(input.Query))) < 2 || len([]rune(input.Query)) > 160 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "عبارت جست‌وجو نامعتبر است"})
			return
		}
		matches, err := store.SearchProfessors(c.Request.Context(), input.Query, "", 1)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در جست‌وجو"})
			return
		}
		if matches.Total != 1 {
			c.JSON(http.StatusOK, gin.H{"recorded": false})
			return
		}
		token, err := visitorToken(c, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در ثبت جست‌وجو"})
			return
		}
		recorded, err := store.RecordEngagement(c.Request.Context(), token, matches.Items[0].Slug, "search")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در ثبت جست‌وجو"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"recorded": recorded})
	})
	api.GET("/professors/:slug", func(c *gin.Context) {
		item, err := store.CachedGetProfessor(c.Request.Context(), c.Param("slug"))
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "استاد یافت نشد"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در دریافت اطلاعات استاد"})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, item)
	})
	auth.PUT("/professors/:slug", func(c *gin.Context) {
		user := c.MustGet("user").(User)
		slug := c.Param("slug")
		if user.Role != "admin" && user.ProfessorSlug != slug {
			c.JSON(http.StatusForbidden, gin.H{"error": "فقط پروفایل خود را می‌توانید ویرایش کنید"})
			return
		}
		var item Professor
		if err := decodeJSON(c, &item); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "دادهٔ پروفایل نامعتبر است"})
			return
		}
		item.Normalize()
		item.Views, item.Searches = 0, 0
		if item.Slug != slug {
			c.JSON(http.StatusBadRequest, gin.H{"error": "شناسهٔ استاد قابل تغییر نیست"})
			return
		}
		if err := item.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		exists, err := store.FacultyExists(c.Request.Context(), item.Faculty)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در بررسی دانشکده"})
			return
		}
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "دانشکده انتخاب‌شده ثبت نشده است"})
			return
		}
		if err := store.UpdateProfessor(c.Request.Context(), item); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "استاد یافت نشد"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در ذخیرهٔ اطلاعات"})
			return
		}
		c.JSON(http.StatusOK, item)
	})
	auth.POST("/admin/professors", func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		var input struct {
			Profile  Professor `json:"profile"`
			Username string    `json:"username"`
			Password string    `json:"password"`
		}
		if err := decodeJSON(c, &input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "دادهٔ ارسالی نامعتبر است"})
			return
		}
		input.Profile.Normalize()
		input.Profile.Views, input.Profile.Searches = 0, 0
		if err := input.Profile.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		exists, err := store.FacultyExists(c.Request.Context(), input.Profile.Faculty)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در بررسی دانشکده"})
			return
		}
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "دانشکده انتخاب‌شده ثبت نشده است"})
			return
		}
		if err := store.CreateProfessorAccount(c.Request.Context(), input.Profile, input.Username, input.Password); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "شناسه یا نام کاربری تکراری یا نامعتبر است"})
			return
		}
		c.JSON(http.StatusCreated, input.Profile)
	})
	auth.POST("/admin/faculties", func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		var input struct {
			Name string `json:"name"`
		}
		if err := decodeJSON(c, &input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "درخواست نامعتبر است"})
			return
		}
		if err := store.AddFaculty(c.Request.Context(), input.Name); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"name": strings.TrimSpace(input.Name)})
	})
	auth.POST("/admin/professors/:slug/password", func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		var input struct {
			Password string `json:"password"`
		}
		if err := decodeJSON(c, &input); err != nil || !validPassword(input.Password) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "گذرواژه باید ۱۲ تا ۷۲ نویسه باشد"})
			return
		}
		if err := store.SetProfessorPassword(c.Request.Context(), c.Param("slug"), input.Password); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "حساب استاد یافت نشد"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در تغییر گذرواژه"})
			return
		}
		c.Status(http.StatusNoContent)
	})
	auth.GET("/admin/professors/:slug/account", func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		var username string
		err := store.db.QueryRowContext(c.Request.Context(), `SELECT username FROM users WHERE professor_slug=?`, c.Param("slug")).Scan(&username)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusOK, gin.H{"exists": false})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در بررسی حساب استاد"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"exists": true, "username": username})
	})
	auth.POST("/admin/professors/:slug/account", func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := decodeJSON(c, &input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "درخواست نامعتبر است"})
			return
		}
		if _, err := store.GetProfessor(c.Request.Context(), c.Param("slug")); errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "استاد یافت نشد"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در دریافت استاد"})
			return
		}
		if err := store.CreateUser(c.Request.Context(), input.Username, input.Password, "professor", c.Param("slug")); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "نام کاربری یا گذرواژه نامعتبر یا تکراری است"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"username": strings.TrimSpace(input.Username)})
	})

	resumeSlots := make(chan struct{}, 2)
	api.GET("/professors/:slug/resume.pdf", func(c *gin.Context) {
		item, err := store.CachedGetProfessor(c.Request.Context(), c.Param("slug"))
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "استاد یافت نشد"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در دریافت اطلاعات استاد"})
			return
		}
		select {
		case resumeSlots <- struct{}{}:
			defer func() { <-resumeSlots }()
		default:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "سامانهٔ خروجی مشغول است؛ کمی بعد دوباره تلاش کنید"})
			return
		}
		pdf, err := RenderResumePDF(c.Request.Context(), item, cfg)
		if err != nil {
			log.Printf("PDF render failed for %s: %v", item.Slug, err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ساخت رزومه ممکن نشد"})
			return
		}
		c.Header("Content-Disposition", `attachment; filename="`+item.Slug+`-resume.pdf"`)
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "application/pdf", pdf)
	})
	return router
}

func main() {
	cfg := readConfig()
	if os.Getenv("GIN_MODE") == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}
	store, err := OpenStore(cfg.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if len(os.Args) > 1 {
		if len(os.Args) != 3 || (os.Args[1] != "import-official" && os.Args[1] != "sync-official-existing") {
			log.Fatal("usage: go run . <import-official|sync-official-existing> <official-faculty-profiles.json>")
		}
		cache, err := NewCache(cfg.RedisURL, cfg.RedisPrefix)
		if err != nil {
			log.Fatal(err)
		}
		defer cache.Close()
		if err := cache.Ping(context.Background()); err != nil {
			log.Fatalf("Redis must be available during bulk import: %v", err)
		}
		store.SetCache(cache)
		if os.Args[1] == "import-official" {
			report, err := store.ImportOfficialExport(context.Background(), os.Args[2])
			if err != nil {
				log.Fatalf("official import: %v (report: %+v)", err, report)
			}
			encoded, _ := json.Marshal(report)
			log.Printf("official import complete: %s", encoded)
		} else {
			report, err := store.SyncExistingOfficialProfiles(context.Background(), os.Args[2])
			if err != nil {
				log.Fatalf("official sync: %v (report: %+v)", err, report)
			}
			encoded, _ := json.Marshal(report)
			log.Printf("official sync complete: %s", encoded)
		}
		return
	}
	if err := store.Seed(cfg.SeedDataPath, os.Getenv("BOOTSTRAP_ADMIN_USERNAME"), os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"), os.Getenv("BOOTSTRAP_FACULTY_PASSWORD")); err != nil {
		log.Fatal(err)
	}
	cache, err := NewCache(cfg.RedisURL, cfg.RedisPrefix)
	if err != nil {
		log.Fatalf("invalid REDIS_URL: %v", err)
	}
	defer cache.Close()
	if err := cache.Ping(context.Background()); err != nil {
		log.Printf("Redis unavailable; serving from SQLite until it reconnects: %v", err)
	} else {
		log.Printf("Redis cache connected")
	}
	store.SetCache(cache)
	server := &http.Server{Addr: ":" + cfg.Port, Handler: Router(store, cfg), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("faculty API listening on :%s", cfg.Port)
	log.Fatal(server.ListenAndServe())
}
