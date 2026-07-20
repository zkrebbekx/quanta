package quanta

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
)

// Quantity is a float64 magnitude paired with a [Unit].
//
// Quantities are immutable values: every operation returns a new Quantity and
// none of them mutate the receiver, so a Quantity is safe to copy and safe for
// concurrent use.
//
// The zero Quantity is the dimensionless number zero.
type Quantity struct {
	value float64
	unit  Unit
	// si is the exact magnitude in the coherent SI unit of the dimension. It
	// is carried unchanged through conversions, which is what makes a chain of
	// conversions lossless: the float64 in value is only ever a rendering of
	// si in the current unit. A nil si means "derive it from value", which is
	// exact because every finite float64 is a rational.
	//
	// The invariant is value == float64(si), correctly rounded. Nothing ever
	// mutates the pointed-to value, so sharing it between quantities is safe.
	si *big.Rat
}

// New returns the quantity value·u.
func New(value float64, u Unit) Quantity { return Quantity{value: value, unit: u} }

// exactSI returns the exact magnitude in coherent SI units, deriving it from
// the float64 magnitude if no exact value is being carried. It returns nil for
// a non-finite magnitude, which has no rational representation. The result
// must not be mutated.
func (q Quantity) exactSI() *big.Rat {
	if q.si != nil {
		return q.si
	}
	if math.IsNaN(q.value) || math.IsInf(q.value, 0) {
		return nil
	}
	r := new(big.Rat).SetFloat64(q.value)
	r.Mul(r, q.unit.fac())
	if q.unit.offset != nil {
		r.Add(r, q.unit.offset)
	}
	return r
}

// Value returns the magnitude in the quantity's own unit.
func (q Quantity) Value() float64 { return q.value }

// Unit returns the quantity's unit.
func (q Quantity) Unit() Unit { return q.unit }

// Dimension returns the dimension of the quantity's unit.
func (q Quantity) Dimension() Dimension { return q.unit.dim }

// IsZero reports whether the magnitude is exactly zero. Note that this is a
// question about the magnitude in the quantity's own unit, which for an affine
// unit is not the same question as whether the quantity is at the origin of its
// dimension: 0 °C is not 0 K.
func (q Quantity) IsZero() bool { return q.value == 0 }

// convert rescales v from one unit to another, both assumed to share a
// dimension. The whole computation is carried out in exact rational arithmetic
// and rounded to float64 exactly once, at the end:
//
//	v_to = (v_from*factor_from + offset_from - offset_to) / factor_to
//
// Non-finite magnitudes cannot be represented as rationals, so they fall back
// to float64 arithmetic; NaN and ±Inf are preserved.
func convert(v float64, from, to Unit) float64 {
	if from.Equal(to) {
		return v
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		f, _ := from.fac().Float64()
		t, _ := to.fac().Float64()
		fo, _ := from.off().Float64()
		to64, _ := to.off().Float64()
		return (v*f + fo - to64) / t
	}
	r := new(big.Rat).SetFloat64(v)
	r.Mul(r, from.fac())
	if from.offset != nil {
		r.Add(r, from.offset)
	}
	if to.offset != nil {
		r.Sub(r, to.offset)
	}
	r.Quo(r, to.fac())
	out, _ := r.Float64()
	return out
}

// scale rescales a difference from one unit to another, ignoring both units'
// zero points. It is the right conversion for deltas.
func scale(v float64, from, to Unit) float64 {
	return convert(v, linear(from), linear(to))
}

// linear strips a unit's zero point without renaming it.
func linear(u Unit) Unit {
	u.offset = nil
	return u
}

// In converts the quantity to unit u, which must have the same dimension.
// Offsets are applied in both directions, so New(0, degC).In(kelvin) is
// 273.15 K and New(0, degC).In(degF) is 32 °F.
//
// Conversion is lossless. The exact magnitude is held as a rational and
// carried through unchanged; the float64 you read back with [Quantity.Value]
// is that rational rendered in the current unit. A conversion therefore never
// accumulates error, however long the chain, and converting there and back
// always returns bit-for-bit the value you started with:
//
//	q.MustIn(foot).MustIn(nauticalMile).MustIn(metre).MustIn(mile) == q
//
// It returns an error wrapping [ErrDimensionMismatch] if the dimensions differ.
func (q Quantity) In(u Unit) (Quantity, error) {
	if q.unit.dim != u.dim {
		return Quantity{}, fmt.Errorf("cannot convert %s to %s: %w",
			q.unit.dim, u.dim, ErrDimensionMismatch)
	}
	if q.unit.Equal(u) {
		return q, nil
	}
	e := q.exactSI()
	if e == nil { // non-finite magnitude: no rational to carry
		return Quantity{value: convert(q.value, q.unit, u), unit: u}, nil
	}
	r := new(big.Rat).Set(e)
	if u.offset != nil {
		r.Sub(r, u.offset)
	}
	r.Quo(r, u.fac())
	v, _ := r.Float64()
	return Quantity{value: v, unit: u, si: e}, nil
}

// MustIn is [Quantity.In] for cases where the conversion is known to be valid,
// such as package-level constants and tests. It panics on error.
func (q Quantity) MustIn(u Unit) Quantity {
	out, err := q.In(u)
	if err != nil {
		panic(err)
	}
	return out
}

// Add returns q+o, expressed in q's unit. Both quantities must have the same
// dimension. o is converted losslessly, but the addition itself is ordinary
// float64 arithmetic — see the package documentation on what is and is not
// exact.
//
// Affine units follow the rules of temperature arithmetic: an absolute
// temperature may not be added to anything, so o must be linear. If q itself is
// affine, o is treated as a difference and added to q's magnitude — 20 °C plus
// 5 Δ°C is 25 °C. Otherwise the operation is an ordinary conversion and sum.
func (q Quantity) Add(o Quantity) (Quantity, error) { return q.addSub(o, 1) }

// Sub returns q-o, expressed in q's unit, with one exception: subtracting one
// affine quantity from another yields a difference, so the result is expressed
// in q's delta unit (see [Unit.Delta]) — 30 °C minus 10 °C is 20 Δ°C, not
// 20 °C. Subtracting a linear quantity from an affine one shifts it and stays
// affine. Subtracting an affine quantity from a linear one is meaningless and
// returns an error wrapping [ErrAffineUnit].
func (q Quantity) Sub(o Quantity) (Quantity, error) { return q.addSub(o, -1) }

func (q Quantity) addSub(o Quantity, sign float64) (Quantity, error) {
	if q.unit.dim != o.unit.dim {
		op := "add"
		if sign < 0 {
			op = "subtract"
		}
		return Quantity{}, fmt.Errorf("cannot %s %s and %s: %w",
			op, q.unit.dim, o.unit.dim, ErrDimensionMismatch)
	}

	switch {
	case q.unit.IsAffine() && o.unit.IsAffine():
		if sign > 0 {
			return Quantity{}, fmt.Errorf("cannot add two absolute %s quantities (%s, %s): %w",
				q.unit.dim, q.unit.symbol, o.unit.symbol, ErrAffineUnit)
		}
		// Difference of two absolute values: a delta in q's scale.
		d := q.unit.Delta()
		return Quantity{value: q.value - convert(o.value, o.unit, q.unit), unit: d}, nil

	case q.unit.IsAffine():
		// Absolute ± difference: stays absolute, in q's unit.
		return Quantity{value: q.value + sign*scale(o.value, o.unit, q.unit), unit: q.unit}, nil

	case o.unit.IsAffine():
		verb := "add"
		if sign < 0 {
			verb = "subtract"
		}
		return Quantity{}, fmt.Errorf("cannot %s the absolute quantity %s from a %s value: %w",
			verb, o.unit.symbol, q.unit.symbol, ErrAffineUnit)

	default:
		return Quantity{value: q.value + sign*convert(o.value, o.unit, q.unit), unit: q.unit}, nil
	}
}

// Mul returns q·o. The dimensions multiply and the units compound, so metres
// times metres is m² and newtons times metres is N·m. Affine units may not be
// multiplied; doing so returns an error wrapping [ErrAffineUnit].
func (q Quantity) Mul(o Quantity) (Quantity, error) {
	u, err := combine(q.unit, o.unit, 1)
	if err != nil {
		return Quantity{}, fmt.Errorf("cannot multiply %s by %s: %w",
			describe(q.unit), describe(o.unit), err)
	}
	return Quantity{value: q.value * o.value, unit: u}, nil
}

// Div returns q/o. The dimensions divide and the units compound, so metres
// divided by seconds is m/s. Affine units may not be divided; doing so returns
// an error wrapping [ErrAffineUnit].
func (q Quantity) Div(o Quantity) (Quantity, error) {
	u, err := combine(q.unit, o.unit, -1)
	if err != nil {
		return Quantity{}, fmt.Errorf("cannot divide %s by %s: %w",
			describe(q.unit), describe(o.unit), err)
	}
	return Quantity{value: q.value / o.value, unit: u}, nil
}

// Pow returns q raised to the integer power n. Pow(0) is the dimensionless 1.
// Affine units may not be raised to a power; doing so returns an error
// wrapping [ErrAffineUnit].
func (q Quantity) Pow(n int) (Quantity, error) {
	u, err := powUnit(q.unit, n)
	if err != nil {
		return Quantity{}, fmt.Errorf("cannot raise %s to the power %d: %w",
			describe(q.unit), n, err)
	}
	return Quantity{value: math.Pow(q.value, float64(n)), unit: u}, nil
}

// describe names a unit for error messages, falling back to its dimension when
// the unit has no symbol.
func describe(u Unit) string {
	if u.symbol == "" {
		return "a dimensionless quantity"
	}
	return u.symbol
}

// Neg returns -q, in the same unit. For an affine unit this negates the
// magnitude on that unit's own scale: -q of 5 °C is -5 °C, not -278.15 K.
func (q Quantity) Neg() Quantity { return Quantity{value: -q.value, unit: q.unit} }

// Abs returns |q|, in the same unit, with the same caveat about affine scales
// as [Quantity.Neg].
func (q Quantity) Abs() Quantity { return Quantity{value: math.Abs(q.value), unit: q.unit} }

// Cmp compares two quantities of the same dimension, returning -1 if q sorts
// before o, 0 if they are equal and +1 if q sorts after o.
//
// o is converted losslessly into q's unit and the two float64 magnitudes are
// then compared, so 1 km compares greater than 999 m and 0 °C compares equal
// to 273.15 K. Comparison is deliberately done at float64 precision in q's
// unit rather than on the exact rationals: the latter would make 273.15 K
// differ from 0 °C by the ulp that float64 cannot represent, which is true but
// useless.
//
// It returns an error wrapping [ErrDimensionMismatch] if the dimensions differ.
// If either magnitude is NaN the comparison is not meaningful and the result is
// 0 with no error, matching the behaviour of float64 comparison operators.
func (q Quantity) Cmp(o Quantity) (int, error) {
	if q.unit.dim != o.unit.dim {
		return 0, fmt.Errorf("cannot compare %s with %s: %w",
			q.unit.dim, o.unit.dim, ErrDimensionMismatch)
	}
	b := convert(o.value, o.unit, q.unit)
	switch {
	case q.value < b:
		return -1, nil
	case q.value > b:
		return 1, nil
	default:
		return 0, nil
	}
}

// Equal reports whether two quantities describe the same physical value. It is
// dimension-aware: 1000 m equals 1 km, and a length is never equal to a time.
// Comparing across dimensions is false rather than an error.
func (q Quantity) Equal(o Quantity) bool {
	if q.unit.dim != o.unit.dim {
		return false
	}
	return q.value == convert(o.value, o.unit, q.unit)
}

// String renders the quantity as its magnitude and symbol, for example
// "9.81 m/s²". The magnitude uses the shortest representation that round-trips
// through [strconv.ParseFloat]. A dimensionless quantity renders as the bare
// number.
func (q Quantity) String() string {
	n := strconv.FormatFloat(q.value, 'g', -1, 64)
	if q.unit.symbol == "" {
		return n
	}
	return n + " " + q.unit.symbol
}

// Format renders the quantity with exactly prec digits after the decimal point,
// for example Format(2) on 9.80665 m/s² gives "9.81 m/s²". A negative prec
// means "as many digits as needed", as in [Quantity.String].
func (q Quantity) Format(prec int) string {
	n := strconv.FormatFloat(q.value, 'f', prec, 64)
	if q.unit.symbol == "" {
		return n
	}
	return n + " " + q.unit.symbol
}

// MarshalText implements [encoding.TextMarshaler], producing the same text as
// [Quantity.String].
//
// There is deliberately no UnmarshalText: reading a quantity back requires a
// [Registry] to resolve the unit symbol, and a method on Quantity has nowhere
// to obtain one. Decode with [Registry.Parse] instead, which makes the choice
// of registry explicit.
func (q Quantity) MarshalText() ([]byte, error) { return []byte(q.String()), nil }
