package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/Wei-xu293/gol/model"
	"github.com/Wei-xu293/gol/store"
	_ "modernc.org/sqlite"
)

func initGrid() model.Universe {
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
	u := model.Universe{
		Width:   width,
		Height:  height,
		Current: initialGrid,
		Next:    make([]uint8, width*height),
		Rule:    0x1808,
	}
	return u
}

// TODO: clean
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

func Stats(ctx context.Context, db *sql.DB) {
	gds, err := store.MostGrowthDieOff(ctx, db)
	if err != nil {
		log.Println(err)
		return
	}
	printGD(gds)

	fmt.Println()
	td, err := store.TicksData(ctx, db)
	if err != nil {
		log.Println(err)
		return
	}
	printTD(td)
}

func printTD(td model.TicksInfo) {
	fmt.Printf("%-10s %10s %10s\n", "Total frames", "Max alive", "Avg alive")
	fmt.Printf("%-10d %10d %10.2f\n", td.TotalFrames, td.MaxAlive, td.AvgAlive)
}

func printGD(gds []model.GrowthDieOff) {
	fmt.Println("Most growth and die-off per run")
	fmt.Printf("%-5s %7s %7s\n", "Run ID", "Growth", "Die-off")
	for _, gd := range gds {
		fmt.Printf("%-5d %7d %7d\n", gd.RunID, gd.Growth, gd.DieOff)
	}
}

func main() {
	u := initGrid()

	saveCmd := flag.NewFlagSet("save", flag.ExitOnError)
	saveName := saveCmd.String("name", "", "name of the save")

	loadCmd := flag.NewFlagSet("load", flag.ExitOnError)
	loadName := loadCmd.String("name", "", "name of the save to load")
	var enableSave bool
	loadCmd.BoolVar(&enableSave, "save-after", false, "save after load")
	loadCmd.BoolVar(&enableSave, "sa", false, "shorthand for save-after")

	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)
	deleteSavedName := deleteCmd.String("name", "", "name of the save to delete")

	cycleFlag := flag.Bool("cycle", false, "detect cycle")

	headlessFlag := flag.Bool("headless", false, "headless run")

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

	err = store.InitDB(ctx, db)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Print("\033[?25l")       // hide cursor
	defer fmt.Print("\033[?25h") // show cursor when done
	ticks := 0

	loading := false
	var config model.Config

	fmt.Printf("raw os.Args   (len %d): %q\n", len(os.Args), os.Args)
	fmt.Printf("leftover Args (len %d): %q\n", flag.NArg(), flag.Args())

	if flag.NArg() >= 1 {
		switch flag.Arg(0) {
		case "load":
			loadCmd.Parse(flag.Args()[1:])
			if *loadName == "" {
				fmt.Println("Error: <name> not provided")
				loadCmd.PrintDefaults()
				return
			}
			univ, cfg, frame, err := store.LoadUniverse(*loadName, db, ctx)
			if err != nil {
				log.Println(err)
				return
			}
			ticks += frame
			u = univ
			config = cfg
			loading = true
		case "save":
			saveCmd.Parse(flag.Args()[1:])
			if *saveName == "" {
				fmt.Println("Error: <name> not provided")
				saveCmd.PrintDefaults()
				return
			}
		case "list":
			List(ctx, db)
			return
		case "stats":
			Stats(ctx, db)
			return
		case "delete":
			deleteCmd.Parse(flag.Args()[1:])
			if *deleteSavedName == "" {
				fmt.Println("Error: <name> not provided")
				deleteCmd.PrintDefaults()
				return
			}
			err := store.DeleteSaveByName(ctx, db, *deleteSavedName)
			if err != nil {
				log.Println(err)
			}
			return
		case "bench":
			return
		default:
			break
		}
	}

	u.CountLiveCells()

	ticker := time.NewTicker(67 * time.Millisecond)

	timeout := time.After(30 * time.Second)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	seen := make(map[uint32]struct{})

	var configID int64
	if !loading {
		configID, err = store.InsertConfig(ctx, db, &u)

		if errors.Is(err, sql.ErrNoRows) {
			configID, err = store.GetUniverseConfigID(ctx, db, &u)
		}
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		configID = config.Id
	}
	fmt.Println("Game config:", configID)

	runID, err := store.InsertRun(ctx, db, configID)
	if err != nil {
		log.Println(err)
		return
	}

	cnts := make([]model.Tick, 0)

	first := true
	var endReason string
	if *headlessFlag {
		ticker.Stop()
	} else {
		defer ticker.Stop()
	}
loop:
	for {
		select {
		case <-timeout:
			if *headlessFlag {
				continue
			}
			fmt.Println("Done!")
			endReason = "timeout"
			break loop
		case interrupt := <-c:
			fmt.Println("Got signal:", interrupt)
			endReason = "interrupt"
			break loop
		case <-ticker.C:
			if !first {
				u.NextState()
			}
			cnts = append(cnts, model.Tick{
				Frame:     uint32(ticks),
				LiveCount: uint32(u.LiveCount),
				Hash:      u.CurrentHash(),
			})
			if !*headlessFlag {
				u.CurrentFrame(first)
				u.B.WriteTo(os.Stdout)
				u.B.Reset()
			}
			first = false
			ticks++

			if *cycleFlag {
				if _, ok := seen[u.CurrentHash()]; ok {
					fmt.Printf("Cycle detected on frame %d\n", ticks-1)
					endReason = "cycle"
					break loop
				} else {
					seen[u.CurrentHash()] = struct{}{}
				}
			}

			if u.LiveCount == 0 {
				fmt.Println("Extinction")
				endReason = "extinction"
				break loop
			}
		}
	}

	fmt.Printf("Game over after %d ticks\n", ticks)

	fmt.Printf("%d new ticks\n", len(cnts))
	err = store.InsertTicks(ctx, db, runID, cnts)
	if err != nil {
		fmt.Printf("Failed to insert %d ticks\n", ticks)
		return
	}

	if saveCmd.Parsed() {
		store.Save(ctx, db, runID, &u, saveCmd, saveName, ticks)
	}

	if enableSave {
		store.Save(ctx, db, runID, &u, loadCmd, loadName, ticks)
	}

	err = store.UpdateRunById(ctx, db, runID, endReason)
	if err != nil {
		fmt.Println(err)
	}
}
