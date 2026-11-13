package inventory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var seq atomic.Int64

// Needs a THROWAWAY database with migrations applied: the ledger is append-only,
// so test rows can't be cleaned up.
func testService(t *testing.T) *Service {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run integration tests")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return &Service{DB: pool}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// seed creates one tenant with one product and one location.
func seed(t *testing.T, s *Service) (uid, pid, lid int64) {
	t.Helper()
	ctx := context.Background()
	tag := fmt.Sprintf("%d-%d", time.Now().UnixNano(), seq.Add(1))
	must(t, s.DB.QueryRow(ctx, `INSERT INTO users (name,email,password_hash) VALUES ('t',$1,'x') RETURNING id`, tag+"@test.local").Scan(&uid))
	must(t, s.DB.QueryRow(ctx, `INSERT INTO products (user_id,name,sku) VALUES ($1,'p',$2) RETURNING id`, uid, tag).Scan(&pid))
	must(t, s.DB.QueryRow(ctx, `INSERT INTO locations (user_id,name) VALUES ($1,'l') RETURNING id`, uid).Scan(&lid))
	return
}

func move(s *Service, uid int64, typ, key string, pid, lid, q int64) (int64, bool, error) {
	m := MovementIn{Type: typ, IdempotencyKey: key, Items: []ItemIn{{ProductID: pid, LocationID: lid, Quantity: q}}}
	if err := m.normalize(); err != nil {
		return 0, false, err
	}
	return s.Post(context.Background(), uid, m)
}

func qty(t *testing.T, s *Service, uid, pid, lid int64) (q int64) {
	t.Helper()
	must(t, s.DB.QueryRow(context.Background(),
		`SELECT coalesce(sum(quantity),0)::bigint FROM inventory_balances WHERE user_id=$1 AND product_id=$2 AND location_id=$3`,
		uid, pid, lid).Scan(&q))
	return
}

func TestTenantIsolation(t *testing.T) {
	s := testService(t)
	_, pidA, _ := seed(t, s)
	uidB, _, lidB := seed(t, s)
	if _, _, err := move(s, uidB, "in", "", pidA, lidB, 1); err == nil {
		t.Fatal("tenant B must not be able to move tenant A's product")
	}
}

func TestConcurrentStockOutNeverGoesNegative(t *testing.T) {
	s := testService(t)
	uid, pid, lid := seed(t, s)
	if _, _, err := move(s, uid, "in", "", pid, lid, 10); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var ok, short atomic.Int32
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := move(s, uid, "out", "", pid, lid, 1)
			switch {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, ErrInsufficient):
				short.Add(1)
			default:
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 10 || short.Load() != 15 || qty(t, s, uid, pid, lid) != 0 {
		t.Fatalf("ok=%d short=%d qty=%d, want 10/15/0", ok.Load(), short.Load(), qty(t, s, uid, pid, lid))
	}
}

func TestIdempotentReplay(t *testing.T) {
	s := testService(t)
	uid, pid, lid := seed(t, s)
	_, replayed, err := move(s, uid, "in", "k1", pid, lid, 5)
	if err != nil || replayed {
		t.Fatalf("first call: replayed=%v err=%v", replayed, err)
	}
	if _, replayed, err = move(s, uid, "in", "k1", pid, lid, 5); err != nil || !replayed {
		t.Fatalf("second call should replay: replayed=%v err=%v", replayed, err)
	}
	if q := qty(t, s, uid, pid, lid); q != 5 {
		t.Fatalf("qty=%d, want 5", q)
	}
}

func TestOpnameRecordsOnlyTheDifference(t *testing.T) {
	s := testService(t)
	uid, pid, lid := seed(t, s)
	if _, _, err := move(s, uid, "in", "", pid, lid, 10); err != nil {
		t.Fatal(err)
	}
	in := OpnameIn{LocationID: lid, Counts: []CountIn{{ProductID: pid, Counted: 7}}}
	if _, changes, _, err := s.Opname(context.Background(), uid, in); err != nil || changes != 1 {
		t.Fatalf("changes=%d err=%v", changes, err)
	}
	if q := qty(t, s, uid, pid, lid); q != 7 {
		t.Fatalf("qty=%d, want 7", q)
	}
	if _, changes, _, err := s.Opname(context.Background(), uid, in); err != nil || changes != 0 {
		t.Fatalf("a matching count must change nothing: changes=%d err=%v", changes, err)
	}
}

func TestOpnameKeepsMovementsMadeAfterTheCount(t *testing.T) {
	s := testService(t)
	uid, pid, lid := seed(t, s)
	if _, _, err := move(s, uid, "in", "", pid, lid, 10); err != nil {
		t.Fatal(err)
	}
	observed := time.Now()
	time.Sleep(50 * time.Millisecond)
	if _, _, err := move(s, uid, "out", "", pid, lid, 3); err != nil { // sold after the count was taken
		t.Fatal(err)
	}
	in := OpnameIn{LocationID: lid, ObservedAt: &observed, Counts: []CountIn{{ProductID: pid, Counted: 8}}}
	if _, changes, _, err := s.Opname(context.Background(), uid, in); err != nil || changes != 1 {
		t.Fatalf("changes=%d err=%v", changes, err)
	}
	// counted 8 while the server said 10 (2 missing); the later sale of 3 must survive: 10-3-2 = 5
	if q := qty(t, s, uid, pid, lid); q != 5 {
		t.Fatalf("qty=%d, want 5", q)
	}
}
