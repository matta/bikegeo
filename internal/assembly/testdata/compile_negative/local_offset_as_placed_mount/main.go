package main

import (
	"bikegeo/internal/assembly"
	"bikegeo/internal/geom2d"
)

func mountStem(base assembly.StemBase, stem assembly.Stem) assembly.StemPlacement {
	ms, _ := base.AttachStem(stem)
	return ms.Finish()
}

func main() {
	pt := geom2d.Point2{X: 400, Y: 600}
	stem, _ := assembly.NewStem(40, 80, -6)
	_ = mountStem(pt, stem)
}
