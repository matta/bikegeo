# Typical lower headset cups

Source record for `TypicalLowerCup` and `LowerCupFor` in `lower_cups.go`
(Q72, Q136).

## Cane Creek 40-Series stacks

The catalog models one typical lower cup per SHIS standard, using the Cane
Creek 40-Series lower cup as the reference part. The `/30` and `/40` columns
are the 1-1/8 in straight and 1.5 in tapered conversion rings. They share one
lower-cup stack height, so the catalog keeps one stack per standard.

| Standard | Bore | Rings | Stack |
| --- | --- | --- | ---: |
| `EC34` | 34 mm pressed | `/30` | 12 mm |
| `EC44` | 44 mm pressed | `/30`, `/40` | 12 mm |
| `ZS44` | 44 mm pressed | `/30` | 4 mm |
| `ZS56` | 56 mm pressed | `/30`, `/40` | 4 mm |
| `IS42` | 42 mm integrated | `/30` | 1 mm |
| `IS52` | 52 mm integrated | `/30`, `/40` | 1 mm |

## Steerer pairing

- `Bore34Pressed` and `Bore42Integrated` accept only the straight 1-1/8 in
  steerer.
- `Bore44Pressed` with a tapered steerer uses `EC44`. With a straight steerer
  it keeps `ZS44` when the frame's default cup is `ZS44`, and uses `EC44`
  otherwise.
- `Bore56Pressed` uses `ZS56` and `Bore52Integrated` uses `IS52`.

## Frame default stacks moved by Q136

Four frames carried a default stack that was not the typical stack. Their
`DefaultLowerCup.StackMm` moved to the typical value. Each frame's
`BasisLowerCup`, `BasisFork`, and chart numbers are unchanged.

| Frame | Standard | Old stack | New stack |
| --- | --- | ---: | ---: |
| `turner-venn-ti` | `ZS56` | 2 mm | 4 mm |
| `revel-el-jefe` | `IS52` | 0 mm | 1 mm |
| `ritchey-ultra` | `IS52` | 0 mm | 1 mm |
| `salsa-cutthroat` | `IS52` | 0 mm | 1 mm |
