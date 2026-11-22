# Legacy frame notes

These are the `Notes` strings of the legacy frames in `internal/frames`, unchanged since the port commit `9942803` and carried over in Slice 17b (Q145) before Slice 18a deletes legacy. They are history, not current provenance: where a section disagrees with the frame's Go file, its source record (`frame_<id>.md`), or a ruling in `docs/reboot/DESIGN.md`, those win. When a frame is re-ingested (Q127), move its section into its source record and delete it here. The Ritchey Ultra is left out because Slice 18b deletes it (Q125).

The text is verbatim except where `make boundary` bans a word (Q121). Each replacement is followed by `[edited]`, and the original wording is in git history at `9942803`. Fork IDs and field names (for example `enve_mountain_472`, `GeoHeadsetSpacerMm`, `HeadsetCupType`) are legacy names and may no longer exist.

## binary-havok

Binary Havok, from `internal/frames/binary_havok.go`.

Titanium 29+ drop-bar endurance & adventure bike frame designed around a 485mm axle-to-crown rigid fork or 100mm suspension fork with an EC44 external lower headset cup. Features rocker sliding dropouts (445-460mm chainstay, forward 445mm position in published geometry).

## binary-kinetik

Binary Kinetik, from `internal/frames/binary_kinetik.go`.

Butted 3Al-2.5V (Grade 9) titanium expedition hardtail frame designed around a 120mm suspension fork (530mm static A2C at 15% sag = 512mm published basis A2C [edited]) or 100-120mm travel (100mm recommended for S & SMD) / ~500mm rigid fork with a ZS44/EC44 headset (EC44 lower cup). Features rocker-style adjustable dropouts (455-472mm chainstay, forward 455mm position in published geometry), clearance for up to 29x3.0" or 27.5x3.0" tires, 12x148mm Boost spacing, 73mm BSA threaded BB, 31.6mm seatpost with internal dropper routing, and 9 accessory mount locations plus rack and fender mounts.

## esker-hayduke

Esker Hayduke, from `internal/frames/esker_hayduke.go`.

Custom seamless, quadruple-butted 4130 Chromoly adventure and bikepacking hardtail designed around a 120mm suspension fork at 30% sag (495mm A2C) or a 495mm rigid fork (Esker Fork Yeah!) with an EC44 lower headset cup (12mm stack). Features Portage adjustable dropouts (440-454mm chainstay, forward 440mm position in published geometry).

## fairlight-holt-2

Fairlight Holt 2.0, from `internal/frames/fairlight_holt_2.go`.

Custom-formed Reynolds 853 steel 29er XC/downcountry hardtail designed around a 100-120mm suspension fork (published baseline: RockShox SID 120mm with 20% sag = 507mm sagged / 531mm static A2C, 44mm offset) and an EC44 lower headset cup (ZS44/28.6 | EC44/40 with 12mm basis spacer [edited]). Compatible with Shimano and SRAM 1x MTB groupsets (single ring only, 55mm chainline max 38T, 52mm chainline max 32T). Features T47 73mm threaded BB, 30.9mm seatpost with Hope 33.4mm custom clamp, 148x12mm Boost rear spacing, post-mount 160mm native rear rotor (180mm with adaptor), and 29" native wheel compatibility.

## moots-mountaineer

Moots Mountaineer, from `internal/frames/moots_mountaineer.go`.

Handcrafted titanium micro-suspension trail mountain bike featuring Moots' signature YBB micro-suspension unit at the rear and designed around a 120mm suspension fork. Features oversized seamless 3Al-2.5V titanium with proprietary double-butted tubing, 44mm straight head tube (Chris King InSet 7, ZS44/EC44), 73mm English threaded BB, 30.9mm seatpost with stealth dropper routing, SRAM UDH dropouts (12x148mm Boost spacing), clearance for up to 29x2.6" tires, and 3 water bottle locations. Published geometry is based on a 120mm fork (baseline 531mm static A2C, 44mm offset) paired with an EC44 lower headset cup. Wheelbase numbers are derived from horizontal rear center plus front center as Moots does not publish wheelbase.

## otso-fenrir-stainless

Otso Fenrir Stainless, from `internal/frames/otso_fenrir_stainless.go`.

Stainless steel bikepacking & adventure frame designed around the 495mm Lithic carbon rigid fork (54mm offset, rear Tuning Chip 450mm chainstay / 72mm BB drop position) or 100-120mm suspension forks with an EC44 external lower headset cup. Lower-cup provenance (C4): the published geometry chart closes to the bare head tube with GeoHeadsetSpacerMm: 0 (BasisLowerCup stack [edited] = 0) around the 495mm fork A2C, while a physical tapered-steerer build uses an external EC44 lower headset cup (HeadsetCupType: CupEC44, 12mm physical stack) that adds 12mm to the front-end structural length relative to the bare-headtube chart (pending manufacturer confirmation of whether the basis chart [edited] omitted the lower cup).

Superseded: this describes the `ea9d423` chart that Q126 found wasn't Otso's. The current chart and its evidence are in `frame_otso_fenrir_stainless.md` (Q140–Q143).

## revel-el-jefe

Revel El Jefe, from `internal/frames/revel_el_jefe.go`.

Cold-formed 3Al-2.5V titanium 29er trail and bikepacking hardtail with CNC-machined 6Al-4V titanium headtube, bottom bracket, and dropouts. Designed around a 120mm suspension fork (120-130mm travel supported; stock RockShox SID Ultimate 120mm, 530mm static A2C at 20% sag = 506mm sagged A2C, 44mm offset, listed nominally as 510mm in the geometry chart) or a 510mm rigid fork with an integrated IS42/IS52 headset (0mm lower cup stack). Features sliding SRAM UDH dropouts with 420-435mm chainstay adjustability (published wheelbases match a 425mm 3D chainstay / 420mm horizontal rear center position), 73mm BSA threaded BB, 31.6mm seatpost with internal cable routing (34.9mm collar), clearance for 29x2.6" tires, 36T max chainring (55mm chainline), 160mm post-mount rear brake (180mm max), and 3 bottle boss sets on S (4 sets on M-XL).

## salsa-cutthroat

Salsa Cutthroat, from `internal/frames/salsa_cutthroat.go`.

Carbon drop-bar ultra-endurance and bikepacking frame with Class 5 VRS (Vibration Reduction System), designed around the Cutthroat Carbon rigid fork (483mm A2C, 51mm offset) or 100mm suspension forks with an integrated IS41/IS52 headset (FSA Orbit ITA). Published basis geometry [edited] is drawn around a 100mm suspension fork (506mm static A2C) at 25% sag (481mm effective A2C). Clearance for up to 29x2.4" tires.

## salsa-fargo

Salsa Fargo, from `internal/frames/salsa_fargo.go`.

Triple-butted seamless CroMoly steel drop-bar bikepacking and off-road exploration frame designed around the Cutthroat Carbon Deluxe rigid fork (483mm A2C, 51mm offset) or 100mm suspension forks with a ZS44/EC44 headset (FSA Orbit ITA). Features Alternator 1.5 dropouts (445-462mm chainstay length) and clearance for up to 29x2.4" (or 29x3.0" on sizes S-XL). Lower-cup provenance (C4): the published geometry chart closes to the bare head tube with GeoHeadsetSpacerMm: 0 (BasisLowerCup stack [edited] = 0) around the 483mm rigid fork A2C, while a physical tapered-steerer fork build uses the EC44 lower headset cup (HeadsetCupType: CupEC44, 12mm physical stack; straight 1-1/8" steerer builds use ZS44 0mm lower crown race adapter), adding 12mm of physical lower-cup stack relative to the bare-headtube chart when built with a tapered steerer.

## seven-custom

Seven Custom, from `internal/frames/seven_custom.go`.

Custom Straight Gauge Titanium 29er mountain frame built by Seven Cycles for Matt Armstrong (the fit studio [edited], fit technician Neil Doshi, order 2026-10-30, design-drawing preview [edited] 2026-11-11). Configured for 29" wheels (optimal 2.4" tire, max 2.6"), Boost 148mm Asymmetric UDH rear spacing, 73mm BSA bottom bracket, 44mm head tube (EC44 lower cup for tapered steerer; GeoHeadsetSpacerMm: 11 models the lower EC44 external headset cup stack in the basis geometry chart [edited]), 30.9mm seatpost with internal dropper port, and ENVE Mountain Fork Boost (52mm rake). Stack (631mm), Reach (424mm), and Wheelbase (1136.2mm) are derived from the 1:5 scale frame design drawing [edited] and verified against the published tube lengths (HTL 103mm, ETT 618mm, CS 463mm, ST 425mm C-C), angles (HTA 70.0°, STA 73.0°), BB drop (79mm), standover (814mm), estimated front center (679mm). Fit provenance: Seven's fit sheet specifies 746mm saddle height, 76mm saddle-nose setback, 245mm saddle length, 37mm saddle-top-to-stem-center drop, and 803mm saddle-tail-to-bar reach. Using the sheet's saddle length horizontally gives X_tail = -321mm and StemClampX = 482mm. The fit-sheet reconstruction puts StemClampY at about 676mm. Matt measured his SQlab at 280mm tip to tail, 35mm longer than the length on the sheet. Using its inferred W70 station and the idmatch BRP with level rails gives modeled X_TIP = -99mm and X_TAIL = -379mm. The discrepancy is unresolved. The idmatch transfer used the same physical saddle and the source reading was taken with the Redshift post unsagged. Note that (481, 726) (700mm from BRP, +13mm above BRP; previously noted as HX 481mm, HY 727mm during AI ingestion) comes from the idmatch report: subtracting 50mm stated bar rise with no backsweep offset yields Seven's stem-center target (~(481, 676)), and a 70mm -6° stem with 0mm spacers and ~11mm upper headset cover places StemClamp at (481.3, 677.1), whereas with the Tumbleweed Persuader 20° bar (-72.8mm backsweep, +45.5mm grip rise) HandGrip sits at (412.3, 712.2) (631.3mm from BRP, matching the rider's Otso Fenrir L with 100mm stem within ~6mm).

## seven-expedition-926d

Seven Expedition 926d, from `internal/frames/seven_expedition_926d.go`.

Handcrafted titanium bikepacking & adventure mountain frame designed for drop handlebars ('d'), 29" wheels with 2.0-2.6" tires, and 100/142mm or Boost 110/148mm spacing. Seven published Rider-Ready geometry across two fit profiles in 12 sizes (Standard Fit 'S' and Endurance Fit 'E', for 24 total sizes). The published geometry was mathematically modeled around [edited] the 472mm A2C ENVE Mountain Carbon fork (specified at 5.5cm / 55mm rake per Seven's specification note: "Optimized for 29 x 2.6" tires at 5.5 cm rake setting") paired with an EC44 external lower headset cup (10mm basis spacer [edited]), yielding ~482mm structural A2C (satisfied within ≤0.6mm across all 24 sizes with zero A2C slop required). While complete builds feature the ENVE Mountain Carbon Boost fork (490/492mm A2C) or 100mm suspension forks, running those taller forks kinematically pitches the frame around the rear axle (e.g. 492mm Boost fork raises stack by ~7mm, shortens reach by ~11mm, slackens HTA by ~1.0°, and reduces BB drop by ~8mm). Wheelbases are derived from horizontal rear center plus front center.

Basis-fork analysis: `legacy_seven_expedition_926d.md`.

## seven-expedition-926f

Seven Expedition 926f, from `internal/frames/seven_expedition_926f.go`.

Handcrafted titanium bikepacking & adventure mountain frame designed for flat handlebars ('f'), 29" wheels with 2.0-2.6" tires, and Boost 110/148mm spacing. Seven published Rider-Ready geometry across two fit profiles in 12 sizes (Standard Fit 'S' and Endurance Fit 'E', for 24 total sizes). Seven's geometry table published 56.3cm (563mm) chainstays for Endurance Fit, an obvious 100mm digit error for 46.3cm (463mm) which matches the Standard Fit chainstay length. The published geometry was mathematically modeled around [edited] the 472mm A2C ENVE Mountain Carbon fork (52mm extended rake position, specified at 55mm / 5.5cm rake per Seven's specification note) paired with an EC44 external lower headset cup (10mm basis spacer [edited]), yielding ~482mm structural A2C (satisfied within ≤0.6mm across all 24 sizes with zero A2C slop required). While complete builds feature the ENVE Mountain Carbon Boost fork (490/492mm A2C) or 100-120mm suspension forks, running those taller forks kinematically pitches the frame around the rear axle (e.g. 492mm Boost fork raises stack by ~9mm, shortens reach by ~6mm, and slackens HTA by ~0.8°). Wheelbases are derived from horizontal rear center plus front center.

Basis-fork analysis: `legacy_seven_expedition_926f.md`.

## sklar-pbj

Sklar PBJ, from `internal/frames/sklar_pbj.go`.

Double butted heat-treated Chromoly steel all-terrain bike (ATB) / rigid mountain touring frame designed around a 485mm A2C rigid chromoly steel fork (52mm offset, 110x15mm Boost spacing, 180mm post mount, 1-1/8" straight steerer with 30mm crown race, internal dynamo routing, cargo cage and rack/fender mounts). Headtube is EC34/EC44 (EC44 lower cup with 12mm basis spacer [edited]). Suspension-corrected for 100mm suspension forks and officially compatible with up to 120mm suspension forks ('Singlespeed + 120mm Suspension' build tested and confirmed by builder Adam Sklar). Features sliding modular dropouts offering 20mm of rear chainstay adjustment (432-452mm range) compatible with singlespeed, standard geared, and SRAM Transmission (UDH insert kit). Clearance for 29x2.6" tires (clears most 2.8"), 73mm BSA threaded BB, 148x12mm Boost rear spacing (52mm chainline), 160mm post-mount rear brake insert, up to 36T chainring, 31.6mm seatpost with stealth dropper routing (34.9mm clamp included), and 3 bottle cage positions (two in main triangle, one under downtube). Published Medium wheelbase is corrected from 1165mm to 1156mm to resolve an errant digit transposition in published geometry (730mm front center + 427mm rear center = 1157mm).

## sour-pasta-party

Sour Pasta Party, from `internal/frames/sour_pasta_party.go`.

Handmade CrMo steel 29er trail/adventure hardtail designed in Dresden, Germany around 100-120mm suspension forks (published with a 120mm fork at 15% sag and 12mm EC44 lower headset cup) or 490mm rigid forks (Sour Party Fork / Sour Carbon 29"). Wheelbases adapted to modern archetypical 44mm offset fork.

Audit (Slice 17b, Q146): "adapted" means these wheelbases may not be Sour's printed values. The catalog still labels them published and checks chart consistency against them until the frame is re-ingested.

## surly-ecr

Surly ECR, from `internal/frames/surly_ecr.go`.

4130 Chromoly steel 'Natch tubing 29+ rigid adventure and bikepacking bike designed around a 468mm rigid fork (43mm offset) with horizontal slotted dropouts and Gnot-Boost 145mm rear spacing. Clearance for 29x3.0" tires with or without fenders. Traditional Ø34mm head tube (EC34/28.6 upper, EC34/30 lower). Fixed rigid fork only; not compatible with suspension forks.

## surly-krampus

Surly Krampus, from `internal/frames/surly_krampus.go`.

4130 Chromoly steel 'Natch tubing 29+ trail hardtail designed around a 483mm rigid fork (47mm offset) or 100-120mm suspension forks (max 550x51mm) with a ZS44/EC44 headset (EC44 lower cup). Features 'Gnot-Boost' 145mm rear spacing (12x142 or 12x148mm) and clearance for 29x3.0" tires.

## tumbleweed-sunliner

Tumbleweed Sunliner, from `internal/frames/tumbleweed_sunliner.go`.

Size-specific heat-treated Chromoly rigid touring & bikepacking bike designed around a 440mm unicrown steel fork (55mm offset) and 29x2.6" tires (clearance up to 29x3.0") with a 44mm headtube (ZS44/EC44). Fixed rigid fork only; not compatible with suspension forks.

## turner-venn-ti

Turner Venn Ti, from `internal/frames/turner_venn_ti.go`.

Custom shaped 3Al-2.5V titanium trail & all-terrain hardtail featuring double-butted top and down tubes, tapered/butted chainstays, and a 3D-printed titanium lower chainstay yoke with integrated bridge. Designed around a 130mm suspension fork (120-140mm supported; baseline 541mm static A2C at 0% sag, 44mm offset) with a semi-integrated ZS44/ZS56 headset (0mm basis spacer [edited] stack). Features SRAM UDH / T-Type transmission compatibility, T47 (92mm) threaded BB, 30.9mm seatpost with internal dropper routing, clearance for 29x2.8" tires, post-mount rear brake up to 200mm, and max 34t chainring (52-55mm chainline). Turner published geometry unweighted (0% sag); size XL published stack of 670mm is an apparent transcription error for 690mm given the 160mm head tube length, 1224mm wheelbase, and 680mm effective top tube.
