package main

import (
	"bikegeo/internal/assembly"
	"bikegeo/internal/geom2d"
)

func main() {
	ht, _ := assembly.NewHeadTubeTop(geom2d.Origin(), 70)
	sb, _ := ht.WithUpperStack(10, 20)
	_, _ = sb.WithUpperStack(5, 5)
}
