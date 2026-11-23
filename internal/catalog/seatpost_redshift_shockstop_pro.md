# Redshift ShockStop PRO Endurance seatpost

Source record for `redshift_shockstop_pro`, gathered 2026-11-23. Matt identified his post as the PRO Endurance with coil springs and elastomers. Redshift now lists that configuration as the ShockStop PRO. The catalog models the 27.2 × 350 mm version. [Rider fit record](../../docs/RIDER_FIT.md) · [Redshift product page](https://redshiftsports.com/products/shockstop-pro-suspension-seatpost)

## Suspension and fit transfer

| Published quantity | Value |
| --- | --- |
| Total suspension travel | 35 mm |
| Rearward offset at rest | 7 mm |
| Rearward offset at 25% travel | 12 mm |
| Saddle pitch through travel | Constant with respect to the post’s mounting orientation |
| Spring preload | Rider-adjustable |

The two offsets differ by **5 mm**. The 25% figure identifies the state at which Redshift specifies the second offset. Actual seated sag depends on the rider and post setup. [Archived technical chart](sources/redshift_shockstop_pro_specs.png) · [Redshift linkage description](https://redshiftsports.com/products/shockstop-pro-suspension-seatpost) · [PRO instructions, revision 1](https://cdn.shopify.com/s/files/1/0089/1220/7951/files/Instructions_-_ShockStop_PRO_Seatpost_-_Rev_1.pdf?v=1775156766)

The chart gives neither the axis used to measure travel and rearward offset nor the vertical drop at 25% travel for a specified post angle. Multiplying 35 mm by 25% gives 8.75 mm of nominal travel. Converting that to a vertical BRP displacement requires the travel convention. The chart alone does not establish a BB-relative X/Y correction.

With the same saddle, installed pitch and bicycle attitude, the linkage translates BRP and the modeled SIT landmark together. A measured BB-relative displacement of either landmark between unloaded and seated states can be applied directly to BRP and reversed for a transfer in the other direction. Matt’s idmatch BRP was taken unloaded. His seated displacement remains unmeasured. [Rider fit record](../../docs/RIDER_FIT.md)

### What the setup manuals establish

| Model | Redshift’s setup guidance | Evidence for vertical drop |
| --- | --- | --- |
| Endurance, 35 mm, coil springs | To reproduce a rigid saddle position at the suggested 20% sag, start with the unloaded saddle 6 mm higher and 5 mm forward. The manual illustrates how to identify that sag by looking at the linkage with the rider seated. | About 6 mm down and 5 mm back at that setup. [Endurance instructions, rev. 6, p. 2](../../docs/sources/redshift_shockstop_endurance_rev6.pdf) |
| PRO Endurance, 35 mm, springs and elastomers | Redshift explains how to tune preload for rider weight, position and preference. Both editions omit a numerical saddle-height correction. | Using the Endurance figure for this model requires an assumption about shared linkage motion. [PRO Endurance instructions, rev. 0](https://cdn.shopify.com/s/files/1/0089/1220/7951/files/Instructions_-_ShockStop_PRO_Endurance_Seatpost_-_Rev_0.pdf?v=1712780228) · [PRO instructions, rev. 1](https://cdn.shopify.com/s/files/1/0089/1220/7951/files/Instructions_-_ShockStop_PRO_Seatpost_-_Rev_1.pdf?v=1775156766) |

### Geometry of the 20% setup

In the Endurance manual’s side views, the upper link points about 44° above the rearward horizontal when unloaded and about 33° at the illustrated 20% sag. These are approximate angles read between pivot centers on page 2. A fixed-length link swings about its forward pivot. The straight line between the two positions of its rear pivot (the chord) is perpendicular to the link at its mid-angle. The drawing therefore predicts motion about 38.5° rearward of vertical. For the manual’s 6 mm drop, `6 × tan(38.5°) ≈ 4.8 mm` rearward, consistent with its rounded 5 mm instruction. The drawing checks the direction independently. The manual supplies the magnitude. [Endurance manual, p. 2](../../docs/sources/redshift_shockstop_endurance_rev6.pdf)

For BRP transfer, bikegeo’s chosen reference is the complete 20% correction in BB-relative coordinates: unloaded to seated is approximately `(−5, −6) mm`, with X forward and Y up. Reverse the signs to set up an unloaded ShockStop from a rigid-post target. Applying the Endurance guidance to the 35 mm PRO Endurance assumes similar linkage motion at the reference installation. Both models have the same published nominal travel and offset specifications. The PRO manual gives preload guidance but no separate 20% correction. [Design ruling Q159](../../docs/DESIGN.md) · [Redshift model specifications](https://redshiftsports.com/products/shockstop-suspension-system) · [PRO manual](../../docs/sources/redshift_shockstop_pro_rev1.pdf)

## Dimensions and compatibility

Redshift’s clearance drawing gives the following dimensions along the post for both 27.2 mm versions. Letters refer to the arrows in the drawing. [Archived dimension drawing](sources/redshift_shockstop_pro_dimensions.png)

| Dimension | 280 mm post | 350 mm post |
| --- | ---: | ---: |
| Stack height (A) | 90 mm | 90 mm |
| Rails to minimum insertion mark (B) | 205 mm | 275 mm |
| Total length (C) | 280 mm | 350 mm |
| Minimum insertion (D) | 75 mm | 75 mm |
| Maximum insertion (E) | 190 mm | 260 mm |
| Mass on dimension drawing | 445 g | 502 g |
| Mass on technical chart | 439 g | 547 g |

The two manufacturer sources disagree on mass. The technical chart specifies 6061-T6 aluminum, a 110 kg rider limit, compatibility with 7 mm round and 7 × 9 mm oval rails, and ASTM F2043 Class 2 use with drops under 15 cm. The 350 mm version accepts a Di2 battery with the PRO kit. Redshift requires a shim at least 100 mm long when fitting a larger-diameter seat tube. [Archived technical chart](sources/redshift_shockstop_pro_specs.png) · [PRO instructions, p. 1](https://cdn.shopify.com/s/files/1/0089/1220/7951/files/Instructions_-_ShockStop_PRO_Seatpost_-_Rev_1.pdf?v=1775156766)

## Mapping to bikegeo

- The catalog uses one 12 mm setback value. Placement measures setback perpendicular to the shaft and has no suspension travel input. The older app treated the published 7 and 12 mm offsets as horizontal examples in `internal/ideal/readme.md` at commit `3f9e6c1`. The manufacturer chart does not resolve that axis convention. [Catalog entry](seatposts.go) · [Seatpost placement](../assembly/seatpost.go)
- The catalog uses 100 mm minimum insertion for the assembly with the 100 mm Cane Creek ST27316 shim. Its 250 mm maximum insertion comes from an illustrative 100 mm stack. Redshift’s drawing gives 90 mm stack and 260 mm maximum insertion for the 350 mm post. The catalog’s 30 mm clamp footprint is also illustrative. [Catalog entry](seatposts.go) · [Design rulings Q75c and Q78](../../docs/DESIGN.md)

## Archived sources

| Redshift original | Local copy | SHA-256 |
| --- | --- | --- |
| [Endurance installation manual, rev. 6](https://cdn.shopify.com/s/files/1/0089/1220/7951/files/Instructions_-_ShockStop_Seatpost_-_Rev_6.pdf?v=1704316459) | [Endurance manual](../../docs/sources/redshift_shockstop_endurance_rev6.pdf) | `31afa37ad605431770e89bfd348456b8a847efc09fcb691294530d2fa6477279` |
| [PRO installation manual, rev. 1](https://cdn.shopify.com/s/files/1/0089/1220/7951/files/Instructions_-_ShockStop_PRO_Seatpost_-_Rev_1.pdf?v=1775156766) | [PRO manual](../../docs/sources/redshift_shockstop_pro_rev1.pdf) | `540aff2042d59b34849830564535cf1ae548f6dbb1473b7bd4a67132d307eb6c` |
| [PRO technical specifications](https://cdn.shopify.com/s/files/1/0089/1220/7951/files/PRO_Seatpost_Tech_Specs_Chart_-_support_email.png?v=1795119300) | [Technical chart](sources/redshift_shockstop_pro_specs.png) | `0f6cc2540a07e9747325d13e0f3ec8fd8011179656900f870b4dddc15f7b7335` |
| [PRO Endurance clearance dimensions](https://cdn.shopify.com/s/files/1/0089/1220/7951/files/ShockStop_PRO_Endurance_Seatpost.png?v=1712852657) | [Dimension drawing](sources/redshift_shockstop_pro_dimensions.png) | `4047935d67dc20f48776375a1eda81d9496e21270831a8c85a49c8ff12290f62` |
