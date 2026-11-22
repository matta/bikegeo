# Slice 16c: the fork model

Status: Blocked 2026-11-22 on planning against committed code. The design below is settled, but the exact types, callers, commit split, and probe values aren't written yet. Don't implement it until this line says Planned.

## Goal

Catalog forks own their variants, and travel becomes a closed table. Frames still offer only the forks they list, so this slice's effect can be measured on its own. Rankings move only through the deleted forks, the variants each listed fork now brings, and the new sags. Slice 16d then opens fork choice to every compatible fork (DESIGN.md Milestones).

## Rulings

- **Q128:** forks own their variants, and a fork model is offered in all its variants or none. The basis stays exact.
- **Q129:** travels are a closed table (100, 120, and 130 mm, each with its sags), and the SID 110, Pike 140, and Fox 36 150 are deleted. Fork classes are Slice 16d's.

## Design

### Fork variants

A fork variant is one A2C and offset that a fork model comes in (Q128). Each `catalog.Fork` lists its variants, and a frame's fork list names fork models only. Per-frame A2C and rake on frame entries go away (Q66's per-frame variants are superseded).

- **Variant data** comes from legacy `ForkCatalogEntry.Options`, labelled `catalog`. Where a source record holds the maker's numbers, those replace legacy's and are labelled published: the Lithic Mountain at 495/44 and 498/51.5 (Fenrir record S6), and the ENVE Mountain Boost at 490/44 and 492/52 (S5). The ENVE's 492/55 option isn't in S5, so it is dropped. This is a proposed default; Matt can veto it on plan review.
- **Planning checks every frame entry** against its fork's variants and lists each frame whose offered forks change (for example a frame that listed the SID 100 at 506/44 now also gets 506/51).

### Travel table

A closed type replaces `Fork.TravelMm float64`: rigid, plus one constant per row. An exhaustive switch returns each row's sags, replacing `catalog.SuspensionSagPcts` (Q31). Adding a travel means a new constant and a deliberate sag range, and the `exhaustive` check fails until every switch handles it.

| Travel | Sag range | Sags tried |
| --- | --- | --- |
| 100 mm | 15–20% | 15, 20 |
| 120 mm | 15–25% | 15, 20, 25 |
| 130 mm | 20–25% | 20, 25 |

### Deleted forks

The SID 110, Pike 140, and Fox 36 150 are deleted from the catalog, and the Pike 140 comes off the Turner Venn Ti. The Fox 34 130 stays at 541/44.

### Sag score

Matt prefers the plushest sag, as a minor preference only. The fork sag score (today `scoreRange(sag, 20, 20, 10, 30)`, weight 3 of 36) becomes 100 at the travel's highest sag, falling linearly to a small loss at its lowest. The weight stays 3, so sag only separates builds that are otherwise close. The plan sets the exact loss.

### Parity tests

This slice changes forks from legacy on purpose: forks are deleted, variants move onto fork models and some are corrected, and sags change. A parity check that lists every intended difference as an exception tests nothing. So the slice deletes the fork assertions from `TestFramesMatchLegacy` and deletes `TestForksMatchLegacy`, and catalog-owned tests pin each frame's fork list and each fork's variants instead. The rest of the frame parity (charts, headsets, insertion) stays until Slice 18a, so it still catches accidental changes to non-fork data.

## Left to planning

Planning reads the committed code and adds the exact types and identifiers, every caller (`catalog`, `query`, `compare`, `web`, `cmd/bikegeo-reboot`), how a URL names a fork and its variant, the commit split, each test with probe values (including every ranking and readout the variants and sags move), the rules that bite, and the out-of-scope list (fork classes, the cup catalog, and open fork choice are Slice 16d's). Then it changes the Status line to Planned.
