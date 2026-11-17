package main

import (
	"bikegeo/internal/assembly"
	"bikegeo/internal/catalog"
	"bikegeo/internal/geom2d"
)

func main() {
	ht, _ := assembly.NewHeadTubeTop(geom2d.Origin, 70)
	sb, _ := ht.WithUpperStack(10, 20)
	bar, _ := assembly.NewHandlebar(catalog.DefaultHandlebar(), 65)
	_, _ = sb.AttachHandlebar(bar, 0)
}
