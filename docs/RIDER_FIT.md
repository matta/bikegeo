# Matt's Rider Fit: Facts and Measurement Provenance

This is the persistent record of Matt's fit history, confirmed directly by Matt on 2026-11-15, with saddle measurements clarified on 2026-11-16. Consult it before asking about his saddle, seatpost, fit measurements, or transfer procedure. The facts below are established history; the biomechanical suitability of the resulting position remains open to testing.

## Confirmed Equipment

- **Saddle:** SQlab 610 ERGOLUX® active 2.1, black, 14 cm. Order description: `SQLab 610 ERGOLUX® active 2.1 20NO × 1 900 black / 14cm`.
- **Same physical saddle during fitting:** The fitter moved Matt's saddle from his own bike to the test jig bike. This was not a substitution with a different saddle model.
- **Seatpost on Matt's bike:** Redshift ShockStop PRO Endurance, the suspension version using both coil springs and elastomers. Matt recalls that the current product may be called simply PRO; the spring-and-elastomer configuration is the identifying fact.
- **Setback adapter:** FAIR bicycle Drop Best compatible with the Redshift post. FAIR lists the corresponding [Drop Best UC M6 redshift](https://www.fairbicycle.com/product-page/drop-best-uc-m6-redshift) with 27 mm of nominal forward or rearward offset. This specification does not independently establish the exact change in Matt's BRP coordinates after installation and adjustment.

The confirmed saddle matches bikegeo's reference `sqlab-610-ergolux` / SQlab 610 Ergolux Active 2.1. An earlier conversational description of it as an Ergowave was superseded by the exact order description above.

The owner-measured geometry of that actual saddle is recorded in [SQLAB_SADDLE.md](SQLAB_SADDLE.md), including Matt's physically measured 10 mm SIT elevation above the W70 / BRP reference plane.

Redshift’s published offsets at rest and at 25% travel, the linkage’s effect on saddle pitch, and the catalog’s placement assumptions are documented in the [ShockStop PRO source record](../internal/catalog/seatpost_redshift_shockstop_pro.md).

## Confirmed idmatch Measurement and Transfer Procedure

Matt described the procedure as follows:

1. The fitter placed the idmatch saddle measurement device on the saddle mounted on Matt's own bike.
2. **The Redshift seatpost was unsagged (uncompressed) for that measurement.** The source saddle position was not measured with the post compressed under Matt's weight.
3. The fitter removed that same saddle from Matt's bike and installed it on the test jig bike.
4. The fitter used the same idmatch measurement device to place the saddle in the same measured location on the test jig bike.
5. The purpose was to assess Matt with the saddle in the location copied from his own bike. Matt reports the fitter's conclusion as, “yeah, I don't see a major problem with this fit”.

**The transfer reference was the unsagged source-bike saddle position.** Do not describe the source measurement as rider-loaded or assume that the transfer reproduced the saddle's position while the Redshift post was compressed during riding. No measured compression or numerical sag correction has been supplied. Do not invent one or silently alter the fit targets to account for it.

## Report Values and Their Status

Matt confirmed that the fit report's BRP and handlebar X/Y coordinates match the values already recorded in bikegeo:

| Quantity | Recorded value |
| --- | --- |
| BRP setback behind BB | 219 mm |
| BB-to-BRP radial saddle height | 746 mm |
| BRP coordinates, X forward / Y upward | Approximately (−219, 713.1) mm |
| Report handlebar coordinates (HandGrip) | Approximately (481, 726) mm |

Confirmed by Matt on 2026-11-20: `(481, 726) mm` represents the physical hand grip (`HandGrip`) location rather than the stem clamp. See [DESIGN.md](DESIGN.md) (coordinate and sign conventions, cockpit placement) for the coordinate definitions, and `DefaultGripTarget` in [`internal/catalog/catalog.go`](../internal/catalog/catalog.go) for the default target; the transfer procedure above separately establishes the unsagged saddle measurement state.

The report values document the tested setup; they do not establish that 219 mm is uniquely optimal or that nearby saddle positions were compared.

## Saddle Measurements and Derived SIT Coordinates

**SIT** denotes the physical saddle landmark at the front foot of the raised rear lip, on the middle seating shelf. It is a support reference, not an independently measured location of Matt's pelvis, sit bones or contact-pressure center. **W70** names the saddle-local longitudinal station of the physical BRP landmark; bicycle-relative BRP coordinates describe that landmark after installation. BRP is not a term reserved for bicycle-coordinate values.

The saddle specification records the following owner measurements and their derived geometry:

| Quantity | Value | Status |
| --- | --- | --- |
| TIP-to-SIT longitudinal distance | 200 mm | Owner-measured, corrected from 215 mm |
| W70-to-SIT longitudinal distance | 80 mm | Owner-reported measurement |
| SIT elevation above W70 / BRP reference plane | +10 mm | Physically measured by Matt; confirmed 2026-11-16 |
| W70 station behind TIP | 120 mm | Derived from 200 − 80 mm; identification as the actual 70 mm-width station awaits direct width verification |
| Rail center to W70 | +32 mm | Derived; positive forward along the rails |
| Rail center to SIT | −48 mm | Derived; negative rearward along the rails |

The canonical +32 mm rail-center-to-W70 value supersedes the earlier +23 mm external estimate. Their 9 mm disagreement is not a calibrated uncertainty interval. The measured longitudinal distances run parallel to the rails; they must not be applied perpendicular to the seat tube.

Under the **level-rail approximation**, the W70-to-SIT displacement is `(-80, +10) mm` in positive-forward / positive-upward coordinates. Applying that displacement to the reported BRP gives:

- Modeled SIT setback: `219 + 80 = 299 mm` behind BB.
- Modeled SIT coordinates: approximately `(-299, 723.1) mm`, from `(-219, 713.1) + (-80, +10) mm`.

This corrected derivation supersedes the earlier model-derived 292 mm setback based on a 73 mm span. It is not a new fit-report reading, an independent pelvic measurement or a demonstrated optimal position. The 10 mm elevation is measured; the level-rail installation orientation is an assumption. No numerical installed rail pitch has been supplied. [DESIGN.md](DESIGN.md) records the saddle and rail placement model, and `DefaultBrpTarget` in [`internal/catalog/catalog.go`](../internal/catalog/catalog.go) holds the default saddle target.

## Owner-Reported Riding History and Observations

- Matt rode across Washington with the saddle farther forward than its later Drop Best position.
- He installed the Drop Best in the rearward-offset orientation and lowered the saddle to preserve his intended BB-to-sit-bone distance. The exact before/after coordinates have not been supplied.
- Moving the saddle back made him feel **more solidly supported in the saddle, rather than falling forward**.
- That support made reaching farther forward feel more comfortable, prompting a stem change from **80 mm to 110 mm**. The longer-stem setup felt better again.
- The idmatch session subsequently assessed the copied position described above. Matt has limited ride time on the final setup and wants more scrutiny before basing a roughly $5,500 custom frame on it.
- Matt explicitly considers it possible that a smaller rearward saddle movement would also have worked. His preferred setback range remains untested; do not convert the current target into a claim of proven physiological necessity.
