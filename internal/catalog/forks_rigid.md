# Rigid fork classes and provenance

Source record for `RigidClass` on the commercial rigid forks in `forks.go`
(Q135, Q137, Q138). A rigid fork's axle-to-crown length is quoted
"suspension-corrected" when the manufacturer sizes it to replace a suspension
fork of a given travel. The class groups forks of one effective length so a
frame that lists one fork also accepts the others of its class.

## Classes and stand-ins

A listed rigid fork unlocks only its own class. A listed suspension fork
unlocks its own travel and the rigid classes that stand in for it; the
stand-in is one way.

| Class | Forks | Stands in for |
| --- | --- | --- |
| `440` | `sunliner_440` | none |
| `470` | `surly_ecr_468` | none |
| `485` | `firestarter_deluxe_483`, `surly_krampus_483`, `sklar_steel_485` | 100 mm |
| `495` | `enve_mountain_boost`, `lithic_carbon_mountain` | 120 mm |

130 mm suspension forks unlock no rigid class.

## Forks

| Fork | Class | Steerer | Correction basis |
| --- | --- | --- | --- |
| `sunliner_440` | `440` | straight | Not suspension-corrected. |
| `surly_ecr_468` | `470` | straight, EC34 | 80 mm-corrected. |
| `firestarter_deluxe_483` | `485` | tapered | 100 mm-corrected. |
| `surly_krampus_483` | `485` | straight | 100 mm class, on a frame that accepts up to 120 mm and 550 mm. |
| `sklar_steel_485` | `485` | straight | 100 mm-corrected, on a frame tested to 120 mm. |
| `enve_mountain_boost` | `495` | tapered | 120 mm-corrected. |
| `lithic_carbon_mountain` | `495` | tapered | 100 mm and 120 mm-corrected; Q135 maps it to the 120 mm stand-in. |

## Frames that list a rigid fork

`tumbleweed-sunliner`, `surly-ecr`, `salsa-cutthroat`, `salsa-fargo`,
`surly-krampus`, `sklar-pbj`, `otso-fenrir-stainless`, `seven-custom`,
`seven-expedition-926d`, `seven-expedition-926f`, and `esker-hayduke`.

## Removed placeholders (Q137)

`rigid_483`, `rigid_485`, `rigid_490`, `rigid_500`, and `rigid_510` were
illustrative forks with no manufacturer. They are gone from the fork catalog
and from the fork lists of `binary-havok`, `binary-kinetik`, `revel-el-jefe`,
`ritchey-ultra`, and `sour-pasta-party`. Each frame's `BasisFork` is unchanged.
