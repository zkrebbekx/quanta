package quanta

import "math/big"

// rat parses an exact rational from a decimal or fractional string such as
// "0.45359237" or "5/9". It panics on malformed input, which can only be a
// programming error in this package or in the caller's own definitions.
func rat(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic("quanta: malformed rational literal " + s)
	}
	return r
}

// SI returns a registry preloaded with the SI base and derived units plus the
// common non-SI units that appear alongside them: imperial and US customary
// lengths, masses and volumes, the pressure, energy and information units in
// everyday use, and the two affine temperature scales.
//
// Each call builds and returns a fresh registry, so callers may extend the
// result — with [Registry.Define] and friends — without affecting anyone else.
// Building costs a few tens of microseconds; hoist the result out of hot loops
// rather than calling SI repeatedly.
//
// Some notes on the choices made:
//
//   - The coherent SI unit of mass is the kilogram, so "kg" is registered as
//     the base unit with factor 1 and is not itself prefixable. The gram is a
//     derived unit of 1/1000 kg, and it is the gram that carries prefixes, so
//     "mg" is 10⁻⁶ kg, exactly as SI intends.
//   - Information units (bit, byte) have no SI dimension, so they are modelled
//     as dimensionless. They are the only units that accept binary prefixes,
//     so "MiB" resolves but "Mim" does not.
//   - Temperature differences are expressed with the kelvin, or with the
//     linear delta units "Δ°C" and "Δ°F". See [Unit.Delta].
//   - The US and imperial gallons differ, so neither claims the symbol "gal";
//     they are "gal_US" and "gal_UK".
func SI() *Registry {
	r := NewRegistry()

	// --- Base units ------------------------------------------------------
	m := mustDefineBase(r, "metre", "m", Length)
	// The kilogram already carries a prefix, so it does not take another; the
	// gram below is the prefixable unit of mass.
	kg := mustDefineBase(r, "kilogram", "kg", Mass, NoPrefix())
	s := mustDefineBase(r, "second", "s", Time)
	mustDefineBase(r, "ampere", "A", Current)
	k := mustDefineBase(r, "kelvin", "K", Temperature)
	mustDefineBase(r, "mole", "mol", Amount)
	mustDefineBase(r, "candela", "cd", LuminousIntensity)

	mustAlias(r, "meter", m)
	mustAlias(r, "metre", m)
	mustAlias(r, "second", s)
	mustAlias(r, "kelvin", k)

	// --- Coherent derived units ------------------------------------------
	mustDefine(r, "newton", "N", ratOne, r.MustParseUnit("kg·m/s^2"))
	joule := mustDefine(r, "joule", "J", ratOne, r.MustParseUnit("N·m"))
	mustDefine(r, "watt", "W", ratOne, r.MustParseUnit("J/s"))
	mustDefine(r, "pascal", "Pa", ratOne, r.MustParseUnit("N/m^2"))
	mustDefine(r, "hertz", "Hz", ratOne, r.MustParseUnit("1/s"))
	mustDefine(r, "coulomb", "C", ratOne, r.MustParseUnit("A·s"))
	mustDefine(r, "volt", "V", ratOne, r.MustParseUnit("W/A"))
	ohm := mustDefine(r, "ohm", "Ω", ratOne, r.MustParseUnit("V/A"))
	mustDefine(r, "farad", "F", ratOne, r.MustParseUnit("C/V"))
	mustAlias(r, "ohm", ohm)

	// --- Length ----------------------------------------------------------
	// Every imperial length is exact by international agreement: the inch is
	// exactly 25.4 mm, and everything else follows from it.
	inch := mustDefine(r, "inch", "in", rat("254/10000"), m, NoPrefix())
	foot := mustDefine(r, "foot", "ft", rat("12"), inch, NoPrefix())
	yard := mustDefine(r, "yard", "yd", rat("3"), foot, NoPrefix())
	mustDefine(r, "mile", "mi", rat("1760"), yard, NoPrefix())
	mustDefine(r, "nautical mile", "nmi", rat("1852"), m, NoPrefix())
	mustAlias(r, "inch", inch)
	mustAlias(r, "foot", foot)
	mustAlias(r, "yard", yard)

	// --- Mass ------------------------------------------------------------
	gram := mustDefine(r, "gram", "g", rat("1/1000"), kg)
	tonne := mustDefine(r, "tonne", "t", rat("1000"), kg, NoPrefix())
	pound := mustDefine(r, "pound", "lb", rat("45359237/100000000"), kg, NoPrefix())
	mustDefine(r, "ounce", "oz", rat("1/16"), pound, NoPrefix())
	mustDefine(r, "stone", "st", rat("14"), pound, NoPrefix())
	mustAlias(r, "gram", gram)
	mustAlias(r, "tonne", tonne)
	mustAlias(r, "pound", pound)

	// --- Time ------------------------------------------------------------
	minute := mustDefine(r, "minute", "min", rat("60"), s, NoPrefix())
	hour := mustDefine(r, "hour", "h", rat("60"), minute, NoPrefix())
	day := mustDefine(r, "day", "d", rat("24"), hour, NoPrefix())
	mustAlias(r, "minute", minute)
	mustAlias(r, "hour", hour)
	mustAlias(r, "day", day)

	// --- Volume ----------------------------------------------------------
	litre := mustDefine(r, "litre", "L", rat("1/1000"), r.MustParseUnit("m^3"))
	mustAlias(r, "l", litre)
	mustAlias(r, "litre", litre)
	mustAlias(r, "liter", litre)
	// 231 cubic inches exactly, and 4.54609 L exactly.
	mustDefine(r, "US gallon", "gal_US", rat("3785411784/1000000000"), litre, NoPrefix())
	mustDefine(r, "imperial gallon", "gal_UK", rat("454609/100000"), litre, NoPrefix())

	// --- Pressure, energy, power -----------------------------------------
	pascal := r.MustUnit("Pa")
	mustDefine(r, "bar", "bar", rat("100000"), pascal)
	mustDefine(r, "atmosphere", "atm", rat("101325"), pascal, NoPrefix())
	mustDefine(r, "calorie", "cal", rat("4184/1000"), joule)
	mustDefine(r, "watt-hour", "Wh", rat("3600"), joule)
	// The 2019 SI redefinition fixed the elementary charge exactly, so the
	// electronvolt is an exact number of joules.
	mustDefine(r, "electronvolt", "eV", rat("1.602176634e-19"), joule)

	// --- Temperature ------------------------------------------------------
	degC := mustDefineAffine(r, "degree Celsius", "°C", ratOne, rat("5463/20"), k)
	degF := mustDefineAffine(r, "degree Fahrenheit", "°F", rat("5/9"), rat("45967/180"), k)
	mustAlias(r, "degC", degC)
	mustAlias(r, "degF", degF)
	// Linear counterparts for temperature differences.
	mustDefine(r, "delta degree Celsius", "Δ°C", ratOne, k, NoPrefix())
	mustDefine(r, "delta degree Fahrenheit", "Δ°F", rat("5/9"), k, NoPrefix())

	// --- Information ------------------------------------------------------
	bit := mustDefineBase(r, "bit", "bit", Dimensionless, BinaryPrefix())
	mustDefine(r, "byte", "B", rat("8"), bit, BinaryPrefix())

	return r
}

func mustDefineBase(r *Registry, name, symbol string, d Dimension, opts ...Option) Unit {
	u, err := r.DefineBase(name, symbol, d, opts...)
	if err != nil {
		panic(err)
	}
	return u
}

func mustDefine(r *Registry, name, symbol string, factor *big.Rat, base Unit, opts ...Option) Unit {
	u, err := r.Define(name, symbol, factor, base, opts...)
	if err != nil {
		panic(err)
	}
	return u
}

func mustDefineAffine(r *Registry, name, symbol string, factor, offset *big.Rat, base Unit, opts ...Option) Unit {
	u, err := r.DefineAffine(name, symbol, factor, offset, base, opts...)
	if err != nil {
		panic(err)
	}
	return u
}

func mustAlias(r *Registry, symbol string, u Unit) {
	if err := r.Alias(symbol, u); err != nil {
		panic(err)
	}
}
