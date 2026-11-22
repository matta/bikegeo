# MY25 FOX 34 29" 15QRx110 — Provenance Record

Every dimension below was read directly from the official FOX Help Center 29" User Specification engineering drawing.

## Sources

- **Engineering drawing (URL):** `https://tech.ridefox.com/img/help/page3074-GSM8/MY25_Fox_34_29in_User_Spec.jpg` (FOX Help Center page `page3074`, fetched `2026-11-22`)
- **Archived drawing:** [my25_fox_34_29in_user_spec.jpg](sources/my25_fox_34_29in_user_spec.jpg)
- **SHA-256:** `ba2d292271ae67f93a74ef12f34806ff61c914eb23de63949348fd27a508fa36`

## Gathered Drawing Specifications

- **Model / Wheel / Axle:** MY25 FOX 34, 29", `15QRx110` Boost (`110 mm` spacing)
- **Steerer:** 1.5" Tapered (`Ø28.58 ±0.05 mm` upper, `Ø39.808 ±0.030 mm` crown race seat, `244 ±5 mm` uncut length)
- **Offset / Rake options:** `44.0 ±2.5 mm` and `51.0 ±2.5 mm`
- **Axle-to-crown by travel:**
  - `120 mm` travel: `534.0 mm` extended, `414.0 mm` compressed
  - `130 mm` travel: `544.0 mm` extended, `414.0 mm` compressed
  - `140 mm` travel: `554.0 mm` extended, `414.0 mm` compressed (not included in catalog; `Q129` closes Slice 16c travel to `{100, 120, 130}`)
- **Max tire clearance:** `29 × 2.6"` (`762 mm` peak diameter / `69.2 mm` max width; minimum `6.0 mm` compressed clearance required for tires wider than `2.4"`)

## Catalog Mapping

- **`fox34_120`** (`"Fox Float 34 120mm"`, `assembly.SteererTapered`, `Travel: assembly.Travel120`):
  - Variants: `{StaticA2cMm: 534, RakeMm: 44}`, `{StaticA2cMm: 534, RakeMm: 51}`
- **`fox34_130`** (`"Fox Float 34 130mm"`, `assembly.SteererTapered`, `Travel: assembly.Travel130`):
  - Variants: `{StaticA2cMm: 544, RakeMm: 44}`, `{StaticA2cMm: 544, RakeMm: 51}`
