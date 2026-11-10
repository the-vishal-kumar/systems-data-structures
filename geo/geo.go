// Package geo answers "which points are within r km of here?" three ways:
// a full scan, a geohash grid (look up the cell you're in and its eight
// neighbours), and a point quadtree (descend only into boxes that overlap
// the search area). All three return exactly the same points; they differ
// in how many candidates they look at.
package geo

import (
	"math"
	"sort"
)

// Point is a location with an ID.
type Point struct {
	ID       int
	Lat, Lon float64
}

const earthKm = 6371.0

// Distance is the great-circle (haversine) distance in kilometres.
func Distance(lat1, lon1, lat2, lon2 float64) float64 {
	rad := math.Pi / 180
	dLat, dLon := (lat2-lat1)*rad, (lon2-lon1)*rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthKm * math.Asin(math.Sqrt(a))
}

// Index finds points near a location.
type Index interface {
	// Within returns the IDs within km of (lat, lon), sorted, and how many
	// points it had to check the distance of.
	Within(lat, lon, km float64) (ids []int, checked int)
	Name() string
}

func filter(cands []Point, lat, lon, km float64) []int {
	var out []int
	for _, p := range cands {
		if Distance(lat, lon, p.Lat, p.Lon) <= km {
			out = append(out, p.ID)
		}
	}
	sort.Ints(out)
	return out
}

// ---- full scan ----

type Scan struct{ pts []Point }

func NewScan(pts []Point) *Scan { return &Scan{pts} }
func (s *Scan) Name() string    { return "full scan" }
func (s *Scan) Within(lat, lon, km float64) ([]int, int) {
	return filter(s.pts, lat, lon, km), len(s.pts)
}

// ---- geohash ----

// Encode returns the geohash cell of (lat, lon) at the given number of
// bits (alternating longitude and latitude halvings, longitude first), as
// an integer. The base-32 string form is just this in 5-bit groups.
func Encode(lat, lon float64, bits int) uint64 {
	latLo, latHi, lonLo, lonHi := -90.0, 90.0, -180.0, 180.0
	var h uint64
	for i := range bits {
		h <<= 1
		if i%2 == 0 {
			mid := (lonLo + lonHi) / 2
			if lon >= mid {
				h |= 1
				lonLo = mid
			} else {
				lonHi = mid
			}
		} else {
			mid := (latLo + latHi) / 2
			if lat >= mid {
				h |= 1
				latLo = mid
			} else {
				latHi = mid
			}
		}
	}
	return h
}

// CellSize returns a cell's height and width in degrees at bits.
func CellSize(bits int) (dLat, dLon float64) {
	lonBits, latBits := (bits+1)/2, bits/2
	return 180 / math.Exp2(float64(latBits)), 360 / math.Exp2(float64(lonBits))
}

const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"

// String returns the familiar geohash text for a cell (bits must be a
// multiple of 5).
func String(h uint64, bits int) string {
	out := make([]byte, bits/5)
	for i := range out {
		out[len(out)-1-i] = base32[h&31]
		h >>= 5
	}
	return string(out)
}

// Geohash buckets points by cell. A query looks at the 3×3 block of cells
// around the location, at a precision whose cells are at least km across,
// so every point within km is in one of them.
type Geohash struct {
	cells map[int]map[uint64][]Point // bits → cell → points
}

// Precisions to index, from coarse to fine.
var precisions = []int{10, 15, 20, 25, 30, 35}

func NewGeohash(pts []Point) *Geohash {
	g := &Geohash{cells: map[int]map[uint64][]Point{}}
	for _, b := range precisions {
		m := map[uint64][]Point{}
		for _, p := range pts {
			c := Encode(p.Lat, p.Lon, b)
			m[c] = append(m[c], p)
		}
		g.cells[b] = m
	}
	return g
}

func (g *Geohash) Name() string { return "geohash cells" }

func (g *Geohash) Within(lat, lon, km float64) ([]int, int) {
	// The finest precision whose cell height and width both cover km.
	bits := precisions[0]
	for _, b := range precisions {
		dLat, dLon := CellSize(b)
		hKm := dLat * math.Pi / 180 * earthKm
		wKm := dLon * math.Pi / 180 * earthKm * math.Cos(math.Abs(lat)*math.Pi/180+dLat*math.Pi/180)
		if hKm >= km && wKm >= km {
			bits = b
		}
	}
	dLat, dLon := CellSize(bits)
	var cands []Point
	seen := map[uint64]bool{}
	for _, dy := range []float64{-1, 0, 1} {
		for _, dx := range []float64{-1, 0, 1} {
			y, x := lat+dy*dLat, lon+dx*dLon
			if y > 90 || y < -90 {
				continue
			}
			if x >= 180 {
				x -= 360
			} else if x < -180 {
				x += 360
			}
			c := Encode(y, x, bits)
			if !seen[c] {
				seen[c] = true
				cands = append(cands, g.cells[bits][c]...)
			}
		}
	}
	return filter(cands, lat, lon, km), len(cands)
}

// ---- quadtree ----

const leafCap = 32

type box struct{ minLat, minLon, maxLat, maxLon float64 }

type quad struct {
	b    box
	pts  []Point
	kids *[4]quad
}

// Quadtree splits a box into four when it holds more than 32 points, so
// dense areas get small boxes and empty areas stay as one big box.
type Quadtree struct{ root quad }

func NewQuadtree(pts []Point) *Quadtree {
	q := &Quadtree{root: quad{b: box{-90, -180, 90, 180}}}
	for _, p := range pts {
		q.root.insert(p, 0)
	}
	return q
}

func (q *quad) insert(p Point, depth int) {
	if q.kids == nil {
		q.pts = append(q.pts, p)
		if len(q.pts) > leafCap && depth < 30 {
			q.split(depth)
		}
		return
	}
	q.kids[q.child(p)].insert(p, depth+1)
}

func (q *quad) child(p Point) int {
	midLat, midLon := (q.b.minLat+q.b.maxLat)/2, (q.b.minLon+q.b.maxLon)/2
	i := 0
	if p.Lat >= midLat {
		i |= 2
	}
	if p.Lon >= midLon {
		i |= 1
	}
	return i
}

func (q *quad) split(depth int) {
	b := q.b
	midLat, midLon := (b.minLat+b.maxLat)/2, (b.minLon+b.maxLon)/2
	q.kids = &[4]quad{
		{b: box{b.minLat, b.minLon, midLat, midLon}},
		{b: box{b.minLat, midLon, midLat, b.maxLon}},
		{b: box{midLat, b.minLon, b.maxLat, midLon}},
		{b: box{midLat, midLon, b.maxLat, b.maxLon}},
	}
	pts := q.pts
	q.pts = nil
	for _, p := range pts {
		q.kids[q.child(p)].insert(p, depth+1)
	}
}

func (q *quad) search(s box, out *[]Point) {
	if q.b.maxLat < s.minLat || q.b.minLat > s.maxLat || q.b.maxLon < s.minLon || q.b.minLon > s.maxLon {
		return
	}
	if q.kids == nil {
		for _, p := range q.pts {
			if p.Lat >= s.minLat && p.Lat <= s.maxLat && p.Lon >= s.minLon && p.Lon <= s.maxLon {
				*out = append(*out, p)
			}
		}
		return
	}
	for i := range q.kids {
		q.kids[i].search(s, out)
	}
}

func (q *Quadtree) Name() string { return "quadtree" }

// Within searches the bounding box of the circle, then filters by distance.
// (Boxes that cross the ±180° meridian aren't handled; fine for one country.)
func (q *Quadtree) Within(lat, lon, km float64) ([]int, int) {
	dLat := km / earthKm * 180 / math.Pi
	dLon := dLat / math.Max(math.Cos(lat*math.Pi/180), 1e-6)
	var cands []Point
	q.root.search(box{lat - dLat, lon - dLon, lat + dLat, lon + dLon}, &cands)
	return filter(cands, lat, lon, km), len(cands)
}
