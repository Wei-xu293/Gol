package store

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"

	"github.com/Wei-xu293/gol/model"
)

func createConfigsTable(ctx context.Context, db *sql.DB) {
	_, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			width INTEGER,
			height INTEGER,
			rule INTEGER,
			initial_hash INTEGER,

			UNIQUE (rule, initial_hash)
		)`,
	)
	if err != nil {
		log.Fatal(err)
	}
}

func createRunsTable(ctx context.Context, db *sql.DB) {
	_, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			config_id INTEGER,
			start_time DATETIME,
			end_reason TEXT,

			FOREIGN KEY (config_id) REFERENCES configs(id)
		)`,
	)
	if err != nil {
		log.Fatal(err)
	}
}

func createTicksTable(ctx context.Context, db *sql.DB) {
	_, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS ticks (
			run_id INTEGER,
			frame INTEGER,
			live_count INTEGER,
			hash INTEGER,

			PRIMARY KEY (run_id, frame),
			FOREIGN KEY (run_id) REFERENCES runs(id)
		) WITHOUT ROWID`,
	)
	if err != nil {
		log.Fatal(err)
	}
}

func createSavesTable(ctx context.Context, db *sql.DB) {
	_, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS saves (
			name TEXT PRIMARY KEY,
			run_id INTEGER,
			frame INTEGER,
			current BLOB,

			FOREIGN KEY (run_id) REFERENCES runs(id)
		)`,
	)
	if err != nil {
		log.Fatal(err)
	}
}

func InsertTicks(ctx context.Context, db *sql.DB, runID int, ticks []model.Tick) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, errPrep := tx.Prepare(`INSERT INTO ticks (run_id, frame, live_count, hash)
			VALUES (?, ?, ?, ?)`)
	if errPrep != nil {
		return errPrep
	}
	defer stmt.Close()

	for _, t := range ticks {
		if _, execErr := stmt.Exec(runID, t.Frame, t.LiveCount, t.Hash); execErr != nil {
			return execErr
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func Save(ctx context.Context, db *sql.DB, runID int, u *model.Universe,
	saveCmd *flag.FlagSet, saveName *string, frameTick int) {
	if saveCmd != nil {
		saveUniverse(ctx, db, runID, u, *saveName, frameTick)
	}
}

func saveUniverse(ctx context.Context, db *sql.DB, runID int,
	u *model.Universe, saveName string, frameTick int) {
	result, err := db.ExecContext(ctx,
		`INSERT INTO saves (name, run_id, frame, current) VALUES (?, ?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET run_id = excluded.run_id, frame = excluded.frame, current = excluded.current;`,
		saveName, runID, frameTick, u.Current)

	if err != nil {
		log.Fatal(err)
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 1 {
		log.Printf("Successfully saved a run: %s", saveName)
	}
}

func LoadUniverse(loadName string, db *sql.DB, ctx context.Context) (model.Universe, model.Config, int, error) {
	u := model.Universe{}
	var (
		cfg   model.Config
		frame int
	)
	err := db.QueryRowContext(ctx,
		`SELECT s.frame, s.current, c.id, c.width, c.height, c.rule, c.initial_hash
		FROM saves s JOIN runs r ON s.run_id = r.id JOIN configs c ON r.config_id = c.id
		WHERE s.name = ?;`,
		loadName).Scan(&frame, &u.Current, &cfg.Id, &cfg.Width, &cfg.Height, &cfg.Rule, &cfg.Initial_hash)
	if err != nil {
		return u, cfg, frame, err
	}
	u.Width, u.Height, u.Rule = cfg.Width, cfg.Height, cfg.Rule
	u.Next = make([]uint8, u.Width*u.Height)
	return u, cfg, frame, err
}

func List(ctx context.Context, db *sql.DB) {
	fmt.Printf("%-10s %8s %8s\n", "Name", "Run ID", "Frame")
	rows, err := db.QueryContext(ctx, "SELECT name, run_id, frame FROM saves;")
	if err != nil {
		log.Print(err)
		return
	}
	var (
		name  string
		runID int
		frame int
	)
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&name, &runID, &frame)
		if err != nil {
			log.Print(err)
			return
		}
		fmt.Printf("%-10s %8d %8d\n", name, runID, frame)
	}
	if err = rows.Err(); err != nil {
		log.Println(err)
	}
}

func DeleteSaveByName(ctx context.Context, db *sql.DB, name string) error {
	res, err := db.ExecContext(ctx, "DELETE FROM saves WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	log.Printf("Rows affected %d\n", rows)
	return nil
}

func Stats(ctx context.Context, db *sql.DB) {
	err := mostGrowthDieOff(ctx, db)
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println()
	err = ticksData(ctx, db)
	if err != nil {
		log.Println(err)
		return
	}
}

func ticksData(ctx context.Context, db *sql.DB) error {
	var (
		total_frames int
		max_alive    int
		avg_alive    float32
	)
	err := db.QueryRowContext(ctx,
		`select
    count(*) as total_frames,
		coalesce(max(live_count), 0) as max_alive,
		coalesce(round(avg(live_count), 1), 0.0) as avg_alive
	from ticks`).Scan(&total_frames, &max_alive, &avg_alive)

	if err != nil {
		return err
	}

	fmt.Printf("%-10s %10s %10s\n", "Total frames", "Max alive", "Avg alive")
	fmt.Printf("%-10d %10d %10.2f\n", total_frames, max_alive, avg_alive)
	return err
}

func mostGrowthDieOff(ctx context.Context, db *sql.DB) error {
	fmt.Println("Most growth and die-off per run")
	fmt.Printf("%-5s %7s %7s\n", "Run ID", "Growth", "Die-off")
	rows, err := db.QueryContext(ctx,
		`WITH diff AS (
			SELECT run_id, live_count - lag(live_count) OVER win AS growth
			FROM ticks
			WINDOW win AS (PARTITION BY run_id ORDER BY frame)
		)
		SELECT diff.run_id, coalesce(max(diff.growth),0) AS GROWTH, coalesce(min(diff.growth),0) AS DIE_OFF
		FROM diff
		GROUP BY diff.run_id;`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var (
		runID  int
		growth int
		dieOff int
	)
	for rows.Next() {
		err := rows.Scan(&runID, &growth, &dieOff)
		if err != nil {
			return err
		}
		fmt.Printf("%-5d %7d %7d\n", runID, growth, dieOff)
	}
	if err = rows.Err(); err != nil {
		log.Println(err)
	}
	return err
}

func UpdateRunById(ctx context.Context, db *sql.DB, runID int, endReason string) error {
	res, err := db.ExecContext(ctx,
		`UPDATE runs
		 SET end_reason = ?
	  	 WHERE id = ?;`, endReason, runID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("update run by id: %d rows affected", rows)
	}
	return nil
}
