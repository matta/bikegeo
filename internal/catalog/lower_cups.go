package catalog

import "github.com/matta/bikegeo/internal/assembly"

// TypicalLowerCup returns the typical lower headset cup for a physical cup
// standard: the Cane Creek 40-Series stack, which the /30 and /40 conversion
// rings share (Q136, lower_cups.md). The bare head tube is a chart-basis
// hypothesis, not a physical cup, so it has no typical cup.
func TypicalLowerCup(std assembly.LowerCupStandard) (assembly.LowerCupSetup, bool) {
	switch std {
	case assembly.LowerCupEC34:
		return assembly.LowerCupSetup{Standard: assembly.LowerCupEC34, StackMm: 12}, true
	case assembly.LowerCupEC44:
		return assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12}, true
	case assembly.LowerCupZS44:
		return assembly.LowerCupSetup{Standard: assembly.LowerCupZS44, StackMm: 4}, true
	case assembly.LowerCupZS56:
		return assembly.LowerCupSetup{Standard: assembly.LowerCupZS56, StackMm: 4}, true
	case assembly.LowerCupIS42:
		return assembly.LowerCupSetup{Standard: assembly.LowerCupIS42, StackMm: 1}, true
	case assembly.LowerCupIS52:
		return assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 1}, true
	case assembly.LowerCupBareHeadTube:
		return assembly.LowerCupSetup{}, false
	default:
		return assembly.LowerCupSetup{}, false
	}
}

// LowerCupFor resolves the typical lower cup a fork with the given steerer
// installs on a head tube with the given lower bore (Q136). defaultStd is the
// frame's default cup standard: on a 44 mm bore a straight steerer keeps a
// ZS44 default and otherwise takes EC44, while a tapered steerer always takes
// EC44. It reports false when the steerer does not fit the bore or is not a
// physical steerer.
func LowerCupFor(bore assembly.LowerBore, defaultStd assembly.LowerCupStandard, steerer assembly.Steerer) (assembly.LowerCupSetup, bool) {
	switch steerer {
	case assembly.SteererStraight118, assembly.SteererTapered:
	case assembly.SteererBasis:
		return assembly.LowerCupSetup{}, false
	default:
		return assembly.LowerCupSetup{}, false
	}
	switch bore {
	case assembly.Bore34Pressed:
		if steerer == assembly.SteererStraight118 {
			return TypicalLowerCup(assembly.LowerCupEC34)
		}
		return assembly.LowerCupSetup{}, false
	case assembly.Bore42Integrated:
		if steerer == assembly.SteererStraight118 {
			return TypicalLowerCup(assembly.LowerCupIS42)
		}
		return assembly.LowerCupSetup{}, false
	case assembly.Bore44Pressed:
		if steerer == assembly.SteererStraight118 && defaultStd == assembly.LowerCupZS44 {
			return TypicalLowerCup(assembly.LowerCupZS44)
		}
		return TypicalLowerCup(assembly.LowerCupEC44)
	case assembly.Bore56Pressed:
		return TypicalLowerCup(assembly.LowerCupZS56)
	case assembly.Bore52Integrated:
		return TypicalLowerCup(assembly.LowerCupIS52)
	default:
		return assembly.LowerCupSetup{}, false
	}
}
