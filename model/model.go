package model

import "bytes"

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
