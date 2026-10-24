package bench

import (
	"database/sql"
	"fmt"
	"time"

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
