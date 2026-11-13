package model

import (
	"bytes"
	"hash/fnv"
	"math/rand/v2"
)

type Universe struct {
	Width     uint32
	Height    uint32
	Current   []uint8
	Next      []uint8
	Rule      uint32
	LiveCount int
	B         bytes.Buffer
}

type Tick struct {
	Frame     uint32
	LiveCount uint32
	Hash      uint32
}

type Config struct {
	Id           int64
	Width        uint32
	Height       uint32
	Rule         uint32
	Initial_hash uint32
}

type TicksInfo struct {
	TotalFrames int
	MaxAlive    int
	AvgAlive    float32
}

type GrowthDieOff struct {
	RunID  int
	Growth int
	DieOff int
}

type SaveInfo struct {
	Name  string
	RunID int
	Frame int
}

var dirs = [8][2]int{
	{-1, -1}, {-1, 0}, {-1, 1},
	{0, -1}, {0, 1},
	{1, -1}, {1, 0}, {1, 1},
}

// TODO: count distinct cells version
func (u *Universe) NextState() {
	w, h := int(u.Width), int(u.Height)
	u.LiveCount = 0

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
			u.LiveCount += int(u.Next[idx])
		}
	}
	u.Current, u.Next = u.Next, u.Current
}

// TODO: use \033[%dA passing line count (Height - i)
func (u *Universe) CurrentFrame(first bool) error {
	if !first {
		for range u.Height {
			if _, err := u.B.WriteString("\033[1A\033[2K"); err != nil {
				return err
			}
		}
	}
	for i := range u.Height {
		if _, err := u.B.WriteString("\r\033[2K"); err != nil {
			return err
		}
		for j := range u.Width {
			a := ' '
			idx := i*u.Width + j
			if u.Current[idx] == 1 {
				a = '*'
			}
			if _, err := u.B.WriteRune(a); err != nil {
				return err
			}
		}
		if _, err := u.B.WriteRune('\n'); err != nil {
			return err
		}
	}
	return nil
}

func (u *Universe) CountLiveCells() {
	u.LiveCount = 0
	for _, val := range u.Current {
		u.LiveCount += int(val)
	}
}

func (u *Universe) CurrentHash() uint32 {
	h := fnv.New32a()
	h.Write(u.Current)
	return h.Sum32()
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

func RandUniverse(size uint32, rule uint32) Universe {
	curr, cnt := RancCurrent(size, size)
	return Universe{
		Width:     size,
		Height:    size,
		Current:   curr,
		Next:      make([]uint8, size*size),
		LiveCount: cnt,
		Rule:      rule,
	}
}

func ConfigFromUniv(u *Universe) Config {
	return Config{
		Width:        u.Width,
		Height:       u.Height,
		Rule:         u.Rule,
		Initial_hash: u.CurrentHash(),
	}
}

type SeedPair struct {
	Seed1 uint64 `json:"seed1"`
	Seed2 uint64 `json:"seed2"`
}

func InitRandomGrid(seed SeedPair, width, height uint32, fillRatio float64) Universe {
	r := rand.New(rand.NewPCG(seed.Seed1, seed.Seed2))
	curr := make([]uint8, width*height)
	cnt := uint64(0)
	for i := range curr {
		if r.Float64() < fillRatio {
			curr[i] = 1
		}
		cnt += uint64(curr[i])
	}
	return Universe{
		Width:     width,
		Height:    height,
		Current:   curr,
		Next:      make([]uint8, width*height),
		LiveCount: int(cnt),
	}
}
