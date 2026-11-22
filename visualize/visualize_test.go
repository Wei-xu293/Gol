package visualize

import (
	"math/rand/v2"
	"testing"
)

func TestRoutingConsistency(t *testing.T) {
	qt := QuadTree{
		Boundary: AABB{Point{300, 300}, 300, 300},
		Capacity: 4,
	}
	outOfScope := 0
	pi := make([]Point, 0)
	n := 100000
	for range n {
		p := Point{rand.Int32N(650), rand.Int32N(650)}
		if !qt.Insert(p) {
			outOfScope++
		}
		pi = append(pi, p)
	}
	failures := 0
	for _, p := range pi {
		if !qt.Contains(p) {
			failures++
		}
	}
	if failures != outOfScope {
		t.Errorf("failures differs from out of scope points: got %d failures, want %d points\n", failures, outOfScope)
	}
}
