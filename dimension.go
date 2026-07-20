package quanta

import "strings"

// Base-quantity indices into a [Dimension].
const (
	IdxLength = iota
	IdxMass
	IdxTime
	IdxCurrent
	IdxTemperature
	IdxAmount
	IdxLuminousIntensity

	// NumBaseQuantities is the number of SI base quantities.
	NumBaseQuantities
)

// Dimension is a vector of the seven SI base-quantity exponents, in the order
// length, mass, time, electric current, thermodynamic temperature, amount of
// substance, luminous intensity.
//
// Dimension is a comparable array type: it may be used as a map key and
// compared with ==. The zero Dimension is [Dimensionless].
type Dimension [NumBaseQuantities]int8

// The seven base dimensions, plus the dimensionless (zero) dimension.
var (
	Dimensionless     = Dimension{}
	Length            = Dimension{IdxLength: 1}
	Mass              = Dimension{IdxMass: 1}
	Time              = Dimension{IdxTime: 1}
	Current           = Dimension{IdxCurrent: 1}
	Temperature       = Dimension{IdxTemperature: 1}
	Amount            = Dimension{IdxAmount: 1}
	LuminousIntensity = Dimension{IdxLuminousIntensity: 1}
)

// baseSymbols are the conventional single-letter dimension symbols, indexed by
// the Idx* constants.
var baseSymbols = [NumBaseQuantities]string{
	IdxLength:            "L",
	IdxMass:              "M",
	IdxTime:              "T",
	IdxCurrent:           "I",
	IdxTemperature:       "Θ",
	IdxAmount:            "N",
	IdxLuminousIntensity: "J",
}

// displayOrder fixes the order in which base symbols appear in [Dimension.String],
// following the customary M·L·T ordering of dimensional analysis. It is a
// constant of the package, so String output is fully deterministic.
var displayOrder = [NumBaseQuantities]int{
	IdxMass, IdxLength, IdxTime, IdxCurrent,
	IdxTemperature, IdxAmount, IdxLuminousIntensity,
}

// clamp8 saturates n to the int8 range.
func clamp8(n int) int8 {
	switch {
	case n > 127:
		return 127
	case n < -128:
		return -128
	default:
		return int8(n)
	}
}

// Mul returns the dimension of a product: exponents are added element-wise.
//
// Exponents saturate at the bounds of int8 rather than wrapping. Saturation is
// silent here because Mul cannot fail; the quantity-level operations
// ([Quantity.Mul], [Quantity.Div], [Quantity.Pow]) detect the same condition
// and report [ErrExponentRange] instead. Use [Dimension.CheckedMul] when you
// need to know.
func (d Dimension) Mul(o Dimension) Dimension {
	var out Dimension
	for i := range d {
		out[i] = clamp8(int(d[i]) + int(o[i]))
	}
	return out
}

// Div returns the dimension of a quotient: exponents are subtracted
// element-wise. Exponents saturate exactly as described for [Dimension.Mul].
func (d Dimension) Div(o Dimension) Dimension {
	var out Dimension
	for i := range d {
		out[i] = clamp8(int(d[i]) - int(o[i]))
	}
	return out
}

// Pow returns d raised to the integer power n: every exponent is multiplied by
// n. Exponents saturate exactly as described for [Dimension.Mul].
func (d Dimension) Pow(n int) Dimension {
	var out Dimension
	for i := range d {
		out[i] = clamp8(int(d[i]) * n)
	}
	return out
}

// CheckedMul is [Dimension.Mul] with an overflow report: ok is false if any
// resulting exponent would fall outside the int8 range, in which case the
// returned dimension is saturated.
func (d Dimension) CheckedMul(o Dimension) (Dimension, bool) {
	ok := true
	var out Dimension
	for i := range d {
		n := int(d[i]) + int(o[i])
		if n > 127 || n < -128 {
			ok = false
		}
		out[i] = clamp8(n)
	}
	return out, ok
}

// CheckedDiv is [Dimension.Div] with an overflow report, as for
// [Dimension.CheckedMul].
func (d Dimension) CheckedDiv(o Dimension) (Dimension, bool) {
	ok := true
	var out Dimension
	for i := range d {
		n := int(d[i]) - int(o[i])
		if n > 127 || n < -128 {
			ok = false
		}
		out[i] = clamp8(n)
	}
	return out, ok
}

// CheckedPow is [Dimension.Pow] with an overflow report, as for
// [Dimension.CheckedMul].
func (d Dimension) CheckedPow(n int) (Dimension, bool) {
	ok := true
	var out Dimension
	for i := range d {
		p := int(d[i]) * n
		if p > 127 || p < -128 {
			ok = false
		}
		out[i] = clamp8(p)
	}
	return out, ok
}

// IsDimensionless reports whether every base exponent is zero.
func (d Dimension) IsDimensionless() bool { return d == Dimensionless }

// String renders the dimension using the conventional base symbols M, L, T, I,
// Θ, N and J joined by the middle dot, with Unicode superscript exponents —
// for example "L·T⁻²" for acceleration and "M·L²·T⁻²" for energy. Exponents of
// one are omitted and zero exponents are skipped entirely. The dimensionless
// dimension renders as "1".
//
// The ordering is fixed (mass, length, time, current, temperature, amount,
// luminous intensity), so the output is deterministic and safe to compare.
func (d Dimension) String() string {
	if d.IsDimensionless() {
		return "1"
	}
	var b strings.Builder
	for _, i := range displayOrder {
		e := d[i]
		if e == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("·")
		}
		b.WriteString(baseSymbols[i])
		if e != 1 {
			b.WriteString(superscript(int(e)))
		}
	}
	return b.String()
}

// superscriptDigits maps 0-9 to their Unicode superscript forms.
var superscriptDigits = [10]string{"⁰", "¹", "²", "³", "⁴", "⁵", "⁶", "⁷", "⁸", "⁹"}

// superscript renders n using Unicode superscript characters.
func superscript(n int) string {
	if n == 0 {
		return superscriptDigits[0]
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []string
	for n > 0 {
		digits = append(digits, superscriptDigits[n%10])
		n /= 10
	}
	var b strings.Builder
	if neg {
		b.WriteString("⁻")
	}
	for i := len(digits) - 1; i >= 0; i-- {
		b.WriteString(digits[i])
	}
	return b.String()
}
