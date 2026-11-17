package main

import (
	"bikegeo/internal/assembly"
)

func main() {
	stem, _ := assembly.NewStem(40, 80, -6)
	_ = stem.Finish()
}
