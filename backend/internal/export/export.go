// backend/internal/export/export.go
package export

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"

	"ims/internal/auth"
	"ims/internal/httpx"
)

// Hard limits keep memory and response time bounded (lightweight + safe).
const (
	maxBalanceRows = 20_000
	maxHistoryRows = 50_000
	defaultHistory = 90 * 24 * time.Hour // last 90 days when from/to omitted
)

type Handler struct {
	DB *pgxpool.Pool
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/export/inventory", h.inventory)
}

// --- column maps (easy to extend later, e.g. price) ---

type col struct {
	Header string
	Width  float64
}

var summaryCols = []col{
	{"Nama Produk", 28},
	{"Kode/SKU", 16},
	{"Stok", 10},
	{"Lokasi", 20},
	// later: {"Harga", 12},
}

var historyCols = []col{
	{"Nama Produk", 28},
	{"Kode/SKU", 16},
	{"Keluar/Masuk", 14},
	{"Jumlah", 10},
	{"Tanggal", 20},
	{"Lokasi", 20},
}

var financeCols = []col{
	{"Tanggal", 14},
	{"Jenis", 14},
	{"Kategori", 22},
	{"Keterangan", 34},
	{"Nominal", 18},
}

// --- handler ---

func (h *Handler) inventory(w http.ResponseWriter, r *http.Request) {
	uid := auth.UserID(r.Context())
	from, to, err := parseRange(r)
	if err != nil {
		httpx.Err(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	balances, err := h.loadBalances(r, uid)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	history, err := h.loadHistory(r, uid, from, to)
	if err != nil {
		httpx.DBError(w, err)
		return
	}
	finRows, err := h.loadFinance(r, uid, from, to)
	if err != nil {
		httpx.DBError(w, err)
		return
	}

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	downloadedAt := time.Now()
	periodLabel := formatPeriod(from, to)

	if err := writeSummarySheet(f, balances, periodLabel, downloadedAt); err != nil {
		httpx.Err(w, http.StatusInternalServerError, "failed to build workbook")
		return
	}
	if err := writeHistorySheet(f, history, periodLabel, downloadedAt); err != nil {
		httpx.Err(w, http.StatusInternalServerError, "failed to build workbook")
		return
	}
	if err := writeFinanceSheet(f, finRows, periodLabel, downloadedAt); err != nil {
		httpx.Err(w, http.StatusInternalServerError, "failed to build workbook")
		return
	}
	// remove default "Sheet1" if it still exists after we renamed/created ours
	_ = f.DeleteSheet("Sheet1")

	filename := fmt.Sprintf("inventory-%s.xlsx", downloadedAt.Format("20060102-1504"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	if err := f.Write(w); err != nil {
		// client may have disconnected; nothing useful to return
		return
	}
}

// --- data loading ---

type balanceRow struct {
	Name     string
	SKU      string
	Quantity int64
	Location string
}

func (h *Handler) loadBalances(r *http.Request, uid int64) ([]balanceRow, error) {
	rows, err := h.DB.Query(r.Context(),
		`SELECT p.name, p.sku, b.quantity, l.name
		 FROM inventory_balances b
		 JOIN products p  ON p.user_id = b.user_id AND p.id = b.product_id
		 JOIN locations l ON l.user_id = b.user_id AND l.id = b.location_id
		 WHERE b.user_id = $1
		 ORDER BY p.name, l.name
		 LIMIT $2`,
		uid, maxBalanceRows)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[balanceRow])
}

type historyRow struct {
	Name      string
	SKU       string
	Direction string // "in" | "out"
	Quantity  int64
	CreatedAt time.Time
	Location  string
}

func (h *Handler) loadHistory(r *http.Request, uid int64, from, to time.Time) ([]historyRow, error) {
	rows, err := h.DB.Query(r.Context(),
		`SELECT p.name, p.sku, i.direction, i.quantity, m.created_at, l.name
		 FROM stock_movement_items i
		 JOIN stock_movements m ON m.user_id = i.user_id AND m.id = i.stock_movement_id
		 JOIN products p        ON p.user_id = i.user_id AND p.id = i.product_id
		 JOIN locations l       ON l.user_id = i.user_id AND l.id = i.location_id
		 WHERE i.user_id = $1
		   AND m.created_at >= $2
		   AND m.created_at <  $3
		 ORDER BY m.created_at DESC, i.id DESC
		 LIMIT $4`,
		uid, from, to, maxHistoryRows)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[historyRow])
}

type financeRow struct {
	EntryDate string
	EntryType string
	Category  string
	Notes     string
	Amount    int64
}

func (h *Handler) loadFinance(r *http.Request, uid int64, from, to time.Time) ([]financeRow, error) {
	rows, err := h.DB.Query(r.Context(),
		`SELECT to_char(entry_date, 'YYYY-MM-DD'), entry_type, category, notes, amount
		 FROM finance_entries
		 WHERE user_id = $1
		   AND entry_date >= $2::date
		   AND entry_date <  $3::date
		 ORDER BY entry_date DESC, id DESC
		 LIMIT $4`,
		uid, from.Format("2006-01-02"), to.Format("2006-01-02"), maxHistoryRows)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[financeRow])
}

// --- sheet writers ---

func writeSummarySheet(f *excelize.File, rows []balanceRow, period string, at time.Time) error {
	const name = "Ringkasan Stok"
	idx, err := f.NewSheet(name)
	if err != nil {
		return err
	}
	f.SetActiveSheet(idx)

	_ = f.SetCellValue(name, "A1", "Export Inventory — Ringkasan Stok")
	_ = f.SetCellValue(name, "A2", "Periode: "+period)
	_ = f.SetCellValue(name, "A3", "Diunduh: "+at.Format("02 Jan 2006 15:04"))

	headerRow := 5
	for i, c := range summaryCols {
		cell, _ := excelize.CoordinatesToCellName(i+1, headerRow)
		_ = f.SetCellValue(name, cell, c.Header)
		colName, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(name, colName, colName, c.Width)
	}

	for i, row := range rows {
		r := headerRow + 1 + i
		_ = f.SetCellValue(name, cell(1, r), row.Name)
		_ = f.SetCellValue(name, cell(2, r), row.SKU)
		_ = f.SetCellValue(name, cell(3, r), row.Quantity)
		_ = f.SetCellValue(name, cell(4, r), row.Location)
	}

	if len(rows) >= maxBalanceRows {
		note := headerRow + 1 + len(rows)
		_ = f.SetCellValue(name, cell(1, note), fmt.Sprintf("(data dibatasi %d baris)", maxBalanceRows))
	}
	return nil
}

func writeHistorySheet(f *excelize.File, rows []historyRow, period string, at time.Time) error {
	const name = "Riwayat"
	if _, err := f.NewSheet(name); err != nil {
		return err
	}

	_ = f.SetCellValue(name, "A1", "Export Inventory — Riwayat Keluar/Masuk")
	_ = f.SetCellValue(name, "A2", "Periode: "+period)
	_ = f.SetCellValue(name, "A3", "Diunduh: "+at.Format("02 Jan 2006 15:04"))

	headerRow := 5
	for i, c := range historyCols {
		cell, _ := excelize.CoordinatesToCellName(i+1, headerRow)
		_ = f.SetCellValue(name, cell, c.Header)
		colName, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(name, colName, colName, c.Width)
	}

	for i, row := range rows {
		r := headerRow + 1 + i
		dir := "Masuk"
		if row.Direction == "out" {
			dir = "Keluar"
		}
		_ = f.SetCellValue(name, cell(1, r), row.Name)
		_ = f.SetCellValue(name, cell(2, r), row.SKU)
		_ = f.SetCellValue(name, cell(3, r), dir)
		_ = f.SetCellValue(name, cell(4, r), row.Quantity)
		_ = f.SetCellValue(name, cell(5, r), row.CreatedAt.Format("02 Jan 2006 15:04"))
		_ = f.SetCellValue(name, cell(6, r), row.Location)
	}

	if len(rows) >= maxHistoryRows {
		note := headerRow + 1 + len(rows)
		_ = f.SetCellValue(name, cell(1, note), fmt.Sprintf("(data dibatasi %d baris)", maxHistoryRows))
	}
	return nil
}

func writeFinanceSheet(f *excelize.File, rows []financeRow, period string, at time.Time) error {
	const name = "Keuangan"
	if _, err := f.NewSheet(name); err != nil {
		return err
	}

	_ = f.SetCellValue(name, "A1", "Export Inventory — Catatan Keuangan")
	_ = f.SetCellValue(name, "A2", "Periode: "+period)
	_ = f.SetCellValue(name, "A3", "Diunduh: "+at.Format("02 Jan 2006 15:04"))

	var totalIncome, totalExpense int64
	for _, row := range rows {
		if row.EntryType == "income" {
			totalIncome += row.Amount
		} else {
			totalExpense += row.Amount
		}
	}
	_ = f.SetCellValue(name, "A5", "Total Pemasukan")
	_ = f.SetCellValue(name, "B5", totalIncome)
	_ = f.SetCellValue(name, "A6", "Total Pengeluaran")
	_ = f.SetCellValue(name, "B6", totalExpense)
	_ = f.SetCellValue(name, "A7", "Saldo Bersih")
	_ = f.SetCellValue(name, "B7", totalIncome-totalExpense)

	headerRow := 9
	for i, c := range financeCols {
		_ = f.SetCellValue(name, cell(i+1, headerRow), c.Header)
		colName, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(name, colName, colName, c.Width)
	}

	for i, row := range rows {
		r := headerRow + 1 + i
		jenis := "Pemasukan"
		if row.EntryType == "expense" {
			jenis = "Pengeluaran"
		}
		_ = f.SetCellValue(name, cell(1, r), row.EntryDate)
		_ = f.SetCellValue(name, cell(2, r), jenis)
		_ = f.SetCellValue(name, cell(3, r), row.Category)
		_ = f.SetCellValue(name, cell(4, r), row.Notes)
		_ = f.SetCellValue(name, cell(5, r), row.Amount)
	}

	if len(rows) >= maxHistoryRows {
		note := headerRow + 1 + len(rows)
		_ = f.SetCellValue(name, cell(1, note), fmt.Sprintf("(data dibatasi %d baris)", maxHistoryRows))
	}
	return nil
}

// --- helpers ---

func cell(col, row int) string {
	s, _ := excelize.CoordinatesToCellName(col, row)
	return s
}

// parseRange reads optional ?from=YYYY-MM-DD&to=YYYY-MM-DD.
// to is exclusive end-of-day (start of next day) so a single day is inclusive.
func parseRange(r *http.Request) (from, to time.Time, err error) {
	now := time.Now()
	to = now.Add(24 * time.Hour).Truncate(24 * time.Hour) // start of tomorrow (UTC-ish; fine for local day filter)
	from = to.Add(-defaultHistory)

	if s := r.URL.Query().Get("from"); s != "" {
		from, err = time.ParseInLocation("2006-01-02", s, time.Local)
		if err != nil {
			return from, to, fmt.Errorf("from must be YYYY-MM-DD")
		}
	}
	if s := r.URL.Query().Get("to"); s != "" {
		var t time.Time
		t, err = time.ParseInLocation("2006-01-02", s, time.Local)
		if err != nil {
			return from, to, fmt.Errorf("to must be YYYY-MM-DD")
		}
		// inclusive end date → exclusive next midnight
		to = t.Add(24 * time.Hour)
	}
	if !from.Before(to) {
		return from, to, fmt.Errorf("from must be before to")
	}
	// soft guard against huge ranges (optional)
	if to.Sub(from) > 366*24*time.Hour {
		return from, to, fmt.Errorf("range must not exceed 366 days")
	}
	return from, to, nil
}

func formatPeriod(from, to time.Time) string {
	// to is exclusive; show last inclusive day
	end := to.Add(-time.Second)
	return fmt.Sprintf("%s s/d %s",
		from.Format("02 Jan 2006"),
		end.Format("02 Jan 2006"))
}

// silence unused import if build tags change
var _ = strconv.Itoa