package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"ims/internal/httpx"
)

const (
	cookieName = "ims_session"
	sessionTTL = 30 * 24 * time.Hour
	RoleUser   = "user"
	RoleAdmin  = "admin"
)

type ctxKey struct{}
type roleKey struct{}

// UserID returns the authenticated tenant ID. Only Middleware sets it from the session.
func UserID(ctx context.Context) int64 {
	id, _ := ctx.Value(ctxKey{}).(int64)
	return id
}

// Role returns the authenticated user's role ("user" or "admin").
func Role(ctx context.Context) string {
	r, _ := ctx.Value(roleKey{}).(string)
	return r
}

// IsAdmin reports whether the current request is authenticated as an admin.
func IsAdmin(ctx context.Context) bool {
	return Role(ctx) == RoleAdmin
}

type Handler struct {
	DB                *pgxpool.Pool
	Secure            bool // true in production (HTTPS-only cookie)
	AllowRegistration bool // emergency only; prefer admin-created accounts
	lim               limiter
}

func (h *Handler) Public(r chi.Router) {
	r.Post("/auth/register", h.register)
	r.Post("/auth/login", h.login)
}

func (h *Handler) Private(r chi.Router) {
	r.Post("/auth/logout", h.logout)
	r.Get("/me", h.me)
}

type credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), 12)

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	// Self-registration is closed by default. Admins create accounts via /api/admin/users.
	// ALLOW_REGISTRATION=true is an emergency escape hatch only.
	if !h.AllowRegistration {
		httpx.Err(w, http.StatusForbidden, "registration is closed")
		return
	}
	if !h.throttle(w, r) {
		return
	}
	var in credentials
	if !httpx.Decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	addr, err := mail.ParseAddress(in.Email)
	if err != nil || addr.Address != in.Email || in.Name == "" || len(in.Password) < 10 || len(in.Password) > 72 {
		httpx.Err(w, http.StatusUnprocessableEntity, "name, a valid email and a 10-72 character password are required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if err != nil {
		httpx.Err(w, http.StatusInternalServerError, "internal error")
		return
	}
	var id int64
	err = h.DB.QueryRow(r.Context(),
		`INSERT INTO users (name, email, password_hash, role) VALUES ($1,$2,$3,'user') RETURNING id`,
		in.Name, in.Email, string(hash)).Scan(&id)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	if err := h.startSession(w, r, id); err != nil {
		httpx.Err(w, http.StatusInternalServerError, "could not start session")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id, "name": in.Name, "email": in.Email, "role": RoleUser})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if !h.throttle(w, r) {
		return
	}
	var in credentials
	if !httpx.Decode(w, r, &in) {
		return
	}
	var id int64
	var hash, status, role string
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, password_hash, status, role FROM users WHERE lower(email) = lower($1)`, in.Email).Scan(&id, &hash, &status, &role)
	found := true
	if errors.Is(err, pgx.ErrNoRows) {
		found, hash = false, string(dummyHash) // constant-ish timing for unknown emails
	} else if err != nil {
		httpx.DBError(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil || !found {
		httpx.Err(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if status != "active" {
		httpx.Err(w, http.StatusForbidden, "account suspended")
		return
	}
	if err := h.startSession(w, r, id); err != nil {
		httpx.Err(w, http.StatusInternalServerError, "could not start session")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id, "role": role})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		sum := sha256.Sum256([]byte(c.Value))
		_, _ = h.DB.Exec(r.Context(), `DELETE FROM sessions WHERE token_hash = $1`, sum[:])
	}
	http.SetCookie(w, h.sessionCookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	uid := UserID(r.Context())
	var name, email, role string
	if err := h.DB.QueryRow(r.Context(), `SELECT name, email, role FROM users WHERE id = $1`, uid).Scan(&name, &email, &role); err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": uid, "name": name, "email": email, "role": role})
}

func (h *Handler) sessionCookie(value string, maxAge int) *http.Cookie {
	c := &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.Secure,
		MaxAge:   maxAge,
	}
	// Cross-origin SPAs require SameSite=None; otherwise the browser will not send the cookie.
	if h.Secure {
		c.SameSite = http.SameSiteNoneMode
	} else {
		c.SameSite = http.SameSiteLaxMode
	}
	return c
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request, uid int64) error {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token)) // only the hash is stored
	if _, err := h.DB.Exec(r.Context(),
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1,$2,$3)`,
		sum[:], uid, time.Now().Add(sessionTTL)); err != nil {
		return err
	}
	http.SetCookie(w, h.sessionCookie(token, int(sessionTTL.Seconds())))
	return nil
}

// Middleware resolves the session cookie to a tenant ID and role.
// Suspended users lose access immediately; their data is untouched.
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			httpx.Err(w, http.StatusUnauthorized, "authentication required")
			return
		}
		sum := sha256.Sum256([]byte(c.Value))
		var uid int64
		var role string
		err = h.DB.QueryRow(r.Context(),
			`SELECT s.user_id, u.role FROM sessions s JOIN users u ON u.id = s.user_id
			 WHERE s.token_hash = $1 AND s.expires_at > now() AND u.status = 'active'`, sum[:]).Scan(&uid, &role)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Err(w, http.StatusUnauthorized, "authentication required")
			return
		} else if err != nil {
			httpx.DBError(w, err)
			return
		}
		// Best-effort RLS context. With a connection pool this is not sticky across
		// queries; app-level user_id filters remain the primary isolation layer.
		_, _ = h.DB.Exec(r.Context(), `SELECT set_config('app.current_user_id', $1, false)`, strconv.FormatInt(uid, 10))

		ctx := context.WithValue(r.Context(), ctxKey{}, uid)
		ctx = context.WithValue(ctx, roleKey{}, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin rejects non-admin authenticated users.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdmin(r.Context()) {
			httpx.Err(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP returns the client IP, preferring the first X-Forwarded-For hop when
// the request is behind a trusted reverse proxy (Railway, Render, Fly, etc.).
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// In-memory per-IP limiter: fine for one instance.
type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func (h *Handler) throttle(w http.ResponseWriter, r *http.Request) bool {
	ip := ClientIP(r)
	l := &h.lim
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hits == nil {
		l.hits = map[string][]time.Time{}
	}
	now := time.Now()
	kept := l.hits[ip][:0]
	for _, t := range l.hits[ip] {
		if t.After(now.Add(-time.Minute)) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= 10 {
		l.hits[ip] = kept
		httpx.Err(w, http.StatusTooManyRequests, "too many attempts, try again later")
		return false
	}
	l.hits[ip] = append(kept, now)
	return true
}

// HashPassword is exported for admin create-user and bootstrap.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(b), err
}

// ValidateNewUser checks name, email, password for create flows.
func ValidateNewUser(name, email, password string) error {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || name == "" || len(password) < 10 || len(password) > 72 {
		return errors.New("name, a valid email and a 10-72 character password are required")
	}
	return nil
}
