package model

import (
	"reflect"
	"testing"
)

// Assume this is the function under test (adjust package/name as needed).
//
//     func Extract2D[T any](flat []T, width, height int) [][]T
//
// Rules we test against (Who's we lol? Vibers):
//   - returns nil if width<=0, height<=0, or len(flat) != width*height
//   - otherwise returns [height][width]T reading row-major

// ---- Generic test harness ----------------------------------------------

type tc[T any] struct {
	name          string
	input         []T
	width, height int
	want          [][]T
}

func run[T any](
	t *testing.T,
	extract func([]T, int, int) [][]T,
	cases []tc[T],
) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extract(c.input, c.width, c.height)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("Extract2D(%v, %d, %d) =\n  %#v\nwant\n  %#v",
					c.input, c.width, c.height, got, c.want)
			}
		})
	}
}
