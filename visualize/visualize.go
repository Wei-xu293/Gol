package visualize

import (
	"fmt"
	"math/rand/v2"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type Point struct {
	X, Y int32
}

type AABB struct {
	Center                Point
	HalfWidth, HalfHeight int32
}

type QuadTree struct {
	// Axis-aligned bounding box stored as a center with half-dimensions
	// to represent the boundaries of this quad tree
	Boundary  AABB
	Capacity  int
	Points    []Point
	NorthWest *QuadTree
	NorthEast *QuadTree
	SouthWest *QuadTree
	SouthEast *QuadTree
}

func (r *AABB) contains(p Point) bool {
	return r.Center.X-r.HalfWidth <= p.X && p.X <= r.Center.X+r.HalfWidth &&
		r.Center.Y-r.HalfHeight <= p.Y && p.Y <= r.Center.Y+r.HalfHeight
}

func (qt *QuadTree) subdivide() {
	c := qt.Boundary.Center
	hw, hh := qt.Boundary.HalfWidth, qt.Boundary.HalfHeight
	qt.NorthWest = &QuadTree{
		Boundary: AABB{
			Center:     Point{c.X - hw/2, c.Y - hh/2},
			HalfWidth:  hw / 2,
			HalfHeight: hh / 2},
		Capacity: qt.Capacity,
	}
	qt.NorthEast = &QuadTree{
		Boundary: AABB{
			Center:     Point{c.X + hw/2, c.Y - hh/2},
			HalfWidth:  hw / 2,
			HalfHeight: hh / 2},
		Capacity: qt.Capacity,
	}
	qt.SouthWest = &QuadTree{
		Boundary: AABB{
			Center:     Point{c.X - hw/2, c.Y + hh/2},
			HalfWidth:  hw / 2,
			HalfHeight: hh / 2},
		Capacity: qt.Capacity,
	}
	qt.SouthEast = &QuadTree{
		Boundary: AABB{
			Center:     Point{c.X + hw/2, c.Y + hh/2},
			HalfWidth:  hw / 2,
			HalfHeight: hh / 2},
		Capacity: qt.Capacity,
	}
}

func (qt *QuadTree) isLeaf() bool { return qt.NorthWest == nil }

func (qt *QuadTree) routeDirection(p Point) int {
	goRight := p.X >= qt.Boundary.Center.X
	goTop := p.Y < qt.Boundary.Center.Y
	return b2i(goTop)*2 + b2i(goRight)
}

func (qt *QuadTree) routePoint(p Point) {
	switch qt.routeDirection(p) {
	case 3:
		qt.NorthEast.insert(p)
	case 2:
		qt.NorthWest.insert(p)
	case 1:
		qt.SouthEast.insert(p)
	default:
		qt.SouthWest.insert(p)
	}
}

func (qt *QuadTree) insert(p Point) {
	if qt.isLeaf() {
		if len(qt.Points) < qt.Capacity {
			qt.Points = append(qt.Points, p)
		} else {
			qt.subdivide()
			toInsert := make([]Point, 0)
			toInsert = append(toInsert, qt.Points...)
			toInsert = append(toInsert, p)
			qt.Points = nil
			for _, pi := range toInsert {
				qt.routePoint(pi)
			}
		}
	} else {
		qt.routePoint(p)
	}
}

// Returns true if the point was inserted; false if outside the tree's boundary.
func (qt *QuadTree) Insert(p Point) bool {
	if !qt.Boundary.contains(p) {
		return false
	}
	qt.insert(p)
	return true
}

func (qt *QuadTree) Find(p Point) (Point, bool) {
	if qt == nil {
		return p, false
	}
	if qt.isLeaf() {
		for _, pi := range qt.Points {
			if p == pi {
				return pi, true
			}
		}
	}
	switch qt.routeDirection(p) {
	case 3:
		return qt.NorthEast.Find(p)
	case 2:
		return qt.NorthWest.Find(p)
	case 1:
		return qt.SouthEast.Find(p)
	default:
		return qt.SouthWest.Find(p)
	}
}

func (qt *QuadTree) Contains(p Point) bool {
	if !qt.Boundary.contains(p) {
		return false
	}
	_, found := qt.Find(p)
	return found
}

func (qt *QuadTree) show() {
	if qt == nil {
		return
	}
	if qt.isLeaf() {
		qt.Boundary.draw()
	}
	if qt.NorthWest != nil {
		qt.NorthWest.show()
		qt.NorthEast.show()
		qt.SouthWest.show()
		qt.SouthEast.show()
	}
	for _, p := range qt.Points {
		p.draw()
	}
}

func (r *AABB) draw() {
	rl.DrawRectangle(r.Center.X-r.HalfWidth, r.Center.Y-r.HalfHeight,
		r.HalfWidth*2, r.HalfHeight*2, rl.Black)
	rl.DrawRectangleLines(r.Center.X-r.HalfWidth, r.Center.Y-r.HalfHeight,
		r.HalfWidth*2, r.HalfHeight*2, rl.Orange)

}

func (p *Point) draw() {
	rl.DrawCircle(p.X, p.Y, 3.0, rl.LightGray)
	rl.DrawCircleLines(p.X, p.Y, 3.0, rl.NewColor(214, 211, 31, 1))
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func View() {
	rl.InitWindow(600, 600, "Window")
	defer rl.CloseWindow()
	if !rl.IsWindowReady() {
		fmt.Println("Error: window not ready")
		return
	}
	rl.SetTargetFPS(60)
	qt := QuadTree{
		Boundary: AABB{Point{300, 300}, 300, 300},
		Capacity: 4,
	}
	for range 500 {
		p := Point{rand.Int32N(650), rand.Int32N(650)}
		if !qt.Insert(p) {
			fmt.Println(p)
		}
	}
	for !rl.WindowShouldClose() {
		rl.BeginDrawing()
		rl.ClearBackground(rl.White)

		//cx := rl.GetRenderWidth() / 2
		//cy := rl.GetRenderHeight() / 2

		//rl.DrawLine(int32(cx), 0, int32(cx), int32(rl.GetRenderHeight()), rl.Red)
		//rl.DrawLine(0, int32(cy), int32(rl.GetRenderWidth()), int32(cy), rl.Red)
		qt.show()

		rl.EndDrawing()
	}
}
