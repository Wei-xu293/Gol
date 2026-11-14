package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"sync"
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

func listSaves(ctx context.Context, db *sql.DB) {
	fmt.Printf("%-10s %8s %8s\n", "Name", "Run ID", "Frame")
	saves, err := store.GetSaves(ctx, db)
	if err != nil {
		log.Println(err)
		return
	}
	for _, s := range saves {
		fmt.Printf("%-10s %8d %8d\n", s.Name, s.RunID, s.Frame)
	}
	fmt.Println()
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

func Step(ctx context.Context, u *model.Universe, checkCycle bool, ticks int,
	cnts []model.Tick, seen map[uint32]struct{}) (int, []model.Tick, string, error) {
	if err := ctx.Err(); err != nil {
		return ticks, cnts, "", err
	}
	if ticks > 0 {
		u.NextState()
	}
	cnts = append(cnts, model.Tick{
		Frame:     uint32(ticks),
		LiveCount: uint32(u.LiveCount),
		Hash:      u.CurrentHash(),
	})
	ticks++
	if checkCycle {
		if _, ok := seen[u.CurrentHash()]; ok {
			return ticks, cnts, "cycle", nil
		} else {
			seen[u.CurrentHash()] = struct{}{}
		}
	}

	if u.LiveCount == 0 {
		return ticks, cnts, "extinction", nil
	}
	return ticks, cnts, "", nil
}

func headless(u *model.Universe, ticks int, cnts []model.Tick, duration time.Duration, maxTicks int) (int, []model.Tick, string) {
	sigCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignal()
	runCtx, stopTimeout := context.WithTimeout(sigCtx, duration)
	defer stopTimeout()
	seen := make(map[uint32]struct{})
	endReason := ""
	var err error
	for range maxTicks {
		ticks, cnts, endReason, err = Step(runCtx, u, false, ticks, cnts, seen)
		if err != nil {
			switch {
			case errors.Is(err, context.Canceled):
				endReason = "interrupt"

			case errors.Is(err, context.DeadlineExceeded):
				endReason = "timeout"
			}
		}
		if endReason != "" {
			break
		}
	}
	fmt.Printf("ticks: %d, len(cnts): %d\n", ticks, len(cnts))
	if endReason == "" {
		endReason = "finished"
	}
	fmt.Println(endReason)
	return ticks, cnts, endReason
}

func terminalSimulation(u *model.Universe, ticks int, cnts []model.Tick, cycle bool) (int, []model.Tick, string) {
	ticker := time.NewTicker(67 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	seen := make(map[uint32]struct{})

	var endReason string
	first := true
loop:
	for {
		select {
		case <-timeout:
			fmt.Println("Done!")
			endReason = "timeout"
			break loop
		case interrupt := <-c:
			fmt.Println("Got signal:", interrupt)
			endReason = "interrupt"
			break loop
		case <-ticker.C:
			ticks, cnts, endReason, _ = Step(context.TODO(), u, cycle, ticks, cnts, seen)
			u.CurrentFrame(first)
			u.B.WriteTo(os.Stdout)
			u.B.Reset()
			if endReason != "" {
				break loop
			}
			first = false
		}
	}
	return ticks, cnts, endReason
}

func hunterWoker(ctx context.Context, rule uint32) <-chan model.HuntResult {
	result := make(chan model.HuntResult)
	sp := model.SeedPair{
		Seed1: rand.Uint64(),
		Seed2: rand.Uint64(),
	}
	u := model.InitRandomGrid(sp, 50, rand.Uint32N(1000), 0.25)
	u.Rule = rule
	log.Println(u.Width, u.Height)
	ticks := 0
	cnts := make([]model.Tick, 0)
	var endReason string
	seen := make(map[uint32]struct{})
	err := ctx.Err()
	go func() {
		for {
			if err != nil {
				switch {
				case errors.Is(err, context.Canceled):
					endReason = "interrupt"

				case errors.Is(err, context.DeadlineExceeded):
					endReason = "timeout"
				}
				break
			}
			ticks, cnts, endReason, err = Step(ctx, &u, true, ticks, cnts, seen)
			if endReason != "" {
				break
			}
		}

		log.Println(endReason)
		var stab model.Stabilization
		switch endReason {
		case "cycle":
			stab = model.Oscillator
		default:
			stab = model.Unclassified
		}

		result <- model.HuntResult{
			Sp:          sp,
			Ticks:       ticks,
			StabilizeAs: stab,
		}
		close(result)
	}()
	return result
}

func fanIn(inputs ...<-chan model.HuntResult) <-chan model.HuntResult {
	res := make(chan model.HuntResult)
	var wg sync.WaitGroup
	for _, in := range inputs {
		wg.Go(func() {
			for s := range in {
				res <- s
			}
		})
	}
	go func() {
		wg.Wait()
		close(res)
	}()
	return res
}

func hunt(rule uint32) {
	n := 10
	channels := make([]<-chan model.HuntResult, n)
	for i := range n {
		channels[i] = hunterWoker(context.Background(), rule)
	}
	c := fanIn(channels...)

	for res := range c {
		log.Println(res)
	}
	log.Println("Finished hunting.")
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
		log.Println(status)
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
			listSaves(ctx, db)
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
		case "hunt":
			hunt(0x1848)
			return
		default:
			break
		}
	}

	u.CountLiveCells()

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
	fmt.Printf("Run ID: %d\n", runID)

	var endReason string
	cnts := make([]model.Tick, 0)
	if *headlessFlag {
		ticks, cnts, endReason = headless(&u, ticks, cnts, 30*time.Second, 10000)
	} else {
		ticks, cnts, endReason = terminalSimulation(&u, ticks, cnts, *cycleFlag)
	}

	fmt.Printf("Game over after %d ticks\n", ticks)
	switch endReason {
	case "cycle":
		fmt.Printf("Cycle detected on frame %d\n", ticks-1)
	case "extinction":
		fmt.Println("Extinction")
	}

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
