package main

import (
	"bikegeo/internal/assembly"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

func main() {
	sa, _ := assembly.NewSeatAxis(geom2d.Origin(), 73)
	saddle, _ := assembly.NewSaddle(model.ReferenceIdmatchSaddle())
	_, _ = sa.AttachSaddle(saddle, 0)
}
