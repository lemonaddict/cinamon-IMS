package finance

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ims/internal/auth"
	"ims/internal/httpx"
)

type Handler struct{ DB *pgxpool.Pool }

func (h *Handler) Routes(r chi.Router) {
	r.Get("/finance", h.list)
	r.Get("/finance/summary", h.summary)
	r.Post("/finance", h.create)
	r.Put("/finance/{id}", h.update)
	r.Delete("/finance/{id}", h.delete)
}

type entry struct {
	ID        int64  `json:"id"`
	EntryType string `json:"entry_type"`
	Category  string `json:"category"`
	Amount    int64  `json:"amount"`
	EntryDate string `json:"entry_date"`
	Notes     string `json:"notes"`
}

type entryIn struct {
	EntryType string `json:"entry_type"`
	Category  string `json:"category"`
	Amount    int64  `json:"amount"`
	EntryDate string `json:"entry_date"`
	Notes     string `json:"notes"`
}

type summaryOut struct {
	TotalIncome  int64 `json:"total_income"`
	TotalExpense int64 `json:"total_expense"`
	Balance      int64 `json:"balance"`
}

const entryCols = `id, entry_type, category, amount, to_char(entry_date, 'YYYY-MM-DD'), notes`

func readEntry(w http.ResponseWriter, r *http.Request) (entryIn, bool) {
	var in entryIn
	if !httpx.Decode(w, r, &in) {
		return in, false
	}
	in.EntryType = strings.TrimSpace(in.EntryType)
	in.Category = strings.TrimSpace(in.Category)
	in.EntryDate = strings.TrimSpace(in.EntryDate)
	in.Notes = strings.TrimSpace(in.Notes)

	if in.EntryType != "income" && in.EntryType != "expense" {
		httpx.Err(w, http.StatusUnprocessableEntity, "entry_type must be income or expense")
		return in, false
	}
	if in.Category == "" || len(in.Category) > 100 {
		httpx.Err(w, http.StatusUnprocessableEntity, "category is required (max 100 characters)")
		return in, false
	}
	if in.Amount <= 0 {
		httpx.Err(w, http.StatusUnprocessableEntity, "amount must be a positive integer")
		return in, false
	}
	if in.EntryDate == "" {
		in.EntryDate = time.Now().Format("2006-01-02")
	} else if _, err := time.Parse("2006-01-02", in.EntryDate); err != nil {
		httpx.Err(w, http.StatusUnprocessableEntity, "entry_date must be YYYY-MM-DD")
		return in, false
	}
	if len(in.Notes) > 500 {
		httpx.Err(w, http.StatusUnprocessableEntity, "notes max 500 characters")
		return in, false
	}
	return in, true
}

func scanEntry(row pgx.Row) (entry, error) {
	var e entry
	err := row.Scan(&e.ID, &e.EntryType, &e.Category, &e.Amount, &e.EntryDate, &e.Notes)
	return e, err
}

func buildDateFilter(r *http.Request, args []any) (string, []any, bool) {
	var conds []string
	if from := strings.TrimSpace(r.URL.Query().Get("from")); from != "" {
		if _, err := time.Parse("2006-01-02", from); err != nil {
			return "", nil, false
		}
		args = append(args, from)
		conds = append(conds, fmt.Sprintf("entry_date >= $%d", len(args)))
	}
	if to := strings.TrimSpace(r.URL.Query().Get("to")); to != "" {
		if _, err := time.Parse("2006-01-02", to); err != nil {
			return "", nil, false
		}
		args = append(args, to)
		conds = append(conds, fmt.Sprintf("entry_date <= $%d", len(args)))
	}
	if len(conds) == 0 {
		return "", args, true
	}
	return " AND " + strings.Join(conds, " AND "), args, true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	args := []any{auth.UserID(r.Context())}
	where := "user_id = $1"

	if t := strings.TrimSpace(r.URL.Query().Get("type")); t != "" {
		if t != "income" && t != "expense" {
			httpx.Err(w, http.StatusUnprocessableEntity, "type must be income or expense")
			return
		}
		args = append(args, t)
		where += fmt.Sprintf(" AND entry_type = $%d", len(args))
	}
	dateClause, args, ok := buildDateFilter(r, args)
	if !ok {
		httpx.Err(w, http.StatusUnprocessableEntity, "date filter must be YYYY-MM-DD")
		return
	}
	where += dateClause

	args = append(args, limit, offset)
	query := fmt.Sprintf(
		`SELECT %s FROM finance_entries WHERE %s ORDER BY entry_date DESC, id DESC LIMIT $%d OFFSET $%d`,
		entryCols, where, len(args)-1, len(args),
	)
	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[entry])
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSONList(w, items)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	args := []any{auth.UserID(r.Context())}
	dateClause, args, ok := buildDateFilter(r, args)
	if !ok {
		httpx.Err(w, http.StatusUnprocessableEntity, "date filter must be YYYY-MM-DD")
		return
	}
	var out summaryOut
	query := `SELECT
		COALESCE(SUM(CASE WHEN entry_type = 'income' THEN amount ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN entry_type = 'expense' THEN amount ELSE 0 END), 0)
		FROM finance_entries WHERE user_id = $1` + dateClause
	if err := h.DB.QueryRow(r.Context(), query, args...).Scan(&out.TotalIncome, &out.TotalExpense); err != nil {
		httpx.DBError(w, err)
		return
	}
	out.Balance = out.TotalIncome - out.TotalExpense
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	in, ok := readEntry(w, r)
	if !ok {
		return
	}
	e, err := scanEntry(h.DB.QueryRow(r.Context(),
		`INSERT INTO finance_entries (user_id, entry_type, category, amount, entry_date, notes)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+entryCols,
		auth.UserID(r.Context()), in.EntryType, in.Category, in.Amount, in.EntryDate, in.Notes))
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, e)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDParam(w, r)
	if !ok {
		return
	}
	in, ok := readEntry(w, r)
	if !ok {
		return
	}
	e, err := scanEntry(h.DB.QueryRow(r.Context(),
		`UPDATE finance_entries
		 SET entry_type=$3, category=$4, amount=$5, entry_date=$6, notes=$7, updated_at=now()
		 WHERE user_id = $1 AND id = $2 RETURNING `+entryCols,
		auth.UserID(r.Context()), id, in.EntryType, in.Category, in.Amount, in.EntryDate, in.Notes))
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, e)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDParam(w, r)
	if !ok {
		return
	}
	tag, err := h.DB.Exec(r.Context(),
		`DELETE FROM finance_entries WHERE user_id = $1 AND id = $2`,
		auth.UserID(r.Context()), id)
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

