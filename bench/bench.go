package bench

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
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

func RandUniverse(size uint32) model.Universe {
	curr, cnt := RancCurrent(size, size)
	return model.Universe{
		Width:     size,
		Height:    size,
		Current:   curr,
		Next:      make([]uint8, size*size),
		LiveCount: cnt,
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
