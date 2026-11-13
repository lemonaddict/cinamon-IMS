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
)

type ctxKey struct{}

// UserID returns the authenticated tenant ID. It is only ever set by Middleware
// from the session; no request field can supply it.
func UserID(ctx context.Context) int64 {
	id, _ := ctx.Value(ctxKey{}).(int64)
	return id
}

type Handler struct {
	DB     *pgxpool.Pool
	Secure bool // true in production (HTTPS-only cookie)
	AllowRegistration bool
	lim    limiter
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
		`INSERT INTO users (name, email, password_hash) VALUES ($1,$2,$3) RETURNING id`,
		in.Name, in.Email, string(hash)).Scan(&id)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	if err := h.startSession(w, r, id); err != nil {
		httpx.Err(w, http.StatusInternalServerError, "could not start session")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id, "name": in.Name, "email": in.Email})
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
	var hash, status string
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, password_hash, status FROM users WHERE lower(email) = lower($1)`, in.Email).Scan(&id, &hash, &status)
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
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		sum := sha256.Sum256([]byte(c.Value))
		_, _ = h.DB.Exec(r.Context(), `DELETE FROM sessions WHERE token_hash = $1`, sum[:])
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/", MaxAge: -1, HttpOnly: true, Secure: h.Secure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	uid := UserID(r.Context())
	var name, email string
	if err := h.DB.QueryRow(r.Context(), `SELECT name, email FROM users WHERE id = $1`, uid).Scan(&name, &email); err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": uid, "name": name, "email": email})
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
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true,
		Secure: h.Secure, SameSite: http.SameSiteLaxMode, MaxAge: int(sessionTTL.Seconds())})
	return nil
}

// Middleware resolves the session cookie to a tenant ID. Suspended users lose
// access immediately; their data is untouched.
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			httpx.Err(w, http.StatusUnauthorized, "authentication required")
			return
		}
		sum := sha256.Sum256([]byte(c.Value))
		var uid int64
		err = h.DB.QueryRow(r.Context(),
			`SELECT s.user_id FROM sessions s JOIN users u ON u.id = s.user_id
			 WHERE s.token_hash = $1 AND s.expires_at > now() AND u.status = 'active'`, sum[:]).Scan(&uid)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Err(w, http.StatusUnauthorized, "authentication required")
			return
		} else if err != nil {
			httpx.DBError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, uid)))
	})
}

// In-memory per-IP limiter: fine for one instance. Behind Caddy, key on the
// forwarded client IP instead of RemoteAddr.
type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func (h *Handler) throttle(w http.ResponseWriter, r *http.Request) bool {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
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
