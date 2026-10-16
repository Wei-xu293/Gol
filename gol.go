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
		`INSERT INTO saves (name, run_id, frame, current) VALUES (?, ?, ?, ?)`,
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

func loadUniverse(loadName string) {

}

func main() {
	u := initGrid()
	u.countLiveCells()

	saveCmd := flag.NewFlagSet("save", flag.ExitOnError)
	saveName := saveCmd.String("name", "", "name")

	loadCmd := flag.NewFlagSet("load", flag.ExitOnError)
	loadName := loadCmd.String("name", "", "name")
	var enableSave bool
	loadCmd.BoolVar(&enableSave, "save-after", false, "save after load")
	loadCmd.BoolVar(&enableSave, "sa", false, "shorthand for save-after")
	flag.Parse()

	if len(os.Args) > 2 {
		switch os.Args[1] {
		case "load":
			loadCmd.Parse(os.Args[2:])
			loadUniverse(*loadName)
		default:
			break
		}
	}

	fmt.Print("\033[?25l")       // hide cursor
	defer fmt.Print("\033[?25h") // show cursor when done
	ticks := 0
	ticker := time.NewTicker(67 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

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

	var gameID int64
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
	).Scan(&gameID)

	if errors.Is(err, sql.ErrNoRows) {
		err = db.QueryRowContext(ctx,
			`SELECT id FROM configs WHERE rule = ? AND initial_hash = ?`,
			u.Rule, u.CurrentHash()).Scan(&gameID)
	}
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Println("Game:", gameID)

	var runID uint32
	err = db.QueryRowContext(ctx,
		`INSERT INTO runs (config_id, start_time) VALUES (?, ?) RETURNING ID`,
		gameID, time.Now()).Scan(&runID)

	cnts := make([]Tick, 0)

	first := true
	for {
		select {
		case <-timeout:
			fmt.Println("Done!")
			fmt.Printf("Game over after %d ticks\n", ticks)

			fmt.Printf("%d ticks\n", len(cnts))
			err := insertTicks(ctx, db, runID, cnts)
			if err != nil {
				fmt.Printf("Failed to insert %d ticks", ticks)
			}

			if len(os.Args) > 2 {
				switch os.Args[1] {
				case "save":
					saveCmd.Parse(os.Args[2:])
					Save(ctx, db, runID, &u, saveCmd, saveName, ticks)
				default:
					break
				}
			}
			return
		case interrupt := <-c:
			fmt.Println("Got signal:", interrupt)
			fmt.Printf("Game over after %d ticks\n", ticks)

			fmt.Printf("%d ticks\n", len(cnts))
			err := insertTicks(ctx, db, runID, cnts)
			if err != nil {
				fmt.Printf("Failed to insert %d ticks", ticks)
			}

			if len(os.Args) > 2 {
				switch os.Args[1] {
				case "save":
					saveCmd.Parse(os.Args[2:])
					Save(ctx, db, runID, &u, saveCmd, saveName, ticks)
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
