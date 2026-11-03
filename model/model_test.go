package model

import (
	"slices"
	"testing"
)

func getSquare() Universe {
	return Universe{
		Width:     2,
		Height:    2,
		Current:   slices.Clone(slices.Repeat([]uint8{1}, 4)),
		Next:      make([]uint8, 4),
		Rule:      0x1808,
		LiveCount: 4,
	}
}

func TestNextState(t *testing.T) {
	squareDead := getSquare()
	squareDead.Current = make([]uint8, 4)
	var tests = []struct {
		u     Universe
		steps int
		v     Universe
		name  string
	}{
		{getSquare(), 1, squareDead, "square"},
		// square with 3x3 board
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range tt.steps {
				tt.u.NextState()
			}
			if !slices.Equal(tt.u.Current, tt.v.Current) {
				t.Errorf("got %#v, want %#v", tt.u.Current, tt.v.Current)
			}
		})
	}
}
