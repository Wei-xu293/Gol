package bench

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-xu293/gol/model"
	_ "modernc.org/sqlite"
)

func Run() error {
	db, err := sql.Open("sqlite", "gol_bench.db")
	if err != nil {
		return err
	}
	defer db.Close()
	return nil
}

func RancCurrent(width uint32, height uint32) ([]uint8, int) {
	cnt := 0
	curr := make([]uint8, width*height)
	for i := range curr {
		curr[i] = uint8(rand.UintN(2))
		cnt += int(curr[i])
	}
	return curr, cnt
}

func RandUniverse(size uint32, rule uint32) model.Universe {
	curr, cnt := RancCurrent(size, size)
	return model.Universe{
		Width:     size,
		Height:    size,
		Current:   curr,
		Next:      make([]uint8, size*size),
		LiveCount: cnt,
		Rule:      rule,
	}
}

func ConfigFromUniv(u *model.Universe) model.Config {
	return model.Config{
		Width:        u.Width,
		Height:       u.Height,
		Rule:         u.Rule,
		Initial_hash: u.CurrentHash(),
	}
}

func setupFileDB(b *testing.B) *sql.DB {
	dbFile := filepath.Join(b.TempDir(), "bench.db")

	conn, err := sql.Open("sqlite", dbFile)
	if err != nil {
		b.Fatalf("failed to open db: %v", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	return conn
}

func BenchmarkInserts(b *testing.B) {
	univs := make([]model.Universe, 1000)
	size, rule := uint32(32), uint32(0x1808)
	for i := range univs {
		univs[i] = RandUniverse(size, rule)
	}

	for b.Loop() {

	}
}

func seedMassiveData(db *sql.DB) error {
	start := time.Now()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO ticks (run_id, frame, live_count, hash)
			VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	fmt.Printf("Seeded rows in %v\n", time.Since(start))
	return nil
}
