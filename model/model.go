package model

import (
	"bytes"
	"hash/fnv"
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

type Config struct {
	Id           int64
	Width        uint32
	Height       uint32
	Rule         uint32
	Initial_hash uint32
}

func (u *Universe) CurrentHash() uint32 {
	h := fnv.New32a()
	h.Write(u.Current)
	return h.Sum32()
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
