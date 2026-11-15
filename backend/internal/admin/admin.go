package admin

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ims/internal/auth"
	"ims/internal/httpx"
)

// Handler exposes user-management endpoints for admins only.
// No inventory/finance features — pure account provisioning.
type Handler struct {
	DB *pgxpool.Pool
}

func (h *Handler) Routes(r chi.Router) {
	r.Use(auth.RequireAdmin)
	r.Get("/admin/users", h.list)
	r.Post("/admin/users", h.create)
	r.Put("/admin/users/{id}/status", h.setStatus)
	r.Put("/admin/users/{id}/role", h.setRole)
	r.Put("/admin/users/{id}/password", h.setPassword)
}

type userRow struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	rows, err := h.DB.Query(r.Context(),
		`SELECT id, name, email, role, status, to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM users ORDER BY id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[userRow])
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSONList(w, items)
}

type createIn struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"` // "user" (default) or "admin"
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in createIn
	if !httpx.Decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if err := auth.ValidateNewUser(in.Name, in.Email, in.Password); err != nil {
		httpx.Err(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	role := auth.RoleUser
	if in.Role == auth.RoleAdmin {
		role = auth.RoleAdmin
	} else if in.Role != "" && in.Role != auth.RoleUser {
		httpx.Err(w, http.StatusUnprocessableEntity, "role must be user or admin")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		httpx.Err(w, http.StatusInternalServerError, "internal error")
		return
	}
	var out userRow
	err = h.DB.QueryRow(r.Context(),
		`INSERT INTO users (name, email, password_hash, role)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, name, email, role, status,
		           to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
		in.Name, in.Email, hash, role).Scan(&out.ID, &out.Name, &out.Email, &out.Role, &out.Status, &out.CreatedAt)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

type statusIn struct {
	Status string `json:"status"` // active | suspended
}

func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDParam(w, r)
	if !ok {
		return
	}
	if id == auth.UserID(r.Context()) {
		httpx.Err(w, http.StatusUnprocessableEntity, "cannot change your own status")
		return
	}
	var in statusIn
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.Status != "active" && in.Status != "suspended" {
		httpx.Err(w, http.StatusUnprocessableEntity, "status must be active or suspended")
		return
	}
	tag, err := h.DB.Exec(r.Context(),
		`UPDATE users SET status = $2, updated_at = now() WHERE id = $1`, id, in.Status)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Err(w, http.StatusNotFound, "not found")
		return
	}
	// Suspended users lose sessions immediately.
	if in.Status == "suspended" {
		_, _ = h.DB.Exec(r.Context(), `DELETE FROM sessions WHERE user_id = $1`, id)
	}
	w.WriteHeader(http.StatusNoContent)
}

type roleIn struct {
	Role string `json:"role"`
}

func (h *Handler) setRole(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDParam(w, r)
	if !ok {
		return
	}
	if id == auth.UserID(r.Context()) {
		httpx.Err(w, http.StatusUnprocessableEntity, "cannot change your own role")
		return
	}
	var in roleIn
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.Role != auth.RoleUser && in.Role != auth.RoleAdmin {
		httpx.Err(w, http.StatusUnprocessableEntity, "role must be user or admin")
		return
	}
	tag, err := h.DB.Exec(r.Context(),
		`UPDATE users SET role = $2, updated_at = now() WHERE id = $1`, id, in.Role)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Err(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type passwordIn struct {
	Password string `json:"password"`
}

func (h *Handler) setPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDParam(w, r)
	if !ok {
		return
	}
	var in passwordIn
	if !httpx.Decode(w, r, &in) {
		return
	}
	if len(in.Password) < 10 || len(in.Password) > 72 {
		httpx.Err(w, http.StatusUnprocessableEntity, "password must be 10-72 characters")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		httpx.Err(w, http.StatusInternalServerError, "internal error")
		return
	}
	tag, err := h.DB.Exec(r.Context(),
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, hash)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Err(w, http.StatusNotFound, "not found")
		return
	}
	// Force re-login after password change.
	_, _ = h.DB.Exec(r.Context(), `DELETE FROM sessions WHERE user_id = $1`, id)
	w.WriteHeader(http.StatusNoContent)
}
