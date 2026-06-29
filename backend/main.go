package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type app struct {
	db *sql.DB
}

type user struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type authUser struct {
	user
}

type post struct {
	ID          int64      `json:"id"`
	AuthorID    int64      `json:"authorId"`
	AuthorName  string     `json:"authorName"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Summary     string     `json:"summary"`
	Content     string     `json:"content"`
	Category    string     `json:"category"`
	Tags        []string   `json:"tags"`
	CoverURL    string     `json:"coverUrl"`
	Status      string     `json:"status"`
	ReadMinutes int        `json:"readMinutes"`
	Views       int64      `json:"views"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
}

type postInput struct {
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Summary     string   `json:"summary"`
	Content     string   `json:"content"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	CoverURL    string   `json:"coverUrl"`
	Status      string   `json:"status"`
	ReadMinutes int      `json:"readMinutes"`
}

type comment struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email,omitempty"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type commentInput struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Content string `json:"content"`
}

type siteSettings struct {
	SiteName        string `json:"siteName"`
	SiteSubtitle    string `json:"siteSubtitle"`
	SiteURL         string `json:"siteUrl"`
	SEODescription  string `json:"seoDescription"`
	SEOKeywords     string `json:"seoKeywords"`
	DefaultOGImage  string `json:"defaultOgImage"`
	FooterText      string `json:"footerText"`
}

type registerInput struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	CaptchaID string `json:"captchaId"`
	Captcha   string `json:"captcha"`
}

type loginInput struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	CaptchaID string `json:"captchaId"`
	Captcha   string `json:"captcha"`
}

func main() {
	dsn := getenv("MYSQL_DSN", "blog:blog_password@tcp(localhost:3306)/tech_blog?parseTime=true&charset=utf8mb4&loc=Local")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(6)
	db.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := waitForDB(ctx, db); err != nil {
		log.Fatal(err)
	}
	if err := migrate(ctx, db); err != nil {
		log.Fatal(err)
	}
	if err := ensureAdminUser(ctx, db); err != nil {
		log.Fatal(err)
	}

	a := &app{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/posts", a.listPosts)
	mux.HandleFunc("GET /api/posts/{slug}", a.getPost)
	mux.HandleFunc("GET /api/comments", a.listComments)
	mux.HandleFunc("POST /api/comments", a.createComment)
	mux.HandleFunc("GET /api/settings", a.getSettings)
	mux.HandleFunc("GET /api/captcha", a.createCaptcha)
	mux.HandleFunc("POST /api/auth/register", a.register)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/logout", a.requireAuth(a.logout))
	mux.HandleFunc("GET /api/auth/me", a.requireAuth(a.me))
	mux.HandleFunc("GET /api/admin/posts", a.requireAuth(a.listManagePosts))
	mux.HandleFunc("POST /api/admin/posts", a.requireAuth(a.createPost))
	mux.HandleFunc("PUT /api/admin/posts/{id}", a.requireAuth(a.updatePost))
	mux.HandleFunc("DELETE /api/admin/posts/{id}", a.requireAuth(a.deletePost))
	mux.HandleFunc("GET /api/admin/comments", a.requireAdmin(a.listAdminComments))
	mux.HandleFunc("PUT /api/admin/comments/{id}", a.requireAdmin(a.updateComment))
	mux.HandleFunc("DELETE /api/admin/comments/{id}", a.requireAdmin(a.deleteComment))
	mux.HandleFunc("PUT /api/admin/settings", a.requireAdmin(a.updateSettings))
	mux.HandleFunc("GET /api/admin/stats", a.requireAdmin(a.adminStats))
	mux.HandleFunc("GET /api/admin/users", a.requireAdmin(a.listUsers))
	mux.HandleFunc("PUT /api/admin/users/{id}", a.requireAdmin(a.updateUser))
	mux.HandleFunc("DELETE /api/admin/users/{id}", a.requireAdmin(a.deleteUser))
	mux.HandleFunc("POST /api/uploads", a.requireAuth(a.uploadImage))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir()))))

	handler := withCORS(mux)
	addr := getenv("APP_ADDR", ":8080")
	log.Printf("blog api listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

func waitForDB(ctx context.Context, db *sql.DB) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastErr error
	for {
		if err := db.PingContext(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("%w: %v", ctx.Err(), lastErr)
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func migrate(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  username VARCHAR(60) NOT NULL UNIQUE,
  password_hash VARCHAR(220) NOT NULL,
  phone VARCHAR(30) NOT NULL,
  email VARCHAR(120) NOT NULL UNIQUE,
  role ENUM('user', 'admin') NOT NULL DEFAULT 'user',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS sessions (
  token CHAR(64) PRIMARY KEY,
  user_id BIGINT NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_sessions_user (user_id),
  INDEX idx_sessions_expires (expires_at),
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS captchas (
  id CHAR(32) PRIMARY KEY,
  code VARCHAR(8) NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  used TINYINT(1) NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_captchas_expires (expires_at)
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS posts (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  author_id BIGINT NULL,
  title VARCHAR(180) NOT NULL,
  slug VARCHAR(220) NOT NULL UNIQUE,
  summary VARCHAR(500) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  category VARCHAR(80) NOT NULL,
  tags JSON NOT NULL,
  cover_url VARCHAR(500) NOT NULL DEFAULT '',
  status ENUM('draft', 'published') NOT NULL DEFAULT 'draft',
  read_minutes INT NOT NULL DEFAULT 3,
  views BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  published_at TIMESTAMP NULL,
  INDEX idx_posts_author (author_id),
  INDEX idx_posts_status_created (status, created_at),
  INDEX idx_posts_category (category),
  CONSTRAINT fk_posts_author FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE SET NULL
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS comments (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(60) NOT NULL,
  email VARCHAR(120) NOT NULL DEFAULT '',
  content VARCHAR(1000) NOT NULL,
  status ENUM('pending', 'approved') NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_comments_status_created (status, created_at)
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS site_settings (
  setting_key VARCHAR(80) PRIMARY KEY,
  setting_value TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if err := ensureColumn(ctx, db, "posts", "author_id", "ALTER TABLE posts ADD COLUMN author_id BIGINT NULL AFTER id"); err != nil {
		return err
	}
	defaults := map[string]string{
		"siteName": "Tech Blog", "siteSubtitle": "Engineering notes", "siteUrl": "",
		"seoDescription": "记录软件工程、前端、后端与 DevOps 实践。", "seoKeywords": "技术博客,Go,Vue,DevOps",
		"defaultOgImage": "", "footerText": "持续记录，认真构建。",
	}
	for key, value := range defaults {
		if _, err := db.ExecContext(ctx, "INSERT IGNORE INTO site_settings (setting_key, setting_value) VALUES (?, ?)", key, value); err != nil { return err }
	}
	return nil
}

func ensureColumn(ctx context.Context, db *sql.DB, table, column, ddl string) error {
	var count int
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, table, column).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = db.ExecContext(ctx, ddl)
	return err
}

func ensureAdminUser(ctx context.Context, db *sql.DB) error {
	username := getenv("ADMIN_USERNAME", "admin")
	password := getenv("ADMIN_PASSWORD", "change-me")
	phone := getenv("ADMIN_PHONE", "13800000000")
	email := getenv("ADMIN_EMAIL", "admin@example.com")

	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
INSERT INTO users (username, password_hash, phone, email, role)
VALUES (?, ?, ?, ?, 'admin')
ON DUPLICATE KEY UPDATE role = 'admin'`, username, hash, phone, email)
	if err != nil {
		return err
	}
	var adminID int64
	if err := db.QueryRowContext(ctx, "SELECT id FROM users WHERE username = ?", username).Scan(&adminID); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "UPDATE posts SET author_id = ? WHERE author_id IS NULL", adminID)
	return err
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *app) createCaptcha(w http.ResponseWriter, r *http.Request) {
	id := randomHex(16)
	code := randomCode(5)
	expiresAt := time.Now().Add(10 * time.Minute)
	_, err := a.db.ExecContext(r.Context(), "INSERT INTO captchas (id, code, expires_at) VALUES (?, ?, ?)", id, code, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":        id,
		"code":      code,
		"expiresAt": expiresAt,
	})
}

func (a *app) register(w http.ResponseWriter, r *http.Request) {
	var input registerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Email = strings.TrimSpace(input.Email)
	input.Captcha = strings.TrimSpace(input.Captcha)
	if input.Username == "" || input.Password == "" || input.Phone == "" || input.Email == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("username, password, phone and email are required"))
		return
	}
	if len(input.Password) < 6 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("password must be at least 6 characters"))
		return
	}
	if err := a.verifyCaptcha(r.Context(), input.CaptchaID, input.Captcha); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	hash, err := hashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	res, err := a.db.ExecContext(r.Context(), `
INSERT INTO users (username, password_hash, phone, email, role)
VALUES (?, ?, ?, ?, 'user')`, input.Username, hash, input.Phone, input.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	id, _ := res.LastInsertId()
	u := user{ID: id, Username: input.Username, Phone: input.Phone, Email: input.Email, Role: "user", CreatedAt: time.Now()}
	token, err := a.createSession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "user": u})
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	var input loginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Captcha = strings.TrimSpace(input.Captcha)
	if input.Username == "" || input.Password == "" || input.CaptchaID == "" || input.Captcha == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("username, password and captcha are required"))
		return
	}
	if err := a.verifyCaptcha(r.Context(), input.CaptchaID, input.Captcha); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var u user
	var passwordHash string
	err := a.db.QueryRowContext(r.Context(), `
SELECT id, username, password_hash, phone, email, role, created_at
FROM users
WHERE username = ?`, input.Username).Scan(&u.ID, &u.Username, &passwordHash, &u.Phone, &u.Email, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) || !verifyPassword(input.Password, passwordHash) {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid username or password"))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	token, err := a.createSession(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
}

func (a *app) listUsers(w http.ResponseWriter, r *http.Request, au authUser) {
	rows, err := a.db.QueryContext(r.Context(), `
SELECT u.id, u.username, u.phone, u.email, u.role, u.created_at, COUNT(p.id)
FROM users u
LEFT JOIN posts p ON p.author_id = u.id
GROUP BY u.id, u.username, u.phone, u.email, u.role, u.created_at
ORDER BY u.created_at DESC, u.id DESC
LIMIT 500`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	type userInfo struct {
		user
		PostCount int64 `json:"postCount"`
	}
	items := []userInfo{}
	for rows.Next() {
		var item userInfo
		if err := rows.Scan(&item.ID, &item.Username, &item.Phone, &item.Email, &item.Role, &item.CreatedAt, &item.PostCount); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *app) updateUser(w http.ResponseWriter, r *http.Request, au authUser) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}
	if id == au.ID {
		writeError(w, http.StatusBadRequest, fmt.Errorf("cannot change your own role"))
		return
	}
	var input struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || (input.Role != "user" && input.Role != "admin") {
		writeError(w, http.StatusBadRequest, fmt.Errorf("role must be user or admin"))
		return
	}
	if _, err := a.db.ExecContext(r.Context(), "UPDATE users SET role = ? WHERE id = ?", input.Role, id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "role": input.Role})
}

func (a *app) deleteUser(w http.ResponseWriter, r *http.Request, au authUser) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}
	if id == au.ID {
		writeError(w, http.StatusBadRequest, fmt.Errorf("cannot delete your own account"))
		return
	}
	if _, err := a.db.ExecContext(r.Context(), "DELETE FROM users WHERE id = ?", id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) logout(w http.ResponseWriter, r *http.Request, au authUser) {
	token := bearerToken(r)
	_, _ = a.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE token = ?", token)
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) me(w http.ResponseWriter, r *http.Request, au authUser) {
	writeJSON(w, http.StatusOK, au.user)
}

func (a *app) verifyCaptcha(ctx context.Context, id, code string) error {
	var stored string
	var expiresAt time.Time
	var used bool
	err := a.db.QueryRowContext(ctx, "SELECT code, expires_at, used FROM captchas WHERE id = ?", id).Scan(&stored, &expiresAt, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("captcha is invalid")
	}
	if err != nil {
		return err
	}
	if used || time.Now().After(expiresAt) {
		return fmt.Errorf("captcha has expired")
	}
	if !strings.EqualFold(stored, code) {
		return fmt.Errorf("captcha is incorrect")
	}
	_, err = a.db.ExecContext(ctx, "UPDATE captchas SET used = 1 WHERE id = ?", id)
	return err
}

func (a *app) createSession(ctx context.Context, userID int64) (string, error) {
	token := randomHex(32)
	_, err := a.db.ExecContext(ctx, "INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, time.Now().Add(7*24*time.Hour))
	return token, err
}

func (a *app) listPosts(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	clauses := []string{"p.status = 'published'"}
	args := []any{}
	if category != "" {
		clauses = append(clauses, "p.category = ?")
		args = append(args, category)
	}
	if q != "" {
		clauses = append(clauses, "(p.title LIKE ? OR p.summary LIKE ? OR p.content LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}

	rows, err := a.db.QueryContext(r.Context(), `
SELECT p.id, p.author_id, COALESCE(u.username, ''), p.title, p.slug, p.summary, p.content, p.category, p.tags, p.cover_url, p.status, p.read_minutes, p.views, p.created_at, p.updated_at, p.published_at
FROM posts p
LEFT JOIN users u ON u.id = p.author_id
WHERE `+strings.Join(clauses, " AND ")+`
ORDER BY COALESCE(p.published_at, p.created_at) DESC, p.id DESC
LIMIT 100`, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	posts, err := scanPosts(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	categories := make(map[string]int)
	for _, item := range posts {
		categories[item.Category]++
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": posts, "categories": categories})
}

func (a *app) listManagePosts(w http.ResponseWriter, r *http.Request, au authUser) {
	clauses := []string{"1 = 1"}
	args := []any{}
	if au.Role != "admin" {
		clauses = append(clauses, "p.author_id = ?")
		args = append(args, au.ID)
	}
	rows, err := a.db.QueryContext(r.Context(), `
SELECT p.id, p.author_id, COALESCE(u.username, ''), p.title, p.slug, p.summary, p.content, p.category, p.tags, p.cover_url, p.status, p.read_minutes, p.views, p.created_at, p.updated_at, p.published_at
FROM posts p
LEFT JOIN users u ON u.id = p.author_id
WHERE `+strings.Join(clauses, " AND ")+`
ORDER BY p.updated_at DESC, p.id DESC
LIMIT 200`, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	posts, err := scanPosts(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": posts})
}

func (a *app) getPost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(r.Context(), `
SELECT p.id, p.author_id, COALESCE(u.username, ''), p.title, p.slug, p.summary, p.content, p.category, p.tags, p.cover_url, p.status, p.read_minutes, p.views, p.created_at, p.updated_at, p.published_at
FROM posts p
LEFT JOIN users u ON u.id = p.author_id
WHERE p.slug = ? AND p.status = 'published'`, slug)

	item, err := scanPost(row)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, fmt.Errorf("post not found"))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if _, err := tx.ExecContext(r.Context(), "UPDATE posts SET views = views + 1 WHERE id = ?", item.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	item.Views++
	writeJSON(w, http.StatusOK, item)
}

func (a *app) listComments(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), "SELECT id, name, '', content, status, created_at FROM comments WHERE status = 'approved' ORDER BY created_at DESC LIMIT 100")
	if err != nil { writeError(w, http.StatusInternalServerError, err); return }
	defer rows.Close()
	items, err := scanComments(rows)
	if err != nil { writeError(w, http.StatusInternalServerError, err); return }
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *app) createComment(w http.ResponseWriter, r *http.Request) {
	var input commentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil { writeError(w, http.StatusBadRequest, err); return }
	input.Name, input.Email, input.Content = strings.TrimSpace(input.Name), strings.TrimSpace(input.Email), strings.TrimSpace(input.Content)
	if input.Name == "" || input.Content == "" { writeError(w, http.StatusBadRequest, fmt.Errorf("name and content are required")); return }
	if len([]rune(input.Name)) > 60 || len([]rune(input.Content)) > 1000 || len(input.Email) > 120 { writeError(w, http.StatusBadRequest, fmt.Errorf("comment is too long")); return }
	res, err := a.db.ExecContext(r.Context(), "INSERT INTO comments (name, email, content) VALUES (?, ?, ?)", input.Name, input.Email, input.Content)
	if err != nil { writeError(w, http.StatusInternalServerError, err); return }
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "message": "留言已提交，审核后展示"})
}

func (a *app) listAdminComments(w http.ResponseWriter, r *http.Request, au authUser) {
	rows, err := a.db.QueryContext(r.Context(), "SELECT id, name, email, content, status, created_at FROM comments ORDER BY created_at DESC LIMIT 300")
	if err != nil { writeError(w, http.StatusInternalServerError, err); return }
	defer rows.Close()
	items, err := scanComments(rows)
	if err != nil { writeError(w, http.StatusInternalServerError, err); return }
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func scanComments(rows *sql.Rows) ([]comment, error) {
	items := []comment{}
	for rows.Next() { var item comment; if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Content, &item.Status, &item.CreatedAt); err != nil { return nil, err }; items = append(items, item) }
	return items, rows.Err()
}

func (a *app) updateComment(w http.ResponseWriter, r *http.Request, au authUser) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64); if err != nil { writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id")); return }
	var input struct { Status string `json:"status"` }
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || (input.Status != "pending" && input.Status != "approved") { writeError(w, http.StatusBadRequest, fmt.Errorf("invalid status")); return }
	if _, err := a.db.ExecContext(r.Context(), "UPDATE comments SET status = ? WHERE id = ?", input.Status, id); err != nil { writeError(w, http.StatusInternalServerError, err); return }
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (a *app) deleteComment(w http.ResponseWriter, r *http.Request, au authUser) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64); if err != nil { writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id")); return }
	if _, err := a.db.ExecContext(r.Context(), "DELETE FROM comments WHERE id = ?", id); err != nil { writeError(w, http.StatusInternalServerError, err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) getSettings(w http.ResponseWriter, r *http.Request) { a.writeSettings(w, r) }

func (a *app) writeSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), "SELECT setting_key, setting_value FROM site_settings")
	if err != nil { writeError(w, http.StatusInternalServerError, err); return }
	defer rows.Close(); values := map[string]string{}
	for rows.Next() { var k, v string; if err := rows.Scan(&k, &v); err != nil { writeError(w, http.StatusInternalServerError, err); return }; values[k] = v }
	writeJSON(w, http.StatusOK, siteSettings{values["siteName"], values["siteSubtitle"], values["siteUrl"], values["seoDescription"], values["seoKeywords"], values["defaultOgImage"], values["footerText"]})
}

func (a *app) updateSettings(w http.ResponseWriter, r *http.Request, au authUser) {
	var input siteSettings
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil { writeError(w, http.StatusBadRequest, err); return }
	if strings.TrimSpace(input.SiteName) == "" { writeError(w, http.StatusBadRequest, fmt.Errorf("site name is required")); return }
	values := map[string]string{"siteName": input.SiteName, "siteSubtitle": input.SiteSubtitle, "siteUrl": input.SiteURL, "seoDescription": input.SEODescription, "seoKeywords": input.SEOKeywords, "defaultOgImage": input.DefaultOGImage, "footerText": input.FooterText}
	tx, err := a.db.BeginTx(r.Context(), nil); if err != nil { writeError(w, http.StatusInternalServerError, err); return }; defer tx.Rollback()
	for key, value := range values { if _, err := tx.ExecContext(r.Context(), "INSERT INTO site_settings (setting_key, setting_value) VALUES (?, ?) ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value)", key, strings.TrimSpace(value)); err != nil { writeError(w, http.StatusInternalServerError, err); return } }
	if err := tx.Commit(); err != nil { writeError(w, http.StatusInternalServerError, err); return }
	writeJSON(w, http.StatusOK, input)
}

func (a *app) adminStats(w http.ResponseWriter, r *http.Request, au authUser) {
	var posts, published, pending, users int64
	queries := []struct{ q string; dest *int64 }{{"SELECT COUNT(*) FROM posts", &posts}, {"SELECT COUNT(*) FROM posts WHERE status='published'", &published}, {"SELECT COUNT(*) FROM comments WHERE status='pending'", &pending}, {"SELECT COUNT(*) FROM users", &users}}
	for _, item := range queries { if err := a.db.QueryRowContext(r.Context(), item.q).Scan(item.dest); err != nil { writeError(w, http.StatusInternalServerError, err); return } }
	writeJSON(w, http.StatusOK, map[string]int64{"posts": posts, "published": published, "pendingComments": pending, "users": users})
}

func (a *app) createPost(w http.ResponseWriter, r *http.Request, au authUser) {
	input, ok := decodeAndValidate(w, r)
	if !ok {
		return
	}
	tagsJSON, _ := json.Marshal(input.Tags)
	publishedAt := publishedValue(input.Status)

	res, err := a.db.ExecContext(r.Context(), `
INSERT INTO posts (author_id, title, slug, summary, content, category, tags, cover_url, status, read_minutes, published_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		au.ID, input.Title, input.Slug, input.Summary, input.Content, input.Category, tagsJSON, input.CoverURL, input.Status, input.ReadMinutes, publishedAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *app) updatePost(w http.ResponseWriter, r *http.Request, au authUser) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}
	if err := a.ensurePostAccess(r.Context(), id, au); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	input, ok := decodeAndValidate(w, r)
	if !ok {
		return
	}
	tagsJSON, _ := json.Marshal(input.Tags)
	publishedAt := publishedValue(input.Status)

	_, err = a.db.ExecContext(r.Context(), `
UPDATE posts
SET title = ?, slug = ?, summary = ?, content = ?, category = ?, tags = ?, cover_url = ?, status = ?, read_minutes = ?,
    published_at = CASE WHEN ? IS NOT NULL THEN COALESCE(published_at, ?) ELSE NULL END
WHERE id = ?`,
		input.Title, input.Slug, input.Summary, input.Content, input.Category, tagsJSON, input.CoverURL, input.Status, input.ReadMinutes, publishedAt, publishedAt, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (a *app) deletePost(w http.ResponseWriter, r *http.Request, au authUser) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}
	if err := a.ensurePostAccess(r.Context(), id, au); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	if _, err := a.db.ExecContext(r.Context(), "DELETE FROM posts WHERE id = ?", id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) uploadImage(w http.ResponseWriter, r *http.Request, au authUser) {
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("image must be smaller than 10MB"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("file is required"))
		return
	}
	defer file.Close()

	head := make([]byte, 512)
	n, _ := file.Read(head)
	contentType := http.DetectContentType(head[:n])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !strings.HasPrefix(contentType, "image/") {
		writeError(w, http.StatusBadRequest, fmt.Errorf("only image files are allowed"))
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		extensions, _ := mime.ExtensionsByType(contentType)
		if len(extensions) > 0 {
			ext = extensions[0]
		}
	}
	if ext == "" {
		ext = ".img"
	}
	name := time.Now().Format("20060102") + "-" + randomHex(8) + ext
	dir := uploadDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dstPath := filepath.Join(dir, name)
	dst, err := os.Create(dstPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"url": "/uploads/" + name})
}

func (a *app) ensurePostAccess(ctx context.Context, postID int64, au authUser) error {
	if au.Role == "admin" {
		return nil
	}
	var authorID sql.NullInt64
	err := a.db.QueryRowContext(ctx, "SELECT author_id FROM posts WHERE id = ?", postID).Scan(&authorID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("post not found")
	}
	if err != nil {
		return err
	}
	if !authorID.Valid || authorID.Int64 != au.ID {
		return fmt.Errorf("only the author or an admin can manage this post")
	}
	return nil
}

func (a *app) requireAuth(next func(http.ResponseWriter, *http.Request, authUser)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("login required"))
			return
		}
		au, err := a.userByToken(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("session is invalid or expired"))
			return
		}
		next(w, r, au)
	}
}

func (a *app) requireAdmin(next func(http.ResponseWriter, *http.Request, authUser)) http.HandlerFunc {
	return a.requireAuth(func(w http.ResponseWriter, r *http.Request, au authUser) {
		if au.Role != "admin" { writeError(w, http.StatusForbidden, fmt.Errorf("admin access required")); return }
		next(w, r, au)
	})
}

func (a *app) userByToken(ctx context.Context, token string) (authUser, error) {
	var au authUser
	err := a.db.QueryRowContext(ctx, `
SELECT u.id, u.username, u.phone, u.email, u.role, u.created_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token = ? AND s.expires_at > NOW()`, token).Scan(&au.ID, &au.Username, &au.Phone, &au.Email, &au.Role, &au.CreatedAt)
	return au, err
}

func bearerToken(r *http.Request) string {
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
}

func decodeAndValidate(w http.ResponseWriter, r *http.Request) (postInput, bool) {
	var input postInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return input, false
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Slug = strings.TrimSpace(input.Slug)
	input.Summary = strings.TrimSpace(input.Summary)
	input.Content = strings.TrimSpace(input.Content)
	input.Category = strings.TrimSpace(input.Category)
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.ReadMinutes <= 0 {
		input.ReadMinutes = 3
	}
	if input.Title == "" || input.Slug == "" || input.Summary == "" || input.Content == "" || input.Category == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("title, slug, summary, content and category are required"))
		return input, false
	}
	if input.Status != "draft" && input.Status != "published" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("status must be draft or published"))
		return input, false
	}
	return input, true
}

type scanner interface {
	Scan(dest ...any) error
}

func scanPosts(rows *sql.Rows) ([]post, error) {
	posts := []post{}
	for rows.Next() {
		item, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, item)
	}
	return posts, rows.Err()
}

func scanPost(s scanner) (post, error) {
	var item post
	var authorID sql.NullInt64
	var tagsRaw []byte
	var publishedAt sql.NullTime
	err := s.Scan(&item.ID, &authorID, &item.AuthorName, &item.Title, &item.Slug, &item.Summary, &item.Content, &item.Category, &tagsRaw, &item.CoverURL, &item.Status, &item.ReadMinutes, &item.Views, &item.CreatedAt, &item.UpdatedAt, &publishedAt)
	if err != nil {
		return item, err
	}
	if authorID.Valid {
		item.AuthorID = authorID.Int64
	}
	if len(tagsRaw) > 0 {
		_ = json.Unmarshal(tagsRaw, &item.Tags)
	}
	if item.Tags == nil {
		item.Tags = []string{}
	}
	if publishedAt.Valid {
		item.PublishedAt = &publishedAt.Time
	}
	return item, nil
}

func publishedValue(status string) any {
	if status == "published" {
		return time.Now()
	}
	return nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := passwordDigest(password, salt)
	return "v1$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(sum), nil
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "v1" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	got := passwordDigest(password, salt)
	return hmac.Equal(got, want)
}

func passwordDigest(password string, salt []byte) []byte {
	sum := append([]byte{}, salt...)
	sum = append(sum, []byte(password)...)
	for i := 0; i < 120000; i++ {
		hash := sha256.Sum256(sum)
		sum = hash[:]
	}
	return sum
}

func randomHex(bytesLen int) string {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		hash := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
		return hex.EncodeToString(hash[:bytesLen])
	}
	return hex.EncodeToString(buf)
}

func randomCode(length int) string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, length)
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		for i := range random {
			random[i] = byte(time.Now().UnixNano() >> (i % 8))
		}
	}
	for i, b := range random {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func uploadDir() string {
	return getenv("UPLOAD_DIR", filepath.Join("..", "uploads"))
}
