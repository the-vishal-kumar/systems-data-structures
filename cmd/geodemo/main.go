// Command geodemo finds the stores within 1, 2 and 5 km of a location among
// 1,000,000 points spread over India (denser in cities), with a full scan,
// geohash cells and a quadtree: points checked and time per query.
//
//	go run ./cmd/geodemo
package main

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/the-vishal-kumar/systems-data-structures/geo"
)

var cities = [][2]float64{ // approximate centres
	{28.61, 77.21}, {19.08, 72.88}, {12.97, 77.59}, {13.08, 80.27}, {22.57, 88.36},
	{17.39, 78.49}, {18.52, 73.86}, {23.02, 72.57}, {26.91, 75.79}, {28.46, 77.03},
}

func main() {
	r := rand.New(rand.NewPCG(4, 2))
	pts := make([]geo.Point, 1_000_000)
	for i := range pts {
		if i%2 == 0 { // half in cities, within ~20 km of a centre
			c := cities[r.IntN(len(cities))]
			pts[i] = geo.Point{ID: i, Lat: c[0] + r.NormFloat64()*0.08, Lon: c[1] + r.NormFloat64()*0.08}
		} else { // half anywhere in a box around India
			pts[i] = geo.Point{ID: i, Lat: 8 + r.Float64()*27, Lon: 69 + r.Float64()*20}
		}
	}
	t := time.Now()
	gh := geo.NewGeohash(pts)
	ghBuild := time.Since(t)
	t = time.Now()
	qt := geo.NewQuadtree(pts)
	qtBuild := time.Since(t)

	fmt.Println("## Finding what's nearby (`cmd/geodemo`)")
	fmt.Println()
	fmt.Printf("1,000,000 points over India, half clustered around 10 cities. Build time: geohash %.0f ms (6 precisions), quadtree %.0f ms. 200 queries per row (the full scan runs the first 20 of them, so its average differs slightly), half in a city centre and half anywhere.\n\n",
		float64(ghBuild.Milliseconds()), float64(qtBuild.Milliseconds()))
	fmt.Println("| Radius | Method | Points checked per query (avg) | Found (avg) | Time per query |\n|---:|---|---:|---:|---:|")
	for _, km := range []float64{1, 2, 5} {
		qs := make([][2]float64, 200)
		for i := range qs {
			if i%2 == 0 {
				c := cities[r.IntN(len(cities))]
				qs[i] = [2]float64{c[0] + r.NormFloat64()*0.02, c[1] + r.NormFloat64()*0.02}
			} else {
				qs[i] = [2]float64{8 + r.Float64()*27, 69 + r.Float64()*20}
			}
		}
		for _, ix := range []geo.Index{geo.NewScan(pts), gh, qt} {
			checked, found := 0, 0
			n := len(qs)
			if ix.Name() == "full scan" {
				n = 20
			}
			t := time.Now()
			for _, q := range qs[:n] {
				ids, c := ix.Within(q[0], q[1], km)
				checked += c
				found += len(ids)
			}
			per := time.Since(t) / time.Duration(n)
			fmt.Printf("| %.0f km | %s | %s | %.0f | %s |\n", km, ix.Name(), comma(checked/n), float64(found)/float64(n), per.Round(time.Microsecond))
		}
	}
	fmt.Println()
}

func comma(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
