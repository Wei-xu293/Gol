package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"os/signal"
	"time"

	_ "modernc.org/sqlite"
)

type Universe struct {
	Width     uint32
	Height    uint32
	Current   []uint8
	Next      []uint8
	Rule      uint32
	liveCount int
	b         bytes.Buffer
}

type Tick struct {
	Frame     uint32
	LiveCount uint32
	Hash      uint32
}

var dirs = [8][2]int{
	{-1, -1}, {-1, 0}, {-1, 1},
	{0, -1}, {0, 1},
	{1, -1}, {1, 0}, {1, 1},
}

func (u *Universe) NextState() {
	w, h := int(u.Width), int(u.Height)
	u.liveCount = 0

	for i := range h {
		for j := range w {
			alive := 0
			for _, d := range dirs {
				ni := (i + d[0] + h) % h
				nj := (j + d[1] + w) % w
				alive += int(u.Current[ni*w+nj])
			}

			idx := i*w + j
			shift := alive + int(u.Current[idx])*9
			u.Next[idx] = uint8((u.Rule >> shift) & 1)
			u.liveCount += int(u.Next[idx])
		}
	}
	u.Current, u.Next = u.Next, u.Current
}

// TODO: use \033[%dA passing line count (Height - i)
func (u *Universe) CurrentFrame(first bool) error {
	if !first {
		for range u.Height {
			if _, err := u.b.WriteString("\033[1A\033[2K"); err != nil {
				return err
			}
		}
	}
	for i := range u.Height {
		if _, err := u.b.WriteString("\r\033[2K"); err != nil {
			return err
		}
		for j := range u.Width {
			a := ' '
			idx := i*u.Width + j
			if u.Current[idx] == 1 {
				a = '*'
			}
			if _, err := u.b.WriteRune(a); err != nil {
				return err
			}
		}
		if _, err := u.b.WriteRune('\n'); err != nil {
			return err
		}
	}
	return nil
}

func (u *Universe) countLiveCells() {
	u.liveCount = 0
	for _, val := range u.Current {
		u.liveCount += int(val)
	}
}

func (u *Universe) LiveCount() int {
	return u.liveCount
}

func (u *Universe) CurrentHash() uint32 {
	h := fnv.New32a()
	h.Write(u.Current)
	return h.Sum32()
}

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

func insertTicks(ctx context.Context, db *sql.DB, runID uint32, ticks []Tick) error {
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

func Save(ctx context.Context, db *sql.DB, runID uint32, u *Universe,
	saveCmd *flag.FlagSet, saveName *string, frameTick int) {
	if saveCmd != nil {
		saveUniverse(ctx, db, runID, u, *saveName, frameTick)
	}
}

func saveUniverse(ctx context.Context, db *sql.DB, runID uint32,
	u *Universe, saveName string, frameTick int) {
	result, err := db.ExecContext(ctx,
		`INSERT INTO saves (name, run_id, frame, current) VALUES (?, ?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET frame = excluded.frame;`,
		saveName, runID, frameTick, u.Current)

	if err != nil {
		log.Fatal(err)
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 1 {
		log.Printf("Successfully saved a %s", saveName)
	}
}

func initGrid() Universe {
	const (
		width  = 15
		height = 21
	)
	// https://xkcd.com/2293/
	var initialGrid = []uint8{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 1, 1, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 1, 0, 1, 1, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
	u := Universe{
		Width:   width,
		Height:  height,
		Current: initialGrid,
		Next:    make([]uint8, width*height),
		Rule:    0x1808,
	}
	return u
}

func loadUniverse(loadName string, db *sql.DB, ctx context.Context) (Universe, int, error) {
	u := Universe{}
	var runID, frame int
	err := db.QueryRowContext(ctx,
		`SELECT run_id, frame,current FROM saves WHERE name = ?`,
		loadName).Scan(&runID, &frame, &u.Current)
	if err != nil {
		return u, frame, err
	}
	var configID int
	err = db.QueryRowContext(ctx,
		`SELECT config_id FROM runs WHERE id = ?`, runID).Scan(&configID)
	if err != nil {
		return u, frame, err
	}
	var width, height, rule uint32
	err = db.QueryRowContext(ctx,
		`SELECT width, height, rule FROM configs WHERE id = ?`,
		configID).Scan(&width, &height, &rule)
	if err != nil {
		return u, frame, err
	}
	u.Width, u.Height, u.Rule = width, height, rule
	u.Next = make([]uint8, width*height)
	return u, frame, err
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
		SELECT diff.run_id, max(diff.growth) AS GROWTH, min(diff.growth) AS DIE_OFF
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

func main() {
	u := initGrid()
	u.countLiveCells()

	saveCmd := flag.NewFlagSet("save", flag.ExitOnError)
	saveName := saveCmd.String("name", "", "name of the save")

	loadCmd := flag.NewFlagSet("load", flag.ExitOnError)
	loadName := loadCmd.String("name", "", "name of the save to load")
	var enableSave bool
	loadCmd.BoolVar(&enableSave, "save-after", false, "save after load")
	loadCmd.BoolVar(&enableSave, "sa", false, "shorthand for save-after")

	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)
	deleteSavedName := deleteCmd.String("name", "", "name of the save to delete")

	flag.Parse()

	ctx := context.Background()

	db, err := sql.Open("sqlite", "gol.db")
	if err != nil {
		fmt.Println(err)
	}
	defer db.Close()

	status := "up"
	if err := db.PingContext(ctx); err != nil {
		status = "down"
		return
	}
	log.Println(status)

	createConfigsTable(ctx, db)
	createRunsTable(ctx, db)
	createTicksTable(ctx, db)
	createSavesTable(ctx, db)

	fmt.Print("\033[?25l")       // hide cursor
	defer fmt.Print("\033[?25h") // show cursor when done
	ticks := 0

	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "load":
			loadCmd.Parse(os.Args[2:])
			if *loadName == "" {
				fmt.Println("Error: <name> not provided")
				loadCmd.PrintDefaults()
				return
			}
			univ, frame, err := loadUniverse(*loadName, db, ctx)
			if err != nil {
				log.Println(err)
				return
			}
			ticks += frame
			u = univ
		case "list":
			List(ctx, db)
			return
		case "stats":
			Stats(ctx, db)
			return
		case "delete":
			deleteCmd.Parse(os.Args[2:])
			if *deleteSavedName == "" {
				fmt.Println("Error: <name> not provided")
				deleteCmd.PrintDefaults()
				return
			}
			err := DeleteSaveByName(ctx, db, *deleteSavedName)
			if err != nil {
				log.Println(err)
			}
			return
		default:
			break
		}
	}

	ticker := time.NewTicker(67 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	var configID int64
	err = db.QueryRowContext(
		ctx,
		`INSERT INTO configs (width, height, rule, initial_hash)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (rule, initial_hash) DO NOTHING
		RETURNING id`,
		u.Width,
		u.Height,
		u.Rule,
		u.CurrentHash(),
	).Scan(&configID)

	if errors.Is(err, sql.ErrNoRows) {
		err = db.QueryRowContext(ctx,
			`SELECT id FROM configs WHERE rule = ? AND initial_hash = ?`,
			u.Rule, u.CurrentHash()).Scan(&configID)
	}
	if err != nil {
		log.Fatal(err)
		return
	}
	fmt.Println("Game config:", configID)

	var runID uint32
	err = db.QueryRowContext(ctx,
		`INSERT INTO runs (config_id, start_time) VALUES (?, ?) RETURNING ID`,
		configID, time.Now()).Scan(&runID)
	if err != nil {
		log.Print(err)
		return
	}

	cnts := make([]Tick, 0)

	first := true
	for {
		select {
		case <-timeout:
			fmt.Println("Done!")
			fmt.Printf("Game over after %d ticks\n", ticks)

			fmt.Printf("%d new ticks\n", len(cnts))
			err := insertTicks(ctx, db, runID, cnts)
			if err != nil {
				fmt.Printf("Failed to insert %d ticks", ticks)
				return
			}

			if len(os.Args) > 2 {
				switch os.Args[1] {
				case "save":
					saveCmd.Parse(os.Args[2:])
					Save(ctx, db, runID, &u, saveCmd, saveName, ticks)
				case "load":
					if enableSave {
						Save(ctx, db, runID, &u, loadCmd, loadName, ticks)
					}
				default:
					break
				}
			}
			return
		case interrupt := <-c:
			fmt.Println("Got signal:", interrupt)
			fmt.Printf("Game over after %d ticks\n", ticks)

			fmt.Printf("%d new ticks\n", len(cnts))
			err := insertTicks(ctx, db, runID, cnts)
			if err != nil {
				fmt.Printf("Failed to insert %d ticks", ticks)
				return
			}

			if len(os.Args) > 2 {
				switch os.Args[1] {
				case "save":
					saveCmd.Parse(os.Args[2:])
					Save(ctx, db, runID, &u, saveCmd, saveName, ticks)
				case "load":
					if enableSave {
						Save(ctx, db, runID, &u, loadCmd, loadName, ticks)
					}
				default:
					break
				}
			}
			return
		case <-ticker.C:
			if !first {
				u.NextState()
			}
			cnts = append(cnts, Tick{
				uint32(ticks),
				uint32(u.LiveCount()),
				u.CurrentHash(),
			})
			u.CurrentFrame(first)
			u.b.WriteTo(os.Stdout)
			u.b.Reset()
			first = false
			ticks++
		}
	}
}
