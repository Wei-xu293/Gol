package store

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/Wei-xu293/gol/model"
)

func createConfigsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			width INTEGER,
			height INTEGER,
			rule INTEGER,
			initial_hash INTEGER,
			initial_board BLOB,

			UNIQUE (rule, initial_hash)
		)`,
	)
	return err
}

func createRunsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			config_id INTEGER,
			start_time DATETIME,
			end_reason TEXT,

			FOREIGN KEY (config_id) REFERENCES configs(id)
		)`,
	)
	return err
}

func createTicksTable(ctx context.Context, db *sql.DB) error {
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
	return err
}

func createSavesTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS saves (
			name TEXT PRIMARY KEY,
			run_id INTEGER,
			frame INTEGER,
			current BLOB,

			FOREIGN KEY (run_id) REFERENCES runs(id)
		)`,
	)
	return err
}

func InitDB(ctx context.Context, db *sql.DB) error {
	if err := createConfigsTable(ctx, db); err != nil {
		return err
	}
	if err := createRunsTable(ctx, db); err != nil {
		return err
	}
	if err := createSavesTable(ctx, db); err != nil {
		return err
	}
	if err := createTicksTable(ctx, db); err != nil {
		return err
	}

	_, err := db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`)
	if err != nil {
		return fmt.Errorf("failed pragma: %v\n", err)
	}
	var syncFlag int
	err = db.QueryRowContext(ctx, "PRAGMA synchronous;").Scan(&syncFlag)
	if err != nil {
		return fmt.Errorf("failed pragma q: %v\n", err)
	}
	fmt.Printf("synchronous = %d\n", syncFlag)
	_, err = db.ExecContext(ctx, `PRAGMA synchronous=NORMAL;`)
	if err != nil {
		return fmt.Errorf("failed pragma: %v\n", err)
	}

	return nil
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

func GetSaves(ctx context.Context, db *sql.DB) ([]model.SaveInfo, error) {
	rows, err := db.QueryContext(ctx, "SELECT name, run_id, frame FROM saves;")
	if err != nil {
		log.Print(err)
		return nil, err
	}
	var s model.SaveInfo
	saves := make([]model.SaveInfo, 0)
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&s.Name, &s.RunID, &s.Frame)
		if err != nil {
			return nil, err
		}
		saves = append(saves, s)
	}
	if err = rows.Err(); err != nil {
		return saves, err
	}
	return saves, nil
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

func TicksData(ctx context.Context, db *sql.DB) (model.TicksInfo, error) {
	var td model.TicksInfo
	err := db.QueryRowContext(ctx,
		`select
    count(*) as total_frames,
		coalesce(max(live_count), 0) as max_alive,
		coalesce(round(avg(live_count), 1), 0.0) as avg_alive
	from ticks`).Scan(&td.TotalFrames, &td.MaxAlive, &td.AvgAlive)
	return td, err
}

func MostGrowthDieOff(ctx context.Context, db *sql.DB) ([]model.GrowthDieOff, error) {
	rows, err := db.QueryContext(ctx,
		`WITH diff AS (
			SELECT run_id, live_count - lag(live_count) OVER win AS growth
			FROM ticks
			WINDOW win AS (PARTITION BY run_id ORDER BY frame)
		)
		SELECT diff.run_id, coalesce(max(diff.growth),0) AS GROWTH, coalesce(min(diff.growth),0) AS DIE_OFF
		FROM diff
		GROUP BY diff.run_id;`)

	var gds []model.GrowthDieOff
	if err != nil {
		return gds, err
	}
	defer rows.Close()
	var gd model.GrowthDieOff
	for rows.Next() {
		err := rows.Scan(&gd.RunID, &gd.Growth, &gd.DieOff)
		if err != nil {
			return gds, err
		}
		gds = append(gds, gd)
	}
	err = rows.Err()
	return gds, err
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

// Inserts the initial universe state as a config record and returns the configID,
// whether it was newly created or already existed.
func InsertConfig(ctx context.Context, db *sql.DB, u *model.Universe) (int64, error) {

	var configID int64
	err := db.QueryRowContext(
		ctx,
		`INSERT INTO configs (width, height, rule, initial_hash, initial_board)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (rule, initial_hash) DO NOTHING
			RETURNING id`,
		u.Width,
		u.Height,
		u.Rule,
		u.CurrentHash(),
		u.Current,
	).Scan(&configID)

	return configID, err
}

// Inserts a new run record given a configID, and returns the runID.
func InsertRun(ctx context.Context, db *sql.DB, configID int64) (int, error) {
	var runID int
	err := db.QueryRowContext(ctx,
		`INSERT INTO runs (config_id, start_time) VALUES (?, ?) RETURNING ID`,
		configID, time.Now()).Scan(&runID)
	return runID, err
}

func GetUniverseConfigID(ctx context.Context, db *sql.DB, u *model.Universe) (int64, error) {
	var configID int64
	err := db.QueryRowContext(ctx,
		`SELECT id FROM configs WHERE rule = ? AND initial_hash = ?`,
		u.Rule, u.CurrentHash()).Scan(&configID)
	return configID, err
}

func InsertConfigs(ctx context.Context, db *sql.DB, cfgs []model.Config) ([]int, error) {
	tx, err := db.BeginTx(ctx, nil)
	var cfgIDs []int
	if err != nil {
		return cfgIDs, err
	}
	defer tx.Rollback()
	stmt, errPrep := tx.Prepare(`INSERT INTO configs (width, height, rule, initial_hash, initial_board)
			VALUES (?, ?, ?, ?, ?)`)
	if errPrep != nil {
		return cfgIDs, errPrep
	}
	defer stmt.Close()

	for _, c := range cfgs {
		if _, execErr := stmt.Exec(c.Width, c.Height, c.Rule, c.Initial_hash, c.Initial_board); execErr != nil {
			return cfgIDs, execErr
		}
	}
	err = tx.Commit()
	return cfgIDs, err
}
