package httpx

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func JSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func JSONList[T any](w http.ResponseWriter, items []T) {
	if items == nil {
		items = []T{}
	}
	JSON(w, http.StatusOK, items)
}

func Err(w http.ResponseWriter, code int, msg string) {
	JSON(w, code, map[string]string{"error": msg})
}

func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		Err(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

// DBError maps database errors to safe responses; internals are logged, never returned.
func DBError(w http.ResponseWriter, err error) {
	var pe *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		Err(w, http.StatusNotFound, "not found")
	case errors.As(err, &pe) && pe.Code == "23505":
		Err(w, http.StatusConflict, "already exists")
	case errors.As(err, &pe) && pe.Code == "23503":
		Err(w, http.StatusConflict, "referenced record does not exist or is still in use")
	case errors.As(err, &pe) && pe.Code == "23514":
		Err(w, http.StatusUnprocessableEntity, "value violates a constraint")
	default:
		log.Printf("db error: %v", err)
		Err(w, http.StatusInternalServerError, "internal error")
	}
}

func IDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		Err(w, http.StatusNotFound, "not found")
		return 0, false
	}
	return id, true
}

func Page(r *http.Request) (limit, offset int) {
	limit = 50
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
		offset = v
	}
	return
}

// SameOrigin blocks cross-site state-changing requests (defence in depth on top of
// SameSite cookies) and sets basic response headers.
func SameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if o := r.Header.Get("Origin"); o != "" {
				if u, err := url.Parse(o); err != nil || u.Host != r.Host {
					Err(w, http.StatusForbidden, "cross-origin request blocked")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
