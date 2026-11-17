package main

import (
	"bikegeo/internal/assembly"
	"bikegeo/internal/geom2d"
)

func main() {
	sa, _ := assembly.NewSeatAxis(geom2d.Origin, 73)
	clamp, _ := sa.WithSeatpostClamp(700, 0)
	_ = assembly.BarClampMount(clamp)
}
