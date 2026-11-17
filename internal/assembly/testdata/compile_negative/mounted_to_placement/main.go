package main

import "bikegeo/internal/assembly"

func main() {
	var mounted assembly.MountedStem
	_ = assembly.StemPlacement(mounted)
}
