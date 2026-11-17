package main

import (
	"bikegeo/internal/assembly"
	"bikegeo/internal/geom2d"
)

func main() {
	ht, _ := assembly.NewHeadTubeTop(geom2d.Origin(), 70)
	sb, _ := ht.WithUpperStack(10, 20)
	stem, _ := assembly.NewStem(40, 80, -6)
	sp, _ := sb.AttachStem(stem)
	mount := sp.BarClampMount()
	_, _ = mount.AttachStem(stem)
}
