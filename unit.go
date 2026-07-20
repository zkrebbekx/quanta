package quanta

import (
	"fmt"
	"math/big"
	"strings"
)

// term is one symbol/exponent pair in a compound unit's factorisation.
type term struct {
	sym string
	exp int
}

// Unit is a named scale on a [Dimension].
//
// A unit records an exact conversion to the coherent SI unit of its dimension
// as a rational factor, plus an optional rational offset for affine units such
// as °C and °F. The relationship is
//
//	value_in_coherent_SI = value*factor + offset
//
// Units are immutable values produced by a [Registry] or by combining other
// units (see [Quantity.Mul] and friends). Copying a Unit is free and safe; a
// Unit is safe for concurrent use.
//
// The zero Unit is the dimensionless unit one: factor 1, no offset, empty
// symbol. It is fully usable, so New(2, quanta.Unit{}) is a valid count of two.
//
// Unit is not comparable with == (it carries a slice of factorisation terms);
// use [Unit.Equal], or compare [Unit.Symbol] values when a map key is needed.
type Unit struct {
	name   string
	symbol string
	dim    Dimension
	// factor is the exact multiplier to the coherent SI unit. A nil factor
	// means exactly 1, which keeps the zero Unit usable.
	factor *big.Rat
	// offset is the exact additive shift to the coherent SI unit, in coherent
	// SI units. A nil offset means zero, i.e. the unit is linear.
	offset *big.Rat
	// terms is the compound factorisation used to render Symbol. It is nil for
	// atomic units, which are equivalent to the single term {symbol, 1}.
	terms []term
	// prefixable reports whether SI decimal prefixes may be attached.
	prefixable bool
	// binary reports whether binary (Ki, Mi, ...) prefixes may be attached.
	binary bool
}

// One is the dimensionless unit one — the identity for unit multiplication and
// the unit of a bare number.
var One = Unit{}

var (
	ratZero = new(big.Rat)
	ratOne  = big.NewRat(1, 1)
)

// Name returns the unit's full name, such as "metre". Compound and prefixed
// units synthesise a name where they can and return the empty string
// otherwise.
func (u Unit) Name() string { return u.name }

// Symbol returns the unit's symbol, such as "m" or "kg·m/s²". The symbol of the
// dimensionless unit one is the empty string.
//
// A compound symbol always re-parses to the same unit, which is why a
// denominator of several factors is parenthesised: "J/(kg·K)", not "J/kg·K".
func (u Unit) Symbol() string { return u.symbol }

// Dimension returns the unit's dimension.
func (u Unit) Dimension() Dimension { return u.dim }

// IsAffine reports whether the unit has a non-zero zero point, as °C and °F do.
// Affine units may be converted to and from, compared, and used with Add and
// Sub under the rules described on those methods, but they may never take part
// in multiplication, division, exponentiation, or a compound unit expression.
func (u Unit) IsAffine() bool { return u.offset != nil && u.offset.Sign() != 0 }

// Factor returns a copy of the exact rational conversion factor from this unit
// to the coherent SI unit of its dimension. The returned value is a fresh
// [big.Rat] the caller may modify freely.
func (u Unit) Factor() *big.Rat { return new(big.Rat).Set(u.fac()) }

// Offset returns a copy of the exact rational offset to the coherent SI unit,
// which is zero for every linear unit. The returned value is a fresh [big.Rat]
// the caller may modify freely.
func (u Unit) Offset() *big.Rat { return new(big.Rat).Set(u.off()) }

// fac returns the internal factor, treating nil as exactly 1. The result must
// not be mutated.
func (u Unit) fac() *big.Rat {
	if u.factor == nil {
		return ratOne
	}
	return u.factor
}

// off returns the internal offset, treating nil as exactly 0. The result must
// not be mutated.
func (u Unit) off() *big.Rat {
	if u.offset == nil {
		return ratZero
	}
	return u.offset
}

// Equal reports whether two units are interchangeable: same dimension, same
// exact factor, same exact offset and same symbol.
func (u Unit) Equal(o Unit) bool {
	return u.symbol == o.symbol && u.dim == o.dim &&
		u.fac().Cmp(o.fac()) == 0 && u.off().Cmp(o.off()) == 0
}

// String returns the unit's symbol, so a Unit may be printed directly.
func (u Unit) String() string { return u.symbol }

// Delta returns the linear "difference" unit that shares this unit's scale but
// has no zero point — the unit in which differences of affine quantities are
// expressed. Delta on °C yields Δ°C, one kelvin of temperature difference; on a
// unit that is already linear it returns the unit unchanged.
func (u Unit) Delta() Unit {
	if !u.IsAffine() {
		return u
	}
	d := u
	d.offset = nil
	d.symbol = "Δ" + u.symbol
	d.name = "delta " + u.name
	d.terms = nil
	d.prefixable = false
	return d
}

// termList returns the unit's factorisation, synthesising the single-term list
// of an atomic unit. The dimensionless unit one factorises to nothing.
func (u Unit) termList() []term {
	if u.terms != nil {
		return u.terms
	}
	if u.symbol == "" {
		return nil
	}
	return []term{{sym: u.symbol, exp: 1}}
}

// combine builds the compound unit a^1 · b^sign, merging terms that share a
// symbol. It reports ErrAffineUnit if either operand is affine and
// ErrExponentRange if the resulting dimension would overflow.
func combine(a, b Unit, sign int) (Unit, error) {
	if a.IsAffine() || b.IsAffine() {
		return Unit{}, ErrAffineUnit
	}
	var (
		dim Dimension
		ok  bool
	)
	if sign > 0 {
		dim, ok = a.dim.CheckedMul(b.dim)
	} else {
		dim, ok = a.dim.CheckedDiv(b.dim)
	}
	if !ok {
		return Unit{}, ErrExponentRange
	}

	factor := new(big.Rat)
	if sign > 0 {
		factor.Mul(a.fac(), b.fac())
	} else {
		if b.fac().Sign() == 0 {
			return Unit{}, ErrInvalidUnit
		}
		factor.Quo(a.fac(), b.fac())
	}

	terms := mergeTerms(a.termList(), b.termList(), sign)
	if err := checkTerms(terms); err != nil {
		return Unit{}, err
	}
	return newCompound(dim, factor, terms), nil
}

// checkTerms bounds the exponent of each symbol in a factorisation. Dimension
// exponents are bounded by int8, but a product of dimensionless units — bytes,
// say, or a caller-defined currency — has no dimension to overflow, so without
// this a symbol could grow an exponent that its own rendering could not parse
// back, and whose exact factor would be enormous.
func checkTerms(terms []term) error {
	for _, t := range terms {
		if t.exp > maxExponent || t.exp < -maxExponent {
			return fmt.Errorf("exponent %d on %q exceeds %d: %w",
				t.exp, t.sym, maxExponent, ErrExponentRange)
		}
	}
	return nil
}

// mergeTerms adds the exponents of b (scaled by sign) into a copy of a,
// preserving the order in which symbols were first seen and dropping symbols
// whose exponent cancels to zero.
func mergeTerms(a, b []term, sign int) []term {
	out := make([]term, 0, len(a)+len(b))
	out = append(out, a...)
	for _, t := range b {
		e := t.exp * sign
		found := false
		for i := range out {
			if out[i].sym == t.sym {
				out[i].exp += e
				found = true
				break
			}
		}
		if !found {
			out = append(out, term{sym: t.sym, exp: e})
		}
	}
	kept := out[:0]
	for _, t := range out {
		if t.exp != 0 {
			kept = append(kept, t)
		}
	}
	return kept
}

// newCompound assembles a compound unit from its dimension, exact factor and
// factorisation, deriving the rendered symbol.
func newCompound(dim Dimension, factor *big.Rat, terms []term) Unit {
	if len(terms) == 0 {
		// Everything cancelled; this is the unit one. Because merging is by
		// symbol and symbols are unique within a registry, the factor of a
		// fully cancelled product is necessarily 1.
		return Unit{}
	}
	if len(terms) == 1 && terms[0].exp == 1 {
		// A single unit power of one is atomic again.
		return Unit{
			name:   "",
			symbol: terms[0].sym,
			dim:    dim,
			factor: factor,
		}
	}
	return Unit{
		symbol: renderTerms(terms),
		dim:    dim,
		factor: factor,
		terms:  terms,
	}
}

// renderTerms formats a factorisation as "a·b²/c": positive powers first, then
// a single solidus followed by the negative powers with their exponents made
// positive. A factorisation with no positive powers renders as "1/…".
//
// A denominator of more than one factor is parenthesised — "J/(kg·K)", never
// "J/kg·K" — because · and / associate to the left, so the unparenthesised
// form would parse back as (J/kg)·K. The rendered symbol always re-parses to
// the same unit.
func renderTerms(terms []term) string {
	var num, den strings.Builder
	denCount := 0
	for _, t := range terms {
		dst, exp := &num, t.exp
		if exp < 0 {
			dst, exp = &den, -exp
			denCount++
		}
		if dst.Len() > 0 {
			dst.WriteString("·")
		}
		dst.WriteString(t.sym)
		if exp != 1 {
			dst.WriteString(superscript(exp))
		}
	}
	if den.Len() == 0 {
		return num.String()
	}
	bottom := den.String()
	if denCount > 1 {
		bottom = "(" + bottom + ")"
	}
	if num.Len() == 0 {
		return "1/" + bottom
	}
	return num.String() + "/" + bottom
}

// powUnit raises a unit to an integer power, exactly.
func powUnit(u Unit, n int) (Unit, error) {
	if u.IsAffine() {
		return Unit{}, ErrAffineUnit
	}
	dim, ok := u.dim.CheckedPow(n)
	if !ok {
		return Unit{}, ErrExponentRange
	}
	if n == 0 {
		return One, nil
	}
	factor, err := ratPow(u.fac(), n)
	if err != nil {
		return Unit{}, err
	}
	src := u.termList()
	terms := make([]term, 0, len(src))
	for _, t := range src {
		terms = append(terms, term{sym: t.sym, exp: t.exp * n})
	}
	if err := checkTerms(terms); err != nil {
		return Unit{}, err
	}
	return newCompound(dim, factor, terms), nil
}

// ratPow raises an exact rational to an integer power. A negative power of
// zero is an invalid unit rather than an infinity.
func ratPow(r *big.Rat, n int) (*big.Rat, error) {
	if n == 0 {
		return new(big.Rat).Set(ratOne), nil
	}
	inv := n < 0
	if inv {
		n = -n
	}
	num := new(big.Int).Exp(r.Num(), big.NewInt(int64(n)), nil)
	den := new(big.Int).Exp(r.Denom(), big.NewInt(int64(n)), nil)
	out := new(big.Rat).SetFrac(num, den)
	if inv {
		if out.Sign() == 0 {
			return nil, ErrInvalidUnit
		}
		out.Inv(out)
	}
	return out, nil
}
