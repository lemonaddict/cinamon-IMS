package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"ims/internal/auth"
	"ims/internal/httpx"
)

var ErrInsufficient = errors.New("insufficient stock")

type Service struct{ DB *pgxpool.Pool }

func (s *Service) Routes(r chi.Router) {
	r.Post("/stock/movements", s.postMovement)
	r.Get("/stock/movements", s.listMovements)
	r.Get("/stock/balances", s.listBalances)
	r.Get("/dashboard", s.dashboard)
	r.Post("/stock/opname", s.postOpname)
}

type ItemIn struct {
	ProductID  int64  `json:"product_id"`
	LocationID int64  `json:"location_id"`
	Quantity   int64  `json:"quantity"`
	Direction  string `json:"direction"` // only used by adjustments
}

type MovementIn struct {
	Type           string   `json:"type"` // in | out | adjustment
	Reference      string   `json:"reference_number"`
	Notes          string   `json:"notes"`
	IdempotencyKey string   `json:"idempotency_key"`
	Items          []ItemIn `json:"items"`
}

func (m *MovementIn) normalize() error {
	if len(m.Items) == 0 || len(m.Items) > 100 {
		return errors.New("1 to 100 items are required")
	}
	if m.Type != "in" && m.Type != "out" && m.Type != "adjustment" {
		return errors.New("type must be in, out or adjustment")
	}
	if len(m.IdempotencyKey) > 100 || len(m.Reference) > 100 || len(m.Notes) > 1000 {
		return errors.New("idempotency_key/reference_number/notes too long")
	}
	for i := range m.Items {
		it := &m.Items[i]
		if it.Quantity < 1 {
			return errors.New("quantity must be a positive integer")
		}
		if m.Type != "adjustment" {
			it.Direction = m.Type
		} else if it.Direction != "in" && it.Direction != "out" {
			return errors.New("adjustment items need direction in or out")
		}
	}
	// Fixed lock order across concurrent requests prevents deadlocks.
	sort.Slice(m.Items, func(a, b int) bool {
		x, y := m.Items[a], m.Items[b]
		if x.ProductID != y.ProductID {
			return x.ProductID < y.ProductID
		}
		if x.LocationID != y.LocationID {
			return x.LocationID < y.LocationID
		}
		return x.Direction < y.Direction
	})
	return nil
}

// Post appends a ledger entry and updates balances in ONE transaction.
// A repeated idempotency_key returns the original movement without re-applying it.
func (s *Service) Post(ctx context.Context, uid int64, m MovementIn) (id int64, replayed bool, err error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO stock_movements (user_id, movement_type, reference_number, notes, idempotency_key)
		 VALUES ($1, $2, nullif($3,''), nullif($4,''), nullif($5,''))
		 ON CONFLICT (user_id, idempotency_key) DO NOTHING RETURNING id`,
		uid, m.Type, m.Reference, m.Notes, m.IdempotencyKey).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id FROM stock_movements WHERE user_id = $1 AND idempotency_key = $2`,
			uid, m.IdempotencyKey).Scan(&id)
		return id, true, err
	}
	if err != nil {
		return 0, false, err
	}

	for _, it := range m.Items {
		// Composite FKs reject products/locations that belong to another tenant.
		if _, err = tx.Exec(ctx,
			`INSERT INTO stock_movement_items (user_id, stock_movement_id, product_id, location_id, quantity, direction)
			 VALUES ($1,$2,$3,$4,$5,$6)`,
			uid, id, it.ProductID, it.LocationID, it.Quantity, it.Direction); err != nil {
			return 0, false, err
		}
		if it.Direction == "in" {
			_, err = tx.Exec(ctx,
				`INSERT INTO inventory_balances (user_id, product_id, location_id, quantity) VALUES ($1,$2,$3,$4)
				 ON CONFLICT (user_id, product_id, location_id)
				 DO UPDATE SET quantity = inventory_balances.quantity + EXCLUDED.quantity, updated_at = now()`,
				uid, it.ProductID, it.LocationID, it.Quantity)
		} else {
			var tag pgconn.CommandTag
			// Guarded atomic decrement: row-locked, can never go negative under concurrency.
			tag, err = tx.Exec(ctx,
				`UPDATE inventory_balances SET quantity = quantity - $4, updated_at = now()
				 WHERE user_id = $1 AND product_id = $2 AND location_id = $3 AND quantity >= $4`,
				uid, it.ProductID, it.LocationID, it.Quantity)
			if err == nil && tag.RowsAffected() == 0 {
				err = ErrInsufficient
			}
		}
		if err != nil {
			return 0, false, err
		}
		if m.Type == "in" || m.Type == "out" {
			if err = recordFinance(ctx, tx, uid, m.Type, it.ProductID, it.Quantity, m.Notes); err != nil {
				return 0, false, err
			}
		}
	}
	return id, false, tx.Commit(ctx)
}

// recordFinance logs stock-in (cost_price as expense) and stock-out (selling_price as income)
// into finance_entries within the same transaction, skipping items with zero price.
func recordFinance(ctx context.Context, tx pgx.Tx, uid int64, moveType string, productID, qty int64, userNotes string) error {
	var name string
	var costPrice, sellingPrice int64
	if err := tx.QueryRow(ctx,
		`SELECT name, cost_price, selling_price FROM products WHERE user_id = $1 AND id = $2`,
		uid, productID).Scan(&name, &costPrice, &sellingPrice); err != nil {
		return err
	}
	entryType, category, unitPrice := "expense", "Pembelian Stok", costPrice
	if moveType == "out" {
		entryType, category, unitPrice = "income", "Penjualan", sellingPrice
	}
	amount := unitPrice * qty
	if amount <= 0 {
		return nil
	}
	notes := fmt.Sprintf("%s × %d", name, qty)
	if extra := strings.TrimSpace(userNotes); extra != "" {
		notes += " — " + extra
	}
	if len(notes) > 500 {
		notes = notes[:500]
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO finance_entries (user_id, entry_type, category, amount, entry_date, notes)
		 VALUES ($1, $2, $3, $4, CURRENT_DATE, $5)`,
		uid, entryType, category, amount, notes)
	return err
}

func (s *Service) postMovement(w http.ResponseWriter, r *http.Request) {
	var in MovementIn
	if !httpx.Decode(w, r, &in) {
		return
	}
	if err := in.normalize(); err != nil {
		httpx.Err(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	id, replayed, err := s.Post(r.Context(), auth.UserID(r.Context()), in)
	switch {
	case errors.Is(err, ErrInsufficient):
		httpx.Err(w, http.StatusConflict, "insufficient stock")
	case err != nil:
		httpx.DBError(w, err)
	case replayed:
		httpx.JSON(w, http.StatusOK, map[string]any{"id": id, "replayed": true})
	default:
		httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
	}
}

type movement struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Reference string          `json:"reference_number"`
	Notes     string          `json:"notes"`
	CreatedAt time.Time       `json:"created_at"`
	Items     json.RawMessage `json:"items"`
}

// One query with json_agg: no N+1.
func (s *Service) listMovements(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	rows, err := s.DB.Query(r.Context(),
		`SELECT m.id, m.movement_type, coalesce(m.reference_number,''), coalesce(m.notes,''), m.created_at,
		        coalesce(json_agg(json_build_object('product_id', i.product_id, 'location_id', i.location_id,
		                 'quantity', i.quantity, 'direction', i.direction)) FILTER (WHERE i.id IS NOT NULL), '[]')
		 FROM stock_movements m
		 LEFT JOIN stock_movement_items i ON i.user_id = m.user_id AND i.stock_movement_id = m.id
		 WHERE m.user_id = $1 GROUP BY m.id ORDER BY m.id DESC LIMIT $2 OFFSET $3`,
		auth.UserID(r.Context()), limit, offset)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[movement])
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSONList(w, items)
}

type balance struct {
	ProductID  int64  `json:"product_id"`
	SKU        string `json:"sku"`
	Product    string `json:"product"`
	LocationID int64  `json:"location_id"`
	Location   string `json:"location"`
	Quantity   int64  `json:"quantity"`
}

func (s *Service) listBalances(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpx.Page(r)
	rows, err := s.DB.Query(r.Context(),
		`SELECT b.product_id, p.sku, p.name, b.location_id, l.name, b.quantity
		 FROM inventory_balances b
		 JOIN products p  ON p.user_id = b.user_id AND p.id = b.product_id
		 JOIN locations l ON l.user_id = b.user_id AND l.id = b.location_id
		 WHERE b.user_id = $1 ORDER BY p.name, l.name LIMIT $2 OFFSET $3`,
		auth.UserID(r.Context()), limit, offset)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[balance])
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSONList(w, items)
}

func (s *Service) dashboard(w http.ResponseWriter, r *http.Request) {
	var products, locations, units int64
	err := s.DB.QueryRow(r.Context(),
		`SELECT (SELECT count(*) FROM products WHERE user_id = $1),
		        (SELECT count(*) FROM locations WHERE user_id = $1),
		        (SELECT coalesce(sum(quantity), 0)::bigint FROM inventory_balances WHERE user_id = $1)`,
		auth.UserID(r.Context())).Scan(&products, &locations, &units)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int64{"products": products, "locations": locations, "total_units": units})
}
