package inventory

import (
	"context"
	"errors"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5"

	"ims/internal/auth"
	"ims/internal/httpx"
)

type CountIn struct {
	ProductID int64 `json:"product_id"`
	Counted   int64 `json:"counted"`
}

type OpnameIn struct {
	LocationID     int64     `json:"location_id"`
	Notes          string    `json:"notes"`
	IdempotencyKey string    `json:"idempotency_key"`
	Counts         []CountIn `json:"counts"`
}

// Opname reconciles physical counts against the CURRENT balance inside one transaction.
// Each balance row is locked, the difference is recorded as an adjustment in the ledger,
// and nothing is overwritten silently.
func (s *Service) Opname(ctx context.Context, uid int64, in OpnameIn) (id int64, changes int, replayed bool, err error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return 0, 0, false, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO stock_movements (user_id, movement_type, reference_number, notes, idempotency_key)
		 VALUES ($1, 'adjustment', 'opname', nullif($2,''), nullif($3,''))
		 ON CONFLICT (user_id, idempotency_key) DO NOTHING RETURNING id`,
		uid, in.Notes, in.IdempotencyKey).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id FROM stock_movements WHERE user_id = $1 AND idempotency_key = $2`,
			uid, in.IdempotencyKey).Scan(&id)
		return id, 0, true, err
	}
	if err != nil {
		return 0, 0, false, err
	}

	sort.Slice(in.Counts, func(a, b int) bool { return in.Counts[a].ProductID < in.Counts[b].ProductID })
	for _, c := range in.Counts {
		if _, err = tx.Exec(ctx,
			`INSERT INTO inventory_balances (user_id, product_id, location_id, quantity) VALUES ($1,$2,$3,0)
			 ON CONFLICT DO NOTHING`, uid, c.ProductID, in.LocationID); err != nil {
			return 0, 0, false, err
		}
		var cur int64
		if err = tx.QueryRow(ctx,
			`SELECT quantity FROM inventory_balances WHERE user_id = $1 AND product_id = $2 AND location_id = $3 FOR UPDATE`,
			uid, c.ProductID, in.LocationID).Scan(&cur); err != nil {
			return 0, 0, false, err
		}
		delta := c.Counted - cur
		if delta == 0 {
			continue
		}
		dir, qty := "in", delta
		if delta < 0 {
			dir, qty = "out", -delta
		}
		if _, err = tx.Exec(ctx,
			`INSERT INTO stock_movement_items (user_id, stock_movement_id, product_id, location_id, quantity, direction)
			 VALUES ($1,$2,$3,$4,$5,$6)`, uid, id, c.ProductID, in.LocationID, qty, dir); err != nil {
			return 0, 0, false, err
		}
		if _, err = tx.Exec(ctx,
			`UPDATE inventory_balances SET quantity = $4, updated_at = now()
			 WHERE user_id = $1 AND product_id = $2 AND location_id = $3`,
			uid, c.ProductID, in.LocationID, c.Counted); err != nil {
			return 0, 0, false, err
		}
		changes++
	}
	return id, changes, false, tx.Commit(ctx)
}

func (s *Service) postOpname(w http.ResponseWriter, r *http.Request) {
	var in OpnameIn
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.LocationID < 1 || len(in.Counts) == 0 || len(in.Counts) > 500 || len(in.IdempotencyKey) > 100 || len(in.Notes) > 1000 {
		httpx.Err(w, http.StatusUnprocessableEntity, "location_id and 1 to 500 counts are required")
		return
	}
	for _, c := range in.Counts {
		if c.Counted < 0 {
			httpx.Err(w, http.StatusUnprocessableEntity, "counted must be zero or more")
			return
		}
	}
	id, changes, replayed, err := s.Opname(r.Context(), auth.UserID(r.Context()), in)
	switch {
	case err != nil:
		httpx.DBError(w, err)
	case replayed:
		httpx.JSON(w, http.StatusOK, map[string]any{"id": id, "replayed": true, "changes": 0})
	default:
		httpx.JSON(w, http.StatusCreated, map[string]any{"id": id, "changes": changes})
	}
}
