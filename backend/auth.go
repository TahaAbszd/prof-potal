package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const cookieName = "hormozgan_session"
const sessionLifetime = 12 * time.Hour

type User struct {
	ID            int64  `json:"-"`
	Username      string `json:"username"`
	Role          string `json:"role"`
	ProfessorSlug string `json:"professorSlug,omitempty"`
}

func validPassword(password string) bool { return len(password) >= 12 && len(password) <= 72 }

func (s *Store) CreateUser(ctx context.Context, username, password, role, slug string) error {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 64 || !validPassword(password) {
		return errors.New("نام کاربری یا گذرواژه نامعتبر است؛ گذرواژه باید ۱۲ تا ۷۲ نویسه باشد")
	}
	if role != "admin" && role != "professor" {
		return errors.New("نقش نامعتبر است")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost+2)
	if err != nil {
		return err
	}
	var professorSlug any
	if role == "professor" {
		professorSlug = slug
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO users(username,password_hash,role,professor_slug) VALUES(?,?,?,?)`, username, string(hash), role, professorSlug)
	return err
}

func (s *Store) CreateProfessorAccount(ctx context.Context, p Professor, username, password string) error {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 64 || !validPassword(password) {
		return errors.New("نام کاربری یا گذرواژه نامعتبر است؛ گذرواژه باید ۱۲ تا ۷۲ نویسه باشد")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost+2)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO professors(slug,data,updated_at) VALUES(?,?,?)`, p.Slug, string(raw), time.Now().Unix()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO users(username,password_hash,role,professor_slug) VALUES(?,?,?,?)`, username, string(hash), "professor", p.Slug); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.cache.bump(ctx, "catalog")
	return nil
}

func (s *Store) Authenticate(ctx context.Context, username, password string) (User, error) {
	var user User
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT id,username,password_hash,role,COALESCE(professor_slug,'') FROM users WHERE username=?`, strings.TrimSpace(username)).Scan(&user.ID, &user.Username, &hash, &user.Role, &user.ProfessorSlug)
	if err != nil {
		return User{}, errors.New("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, errors.New("invalid credentials")
	}
	return user, nil
}

func (s *Store) NewSession(ctx context.Context, userID int64) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(token))
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES(?,?,?)`, hex.EncodeToString(sum[:]), userID, time.Now().Add(sessionLifetime).Unix())
	return token, err
}

func (s *Store) SessionUser(ctx context.Context, token string) (User, error) {
	if len(token) != 64 {
		return User{}, sql.ErrNoRows
	}
	sum := sha256.Sum256([]byte(token))
	var user User
	err := s.db.QueryRowContext(ctx, `SELECT u.id,u.username,u.role,COALESCE(u.professor_slug,'') FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>?`, hex.EncodeToString(sum[:]), time.Now().Unix()).Scan(&user.ID, &user.Username, &user.Role, &user.ProfessorSlug)
	return user, err
}

func (s *Store) DeleteSession(ctx context.Context, token string) {
	if len(token) != 64 {
		return
	}
	sum := sha256.Sum256([]byte(token))
	s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, hex.EncodeToString(sum[:]))
}

func (s *Store) SetProfessorPassword(ctx context.Context, slug, password string) error {
	if !validPassword(password) {
		return errors.New("invalid password")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost+2)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE users SET password_hash=? WHERE role='professor' AND professor_slug=?`, string(hash), slug)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=(SELECT id FROM users WHERE professor_slug=?)`, slug); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ChangeOwnPassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	if !validPassword(newPassword) {
		return errors.New("new password must be 12 to 72 characters")
	}
	var previousHash string
	if err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=?`, userID).Scan(&previousHash); err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(previousHash), []byte(oldPassword)) != nil {
		return errors.New("current password is incorrect")
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost+2)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE users SET password_hash=? WHERE id=?`, string(newHash), userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(cookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "ابتدا وارد حساب خود شوید"})
			return
		}
		user, err := s.SessionUser(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "نشست شما منقضی شده است"})
			return
		}
		c.Set("user", user)
		c.Next()
	}
}

func requireAdmin(c *gin.Context) bool {
	user := c.MustGet("user").(User)
	if user.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "این بخش فقط برای مدیر در دسترس است"})
		return false
	}
	return true
}

type attempt struct {
	Count int
	Until time.Time
}
type limiter struct {
	sync.Mutex
	Entries map[string]attempt
}

func (l *limiter) Allowed(key string) bool {
	l.Lock()
	defer l.Unlock()
	entry := l.Entries[key]
	if time.Now().After(entry.Until) {
		delete(l.Entries, key)
		return true
	}
	return entry.Count < 8
}

func (l *limiter) Failed(key string) {
	l.Lock()
	defer l.Unlock()
	entry := l.Entries[key]
	if time.Now().After(entry.Until) {
		entry = attempt{Until: time.Now().Add(15 * time.Minute)}
	}
	entry.Count++
	l.Entries[key] = entry
}

func (l *limiter) Reset(key string) { l.Lock(); delete(l.Entries, key); l.Unlock() }
