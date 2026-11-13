package catalog

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ims/internal/auth"
	"ims/internal/httpx"
)

type Handler struct{ DB *pgxpool.Pool }

func (h *Handler) Routes(r chi.Router) {
	h.named(r, "/categories", "categories")
	h.named(r, "/locations", "locations")
	r.Get("/products", h.listProducts)
	r.Post("/products", h.createProduct)
	r.Get("/products/{id}", h.getProduct)
	r.Put("/products/{id}", h.updateProduct)
	r.Delete("/products/{id}", h.deleteProduct)
}

type named struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func readName(w http.ResponseWriter, r *http.Request) (string, bool) {
	var in struct {
		Name string `json:"name"`
	}
	if !httpx.Decode(w, r, &in) {
		return "", false
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 200 {
		httpx.Err(w, http.StatusUnprocessableEntity, "name is required (max 200 characters)")
		return "", false
	}
	return name, true
}

// named registers CRUD for tables shaped (id, user_id, name). `table` is always a
// constant from Routes, never user input, so Sprintf is safe here.
func (h *Handler) named(r chi.Router, path, table string) {
	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		limit, offset := httpx.Page(r)
		rows, err := h.DB.Query(r.Context(), fmt.Sprintf(
			`SELECT id, name FROM %s WHERE user_id = $1 ORDER BY name LIMIT $2 OFFSET $3`, table),
			auth.UserID(r.Context()), limit, offset)
		if err != nil {
			httpx.DBError(w, err)
			return
		}
		items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[named])
		if err != nil {
			httpx.DBError(w, err)
			return
		}
		httpx.JSONList(w, items)
	})
	r.Post(path, func(w http.ResponseWriter, r *http.Request) {
		name, ok := readName(w, r)
		if !ok {
			return
		}
		var out named
		err := h.DB.QueryRow(r.Context(), fmt.Sprintf(
			`INSERT INTO %s (user_id, name) VALUES ($1,$2) RETURNING id, name`, table),
			auth.UserID(r.Context()), name).Scan(&out.ID, &out.Name)
		if err != nil {
			httpx.DBError(w, err)
			return
		}
		httpx.JSON(w, http.StatusCreated, out)
	})
	r.Put(path+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := httpx.IDParam(w, r)
		if !ok {
			return
		}
		name, ok := readName(w, r)
		if !ok {
			return
		}
		var out named
		err := h.DB.QueryRow(r.Context(), fmt.Sprintf(
			`UPDATE %s SET name = $3, updated_at = now() WHERE user_id = $1 AND id = $2 RETURNING id, name`, table),
			auth.UserID(r.Context()), id, name).Scan(&out.ID, &out.Name)
		if err != nil {
			httpx.DBError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, out)
	})
	r.Delete(path+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := httpx.IDParam(w, r)
		if !ok {
			return
		}
		tag, err := h.DB.Exec(r.Context(), fmt.Sprintf(
			`DELETE FROM %s WHERE user_id = $1 AND id = $2`, table), auth.UserID(r.Context()), id)
		if err != nil {
			httpx.DBError(w, err)
			return
		}
		if tag.RowsAffected() == 0 {
			httpx.Err(w, http.StatusNotFound, "not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

type product struct {
	ID          int64   `json:"id"`
	CategoryID  *int64  `json:"category_id"`
	Name        string  `json:"name"`
	SKU         string  `json:"sku"`
	Description string  `json:"description"`
	ImagePath   *string `json:"image_path"`
}

type productIn struct {
	CategoryID  *int64 `json:"category_id"`
	Name        string `json:"name"`
	SKU         string `json:"sku"`
	Description string `json:"description"`
}

const productCols = `id, category_id, name, sku, description, image_path`

func readProduct(w http.ResponseWriter, r *http.Request) (productIn, bool) {
	var in productIn
	if !httpx.Decode(w, r, &in) {
		return in, false
	}
	in.Name, in.SKU = strings.TrimSpace(in.Name), strings.TrimSpace(in.SKU)
	if in.Name == "" || len(in.Name) > 200 || in.SKU == "" || len(in.SKU) > 64 || len(in.Description) > 2000 {
		httpx.Err(w, http.StatusUnprocessableEntity, "name (max 200) and sku (max 64) are required; description max 2000")
		return in, false
	}
	return in, true
}

func scanProduct(row pgx.Row) (product, error) {
	var p product
	err := row.Scan(&p.ID, &p.CategoryID, &p.Name, &p.SKU, &p.Description, &p.ImagePath)
	return p, err
}

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	rows, err := h.DB.Query(r.Context(),
		`SELECT `+productCols+` FROM products WHERE user_id = $1 ORDER BY name LIMIT $2 OFFSET $3`,
		auth.UserID(r.Context()), limit, offset)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[product])
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSONList(w, items)
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	in, ok := readProduct(w, r)
	if !ok {
		return
	}
	p, err := scanProduct(h.DB.QueryRow(r.Context(),
		`INSERT INTO products (user_id, category_id, name, sku, description) VALUES ($1,$2,$3,$4,$5) RETURNING `+productCols,
		auth.UserID(r.Context()), in.CategoryID, in.Name, in.SKU, in.Description))
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

func (h *Handler) getProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDParam(w, r)
	if !ok {
		return
	}
	p, err := scanProduct(h.DB.QueryRow(r.Context(),
		`SELECT `+productCols+` FROM products WHERE user_id = $1 AND id = $2`, auth.UserID(r.Context()), id))
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDParam(w, r)
	if !ok {
		return
	}
	in, ok := readProduct(w, r)
	if !ok {
		return
	}
	p, err := scanProduct(h.DB.QueryRow(r.Context(),
		`UPDATE products SET category_id=$3, name=$4, sku=$5, description=$6, updated_at=now()
		 WHERE user_id = $1 AND id = $2 RETURNING `+productCols,
		auth.UserID(r.Context()), id, in.CategoryID, in.Name, in.SKU, in.Description))
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *Handler) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDParam(w, r)
	if !ok {
		return
	}
	tag, err := h.DB.Exec(r.Context(), `DELETE FROM products WHERE user_id = $1 AND id = $2`, auth.UserID(r.Context()), id)
	if err != nil {
		httpx.DBError(w, err) // 409 if the product has stock history
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Err(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
