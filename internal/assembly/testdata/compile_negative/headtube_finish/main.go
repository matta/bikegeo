package main

import (
	"bikegeo/internal/assembly"
	"bikegeo/internal/geom2d"
)

func main() {
	ht, _ := assembly.NewHeadTubeTop(geom2d.Origin, 70)
	_ = ht.Finish()
}
