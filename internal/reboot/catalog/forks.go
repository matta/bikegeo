package catalog

import "bikegeo/internal/reboot/assembly"

// Fork is a catalog fork model. Frames install it at a frame-specific A2C and rake (Q66).
type Fork struct {
	ID      string
	Name    string
	Travel  assembly.ForkTravel
	Steerer assembly.Steerer
}

// FrameFork is one fork a frame supports, at that frame's A2C and rake variant.
type FrameFork struct {
	Fork        Fork
	StaticA2cMm float64
	RakeMm      float64
}

// Spec returns the installed fork's assembly specification.
func (f FrameFork) Spec() assembly.ForkSpec {
	return assembly.ForkSpec{
		StaticA2cMm: f.StaticA2cMm,
		Travel:      f.Fork.Travel,
		RakeMm:      f.RakeMm,
		Steerer:     f.Fork.Steerer,
	}
}

// The fork models, ported from legacy internal/catalog CommonForkCatalog
// (IDs, names, and travel). Steerers follow Q72: the four named straight
// 1-1/8" forks and the other named tapered forks are catalog values; the
// generic rigid_* forks are illustratively tapered.
var (
	sid100               = Fork{ID: "sid_100", Name: "RockShox SID 100mm", Travel: assembly.Travel100, Steerer: assembly.SteererTapered}                   // catalog
	fox34_120            = Fork{ID: "fox34_120", Name: "Fox Float 34 120mm", Travel: assembly.Travel120, Steerer: assembly.SteererTapered}                 // catalog
	sid120               = Fork{ID: "sid_120", Name: "RockShox SID 120mm", Travel: assembly.Travel120, Steerer: assembly.SteererTapered}                   // catalog
	pike120              = Fork{ID: "pike_120", Name: "RockShox Pike 120mm", Travel: assembly.Travel120, Steerer: assembly.SteererTapered}                 // catalog
	fox34_130            = Fork{ID: "fox34_130", Name: "Fox Float 34 130mm", Travel: assembly.Travel130, Steerer: assembly.SteererTapered}                 // catalog
	sunliner440          = Fork{ID: "sunliner_440", Name: "Tumbleweed Chromoly Steel Unicrown 440mm", Steerer: assembly.SteererStraight118}                // catalog
	surlyEcr468          = Fork{ID: "surly_ecr_468", Name: "Surly ECR 4130 Chromoly 468mm", Steerer: assembly.SteererStraight118}                          // catalog
	enveMountain472      = Fork{ID: "enve_mountain_472", Name: "ENVE Mountain Carbon 472mm", Steerer: assembly.SteererTapered}                             // catalog
	enveMountainBoost    = Fork{ID: "enve_mountain_boost", Name: "ENVE Mountain Carbon Boost 492mm", Steerer: assembly.SteererTapered}                     // catalog
	firestarterDeluxe483 = Fork{ID: "firestarter_deluxe_483", Name: "Salsa Firestarter / Cutthroat Carbon Deluxe 483mm", Steerer: assembly.SteererTapered} // catalog
	surlyKrampus483      = Fork{ID: "surly_krampus_483", Name: "Surly Krampus 4130 Chromoly 483mm", Steerer: assembly.SteererStraight118}                  // catalog
	rigid483             = Fork{ID: "rigid_483", Name: "Rigid Bikepacking 483mm", Steerer: assembly.SteererTapered}                                        // illustrative
	rigid485             = Fork{ID: "rigid_485", Name: "Rigid Bikepacking 485mm", Steerer: assembly.SteererTapered}                                        // illustrative
	sklarSteel485        = Fork{ID: "sklar_steel_485", Name: "Sklar Chromoly Steel 485mm", Steerer: assembly.SteererStraight118}                           // catalog
	rigid490             = Fork{ID: "rigid_490", Name: "Rigid Bikepacking 490mm", Steerer: assembly.SteererTapered}                                        // illustrative
	lithicCarbonMountain = Fork{ID: "lithic_carbon_mountain", Name: "Lithic Carbon Mountain", Steerer: assembly.SteererTapered}                            // catalog
	rigid495             = Fork{ID: "rigid_495", Name: "Rigid Bikepacking 495mm", Steerer: assembly.SteererTapered}                                        // illustrative
	rigid500             = Fork{ID: "rigid_500", Name: "Rigid Bikepacking 500mm", Steerer: assembly.SteererTapered}                                        // illustrative
	rigid510             = Fork{ID: "rigid_510", Name: "Rigid Bikepacking 510mm", Steerer: assembly.SteererTapered}                                        // illustrative
)

// Forks returns the fork models in catalog order.
func Forks() []Fork {
	return []Fork{
		sid100, fox34_120, sid120, pike120, fox34_130,
		sunliner440, surlyEcr468, enveMountain472, enveMountainBoost,
		firestarterDeluxe483, surlyKrampus483, rigid483, rigid485, sklarSteel485,
		rigid490, lithicCarbonMountain, rigid495, rigid500, rigid510,
	}
}
