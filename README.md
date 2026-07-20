# quanta

Units of measure for Go — dimensional analysis at run time, with conversion
factors held as **exact rationals** rather than pre-rounded floats.

Every units library can tell you a metre is not a second. The interesting part
is what happens next: whether an inch is `0.0254` or exactly `254/10000`,
whether `°C` can sneak into a product and quietly produce nonsense, and whether
you can add a unit the author never thought of. `quanta` takes those three
questions seriously — conversions run in `math/big.Rat` and round to `float64`
once, affine units are structurally prevented from entering an expression, and
the unit table is a registry you extend, not an enum you're stuck with.

- **Exact conversion factors, lossless conversion.** An inch is `254/10000` m,
  a pound is `45359237/100000000` kg, a US gallon is 231 in³. A quantity
  carries its magnitude as an exact rational, so a mile converted to feet to
  nautical miles to metres and back is bit-for-bit the value you started with.
- **Affine units that can't misbehave.** `0 °C == 273.15 K == 32 °F`, and
  `°C·m`, `°C²` and `k°C` are all errors rather than silent absurdities.
  Temperature *differences* get their own linear units (`Δ°C`).
- **A registry, not an enum.** Define your own units — including your own base
  dimensions — and parse expressions against them: `USD/tonne` works because
  you defined `USD`.
- **Compound expression parsing.** `m/s^2`, `kW·h`, `mg/dL`, `J/(kg·K)`,
  `1/s`, `m⁻²`, with SI prefixes on any unit and binary prefixes on `B`/`bit`.
- **Typed errors.** `ErrDimensionMismatch`, `ErrAffineUnit`, `ErrUnknownUnit`,
  `ErrDuplicateUnit`, `ErrSyntax`, `ErrExponentRange`, plus a `*ParseError`
  with the offending input and byte offset.
- **Zero dependencies.** Go 1.23+.

```go
import "github.com/zkrebbekx/quanta"
```

## Example

```go
si := quanta.SI()

d, _ := si.Parse("26.2 mi")
km, _ := d.In(si.MustUnit("km"))
fmt.Println(km.Format(3)) // 42.165 km

speed, _ := si.Parse("100 km/h")
time, _ := km.Div(speed)
fmt.Println(time.MustIn(si.MustUnit("min")).Format(1)) // 25.3 min

_, err := d.Add(si.MustParse("3 s"))
fmt.Println(errors.Is(err, quanta.ErrDimensionMismatch)) // true
```

## Exactness

A `Quantity` holds two things: the `float64` you read back with `Value()`, and
the exact magnitude as a `big.Rat` in coherent SI units. `In()` carries the
rational through untouched and re-renders the `float64` in the new unit —
the conversion

```
value_to = (exact_si - offset_to) / factor_to
```

runs entirely in `math/big.Rat`. That is the whole precision story, and it is
worth being precise about what it does and does not buy you:

- **You get lossless conversion, and losslessness composes.** A hundred
  conversions carry no more error than one, and there-and-back always returns
  the original `float64` — the test suite checks this over randomised values
  through five awkward unit chains, and none differ. This is stronger than the
  obvious approach of rounding correctly at each hop, which still drifts by an
  ulp about one time in ten. Multiplying pre-rounded `float64` factors, the
  usual approach, is worse again.
- **You do not get exact magnitudes.** Values are `float64`, and arithmetic on
  magnitudes (`Add`, `Mul`, …) is ordinary `float64` arithmetic with ordinary
  `float64` rounding — it drops the exact rational, keeping only the operand
  conversions exact. Conversions are lossless; arithmetic is `float64`.

## Dimensions

`Dimension` is a comparable `[7]int8` of the SI base-quantity exponents, so a
dimension check is one machine comparison and a `Dimension` works as a map key.

```go
acceleration := quanta.Length.Div(quanta.Time.Pow(2))
fmt.Println(acceleration)              // L·T⁻²
fmt.Println(acceleration == quanta.Length) // false
```

Exponents are `int8`. `Quantity.Mul`, `Div` and `Pow` return `ErrExponentRange`
if an exponent would leave `[-128, 127]`; the bare `Dimension` methods can't
return an error, so they saturate — use `CheckedMul`, `CheckedDiv`,
`CheckedPow` when you need to know.

## Affine units

`°C` and `°F` have a zero point, which makes them a different kind of thing
from a scale factor. `quanta` converts them correctly and then confines them:
they may not be multiplied, divided, raised to a power, prefixed, or written
into a compound expression, and each of those returns `ErrAffineUnit`.

```go
c := quanta.New(0, si.MustUnit("°C"))
fmt.Println(c.MustIn(si.MustUnit("K")))  // 273.15 K
fmt.Println(c.MustIn(si.MustUnit("°F"))) // 32 °F

_, err := si.ParseUnit("°C/m") // ErrAffineUnit
```

Differences of temperature are their own units. `Unit.Delta()` turns `°C` into
`Δ°C`, and `Sub` uses it automatically:

```go
hot, cold := si.MustParse("30 °C"), si.MustParse("10 °C")
diff, _ := hot.Sub(cold)
fmt.Println(diff) // 20 Δ°C

warmer, _ := cold.Add(si.MustParse("5 Δ°C"))
fmt.Println(warmer) // 15 °C
```

Adding two absolute temperatures is an error, because it means nothing.

## Registries and parsing

`SI()` returns a fresh registry with the SI base and derived units plus the
common non-SI ones (imperial and US customary lengths, masses and volumes;
minute/hour/day; bar, atmosphere, calorie, watt-hour, electronvolt; byte and
bit with binary prefixes). Extend it, or start from `NewRegistry()` and define
everything yourself — including new base dimensions.

```go
si := quanta.SI()
usd, _ := si.DefineBase("US dollar", "USD", quanta.Dimension{})
price, _ := si.Parse("640 USD/tonne")
```

Parsing rules worth knowing:

- **An exact symbol always beats a prefixed reading.** `min` is the minute, not
  a milli-inch; `cd` is the candela, not a centi-day.
- **The kilogram is handled properly.** `kg` is the coherent base unit and is
  not itself prefixable; the gram carries the prefixes, so `mg` is 10⁻⁶ kg.
- **`·` and `/` bind equally and associate left**, so `a/b/c` is `(a/b)/c` and
  `J/kg·K` is `(J/kg)·K`. Write `J/(kg·K)` if that's what you meant.
- **Binary prefixes are restricted** to units that opt in — `B` and `bit`. There
  is no `Mim`.
- Failures are a `*ParseError` with `Input` and `Offset`, wrapping a sentinel.

## Non-goals

- **No contexts.** No domain-specific conversions that hold only under an
  assumption (spectroscopy's wavelength/frequency trade, for instance).
- **No uncertainty propagation.** Magnitudes carry no error bars.
- **No currency rates.** Define a currency unit if you like; `quanta` ships no
  exchange rates.
- **No exact magnitudes.** Only factors are rational — see *Exactness* above.
- **No `UnmarshalText`.** Decoding needs a registry to resolve the symbol and a
  method has nowhere to get one, so `Quantity` implements `MarshalText` only.
  Decode with `Registry.Parse`, which makes the registry explicit.

## License

MIT
