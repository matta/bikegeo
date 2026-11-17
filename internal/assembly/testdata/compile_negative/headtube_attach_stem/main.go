package main

import (
	"bikegeo/internal/assembly"
	"bikegeo/internal/geom2d"
)

func main() {
	ht, _ := assembly.NewHeadTubeTop(geom2d.Origin(), 70)
	stem, _ := assembly.NewStem(40, 80, -6)
	_, _ = ht.AttachStem(stem)
}
