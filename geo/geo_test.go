package geo

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

func TestDistance(t *testing.T) {
	// One degree of latitude is 2πR/360 = 111.19 km, anywhere.
	if d := Distance(12, 77, 13, 77); math.Abs(d-111.19) > 0.01 {
		t.Fatalf("1° of latitude = %.3f km", d)
	}
	// A degree of longitude shrinks with cos(latitude): half at 60°.
	if d := Distance(60, 10, 60, 11); math.Abs(d-111.19*0.5) > 0.1 {
		t.Fatalf("1° of longitude at 60° = %.3f km", d)
	}
}

func TestGeohashString(t *testing.T) {
	// A well-known test vector: 57.64911, 10.40744 → u4pruydqqvj.
	if got := String(Encode(57.64911, 10.40744, 55), 55); got != "u4pruydqqvj" {
		t.Fatalf("geohash %s", got)
	}
}

// Every index must return exactly what a full scan returns, including
// points just inside the radius in a neighbouring cell.
func TestIndexesAgreeWithScan(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 1))
	pts := make([]Point, 50_000)
	for i := range pts {
		pts[i] = Point{i, 12.8 + r.Float64()*0.4, 77.4 + r.Float64()*0.4} // around Bengaluru
	}
	scan := NewScan(pts)
	idx := []Index{NewGeohash(pts), NewQuadtree(pts)}
	for q := range 300 {
		lat, lon := 12.8+r.Float64()*0.4, 77.4+r.Float64()*0.4
		km := []float64{0.3, 1, 2, 5}[q%4]
		want, _ := scan.Within(lat, lon, km)
		for _, ix := range idx {
			got, checked := ix.Within(lat, lon, km)
			if !slices.Equal(got, want) {
				t.Fatalf("%s at %.4f,%.4f r=%.1f: %d points, scan found %d", ix.Name(), lat, lon, km, len(got), len(want))
			}
			if checked >= len(pts) {
				t.Fatalf("%s checked every point", ix.Name())
			}
		}
	}
}
