// Package quanta is a units-of-measure library for Go: dimensional analysis at
// run time, with conversion factors held as exact rationals.
//
// A [Quantity] is a float64 magnitude paired with a [Unit]. A Unit knows its
// [Dimension] — the vector of the seven SI base-quantity exponents — and its
// exact conversion to the coherent SI unit of that dimension. Adding a length
// to a time is an error, not a silent absurdity; multiplying metres by metres
// gives m²; and converting a mile to metres and back returns the number you
// started with, bit for bit.
//
//	si := quanta.SI()
//	d, _ := si.Parse("26.2 mi")
//	km, _ := d.In(si.MustUnit("km"))
//	fmt.Println(km.Format(3)) // 42.165 km
//
// # Dimensional analysis
//
// [Dimension] is a comparable [7]int8 array, so dimension checks are a single
// machine comparison and a Dimension may be used as a map key. Every operation
// that combines quantities either verifies dimensions ([Quantity.Add],
// [Quantity.Sub], [Quantity.Cmp], [Quantity.In]) or derives the new one
// ([Quantity.Mul], [Quantity.Div], [Quantity.Pow]). Exponents live in an int8;
// operations that would push one outside [-128, 127] report [ErrExponentRange]
// rather than wrapping. The bare [Dimension.Mul], [Dimension.Div] and
// [Dimension.Pow] cannot return an error, so they saturate instead — use
// [Dimension.CheckedMul] and its siblings when you need to know.
//
// # Exact rational factors
//
// Conversion factors are [math/big.Rat] values, never floats. An inch is
// exactly 254/10000 metre; a pound is exactly 45359237/100000000 kilogram; a
// US gallon is exactly 231 cubic inches.
//
// A Quantity carries its magnitude as an exact rational in coherent SI units
// alongside the float64 you read with [Quantity.Value], and [Quantity.In]
// passes that rational through untouched — the float64 is only ever the
// rational rendered in the current unit. Conversion is therefore lossless, and
// losslessness composes: a chain of a hundred conversions has no more error
// than one, and converting there and back returns bit-for-bit the value you
// started with. That is a strictly stronger guarantee than rounding correctly
// at every hop, which still drifts by an ulp roughly one time in ten.
//
// What this does not do is make the magnitudes exact. Values are float64, and
// a float64 is what you get back. Arithmetic on magnitudes ([Quantity.Add],
// [Quantity.Mul] and friends) is ordinary float64 arithmetic, with ordinary
// float64 rounding, and it discards the exact rational — only the operand
// conversions it performs along the way are exact. The precise guarantee is:
// conversions are lossless, arithmetic is float64.
//
// # Affine units
//
// Degrees Celsius and Fahrenheit have a zero point of their own, which is why
// 0 °C is 273.15 K but 0 m/s is 0 mph. quanta models that with an exact
// rational offset, and then confines it. Affine units convert correctly in
// both directions, compare, and negate, but they may not be multiplied,
// divided, raised to a power, prefixed, or written into a compound unit
// expression; each of those returns an error wrapping [ErrAffineUnit]. This
// closes the trap where "°C·m" or "°C²" quietly produces a nonsense factor.
//
// Differences of temperature are a separate idea from temperatures, so they
// get separate units: [Unit.Delta] turns °C into Δ°C, the linear unit of
// temperature difference, and [SI] registers Δ°C and Δ°F alongside the kelvin.
// [Quantity.Sub] applies this automatically — 30 °C minus 10 °C is 20 Δ°C, and
// 20 °C plus 5 Δ°C is 25 °C — while adding two absolute temperatures is an
// error.
//
// # Registries
//
// There is no fixed enumeration of units. A [Registry] is a namespace of
// symbols that callers extend: [Registry.DefineBase] introduces a coherent
// unit (or an entirely new dimension of your own), [Registry.Define] scales an
// existing one by an exact rational, [Registry.DefineAffine] adds a zero
// point, and [Registry.Alias] adds spellings. Registering a symbol twice is
// [ErrDuplicateUnit], never a silent overwrite. [SI] returns a fresh registry
// preloaded with the SI base and derived units plus the common non-SI units;
// because it is fresh on each call, extending it cannot disturb anyone else.
//
// [Registry.ParseUnit] parses compound expressions — "m/s^2", "kW·h", "mg/dL",
// "J/(kg·K)", "1/s", "m⁻²" — including SI decimal prefixes on any prefixable
// unit and IEC binary prefixes on the information units. An exactly registered
// symbol always beats a prefixed reading, so "min" is the minute rather than a
// milli-inch. Multiplication and division bind equally and associate to the
// left, so "a/b/c" is (a/b)/c; parenthesise when that is not what you mean.
// Parse failures are a [*ParseError] carrying the input and a byte offset.
//
// # Immutability and concurrency
//
// Unit and Quantity are immutable values. Every method returns a new value and
// none mutate the receiver, so they are safe to copy and safe to share across
// goroutines. A Registry is guarded internally and is safe for concurrent
// definition and lookup. Accessors that would otherwise expose internal state
// ([Unit.Factor], [Unit.Offset]) return fresh copies.
//
// # Non-goals
//
// quanta deliberately leaves these out:
//
//   - No contexts. There is no mechanism for domain-specific conversions that
//     are only valid under an assumption, such as trading wavelength for
//     frequency in spectroscopy.
//   - No uncertainty. Magnitudes carry no error bars and nothing is
//     propagated.
//   - No currency rates. You can define currency units — nothing stops you
//     from a USD/tonne — but quanta ships no exchange rates and never will.
//   - No exact magnitudes. Only factors are rational; values are float64.
//   - No UnmarshalText. Decoding a quantity needs a registry to resolve the
//     symbol, and a method has nowhere to get one; use [Registry.Parse].
package quanta
