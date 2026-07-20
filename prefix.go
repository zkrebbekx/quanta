package quanta

import (
	"math/big"
	"sort"
)

// prefix is an SI decimal or IEC binary multiplier that may be attached to a
// unit symbol.
type prefix struct {
	// symbol is the canonical rendering, used to build the prefixed unit's
	// symbol.
	symbol string
	// match is the text a caller may write, which differs from symbol only for
	// the ASCII spelling "u" of micro.
	match  string
	name   string
	factor *big.Rat
	binary bool
}

// pow10 returns 10**n as an exact rational.
func pow10(n int) *big.Rat {
	ten := big.NewInt(10)
	if n >= 0 {
		return new(big.Rat).SetInt(new(big.Int).Exp(ten, big.NewInt(int64(n)), nil))
	}
	return new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(ten, big.NewInt(int64(-n)), nil))
}

// pow2 returns 2**n as an exact rational.
func pow2(n int) *big.Rat {
	return new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(n)), nil))
}

// prefixes lists every SI decimal prefix from quecto to quetta and every IEC
// binary prefix. Factors are exact rationals, never floats.
var prefixes = func() []prefix {
	decimal := []struct {
		sym   string
		match string
		name  string
		exp   int
	}{
		{"Q", "", "quetta", 30}, {"R", "", "ronna", 27}, {"Y", "", "yotta", 24},
		{"Z", "", "zetta", 21}, {"E", "", "exa", 18}, {"P", "", "peta", 15},
		{"T", "", "tera", 12}, {"G", "", "giga", 9}, {"M", "", "mega", 6},
		{"k", "", "kilo", 3}, {"h", "", "hecto", 2}, {"da", "", "deca", 1},
		{"d", "", "deci", -1}, {"c", "", "centi", -2}, {"m", "", "milli", -3},
		{"µ", "", "micro", -6}, {"µ", "u", "micro", -6}, {"n", "", "nano", -9},
		{"p", "", "pico", -12}, {"f", "", "femto", -15}, {"a", "", "atto", -18},
		{"z", "", "zepto", -21}, {"y", "", "yocto", -24}, {"r", "", "ronto", -27},
		{"q", "", "quecto", -30},
	}
	binary := []struct {
		sym  string
		name string
		exp  int
	}{
		{"Ki", "kibi", 10}, {"Mi", "mebi", 20}, {"Gi", "gibi", 30},
		{"Ti", "tebi", 40}, {"Pi", "pebi", 50}, {"Ei", "exbi", 60},
		{"Zi", "zebi", 70}, {"Yi", "yobi", 80},
	}

	out := make([]prefix, 0, len(decimal)+len(binary))
	for _, p := range decimal {
		match := p.match
		if match == "" {
			match = p.sym
		}
		out = append(out, prefix{symbol: p.sym, match: match, name: p.name, factor: pow10(p.exp)})
	}
	for _, p := range binary {
		out = append(out, prefix{
			symbol: p.sym, match: p.sym, name: p.name,
			factor: pow2(p.exp), binary: true,
		})
	}
	// Longest match first, then lexicographic, so that resolution is
	// deterministic and multi-character prefixes ("da", "Ki") are tried before
	// the single-character ones they start with.
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].match) != len(out[j].match) {
			return len(out[i].match) > len(out[j].match)
		}
		return out[i].match < out[j].match
	})
	return out
}()

// apply returns u scaled by the prefix, with a synthesised symbol and name.
func (p prefix) apply(u Unit) Unit {
	out := Unit{
		symbol:     p.symbol + u.symbol,
		dim:        u.dim,
		factor:     new(big.Rat).Mul(p.factor, u.fac()),
		prefixable: false,
	}
	if u.name != "" {
		out.name = p.name + u.name
	}
	return out
}
