package bench

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-xu293/gol/model"
	"github.com/Wei-xu293/gol/store"
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

func genTicks(u *model.Universe) []model.Tick {
	ticks := make([]model.Tick, 1000)
	for j := range len(ticks) {
		u.NextState()
		ticks[j] = model.Tick{
			Frame:     uint32(j),
			LiveCount: uint32(u.LiveCount),
			Hash:      u.CurrentHash(),
		}
	}
	return ticks
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

func BenchmarkInsertsTicks(b *testing.B) {
	univs := make([]model.Universe, 1000)
	size, rule := uint32(32), uint32(0x1808)
	for i := range univs {
		univs[i] = RandUniverse(size, rule)
	}
	var allTicks [][]model.Tick
	for i := range univs {
		allTicks = append(allTicks, genTicks(&univs[i]))
	}
	var tests = []struct {
		name string
	}{
		{name: "Default"},
		{name: "WAL"},
		{name: "WAL_normal"},
	}
	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				db := setupFileDB(b)
				defer db.Close()
				ctx := context.Background()
				err := store.InitDB(ctx, db)
				if err != nil {
					b.Fatalf("failed to create tables: %v", err)
				}
				switch tc.name {
				case "WAL":
					_, err := db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`)
					if err != nil {
						b.Fatalf("failed pragma: %v", err)
					}
					b.ResetTimer()
				case "WAL_normal":
					_, err := db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`)
					if err != nil {
						b.Fatalf("failed pragma: %v", err)
					}
					var syncFlag string
					err = db.QueryRowContext(ctx, "PRAGMA synchronous;").Scan(&syncFlag)
					if err != nil {
						b.Fatalf("failed pragma q: %v", err)
					}
					_, err = db.ExecContext(ctx, `PRAGMA synchronous=NORMAL;`)
					if err != nil {
						b.Fatalf("failed pragma: %v", err)
					}
					b.ResetTimer()
				default:
					b.ResetTimer()
				}
				for i := range univs {
					configID, err := store.InsertConfig(ctx, db, &univs[i])

					if errors.Is(err, sql.ErrNoRows) {
						configID, err = store.GetUniverseConfigID(ctx, db, &univs[i])
					}
					if err != nil {
						b.Fatalf("failed to insert/retrieve configID: %v", err)
					}
					runID, err := store.InsertRun(ctx, db, configID)
					if err != nil {
						b.Fatalf("failed to insert run: %v", err)
					}
					if err := store.InsertTicks(ctx, db, runID, allTicks[i]); err != nil {
						b.Fatalf("failed to insert ticks: %v", err)
					}
				}
			}

		})
	}
}
