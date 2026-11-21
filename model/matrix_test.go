package model

import (
	"reflect"
	"testing"
)

// Assume this is the function under test (adjust package/name as needed).
//
//     func Extract2D[T any](flat []T, x, y, owidth, oheight, width, height int) [][]T
//
// Rules we test against (Who's we lol? Vibers):
//   - returns nil if width<=0, height<=0, or len(flat) != width*height
//   - otherwise returns [height][width]T reading row-major

// ---- Generic test harness ----------------------------------------------

type tc[T any] struct {
	name            string
	input           []T
	x, y            int
	owidth, oheight int
	width, height   int
	want            [][]T
}

func run[T any](
	t *testing.T,
	extract func([]T, int, int, int, int, int, int) [][]T,
	cases []tc[T],
) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extract(c.input, c.x, c.y, c.owidth, c.oheight, c.width, c.height)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("Extract2D(%v, %d, %d, %d, %d, %d, %d) =\n  %#v\nwant\n  %#v",
					c.input, c.x, c.y, c.owidth, c.oheight, c.width, c.height, got, c.want)
			}
		})
	}
}
