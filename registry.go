package quanta

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"
)

// Option adjusts how a unit is registered. Options are passed to
// [Registry.DefineBase], [Registry.Define] and [Registry.DefineAffine].
type Option func(*Unit)

// NoPrefix marks a unit as not accepting SI prefixes. It is the right choice
// for a unit whose symbol already carries one — the kilogram — and for symbols
// where prefixing would produce nonsense.
func NoPrefix() Option { return func(u *Unit) { u.prefixable = false } }

// BinaryPrefix marks a unit as accepting IEC binary prefixes (Ki, Mi, Gi, …)
// in addition to the decimal ones. Only information units (bit, byte) are
// registered this way by [SI].
func BinaryPrefix() Option { return func(u *Unit) { u.binary = true } }

// Registry is a namespace of unit symbols. It is the extension point of the
// package: rather than a fixed enumeration of conversions, callers define
// whatever units they need — including entirely new base dimensions — and
// parse expressions against their own registry.
//
// A Registry is safe for concurrent use, including concurrent definition and
// lookup.
type Registry struct {
	mu      sync.RWMutex
	symbols map[string]Unit
	order   []string // canonical symbols, in definition order
}

// NewRegistry returns an empty registry. No symbol is known to it until one is
// defined; only the seven base dimensions themselves are built in.
func NewRegistry() *Registry {
	return &Registry{symbols: make(map[string]Unit)}
}

// define is the common registration path.
func (r *Registry) define(u Unit, opts []Option) (Unit, error) {
	u.prefixable = !u.IsAffine()
	for _, o := range opts {
		o(&u)
	}
	if u.symbol == "" {
		return Unit{}, fmt.Errorf("empty symbol: %w", ErrInvalidUnit)
	}
	if u.factor == nil || u.factor.Sign() == 0 {
		return Unit{}, fmt.Errorf("unit %q must have a non-zero factor: %w", u.symbol, ErrInvalidUnit)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.symbols[u.symbol]; exists {
		return Unit{}, fmt.Errorf("symbol %q is already registered: %w", u.symbol, ErrDuplicateUnit)
	}
	r.symbols[u.symbol] = u
	r.order = append(r.order, u.symbol)
	return u, nil
}

// DefineBase registers a coherent base unit: one whose conversion factor to
// itself is exactly 1, such as the metre for length. Use it to introduce a
// dimension's reference unit, or to invent a dimension of your own.
func (r *Registry) DefineBase(name, symbol string, d Dimension, opts ...Option) (Unit, error) {
	return r.define(Unit{
		name:   name,
		symbol: symbol,
		dim:    d,
		factor: new(big.Rat).Set(ratOne),
	}, opts)
}

// Define registers a unit as an exact rational multiple of an existing one:
// one unit of the new symbol equals factor units of base. The factor must be
// non-zero, and base must be linear — an affine unit cannot be scaled.
//
// Because factor is a [big.Rat], conversions stay exact: an inch is defined as
// exactly 254/10000 of a metre, not as the float64 nearest 0.0254.
func (r *Registry) Define(name, symbol string, factor *big.Rat, base Unit, opts ...Option) (Unit, error) {
	if base.IsAffine() {
		return Unit{}, fmt.Errorf("cannot define %q relative to the affine unit %q: %w",
			symbol, base.symbol, ErrAffineUnit)
	}
	if factor == nil || factor.Sign() == 0 {
		return Unit{}, fmt.Errorf("unit %q must have a non-zero factor: %w", symbol, ErrInvalidUnit)
	}
	return r.define(Unit{
		name:   name,
		symbol: symbol,
		dim:    base.dim,
		factor: new(big.Rat).Mul(factor, base.fac()),
	}, opts)
}

// DefineAffine registers a unit with a zero point of its own: a value v in the
// new unit equals v*factor + offset units of base. The degree Celsius is
// factor 1 and offset 5463/20 (273.15) relative to the kelvin; the degree
// Fahrenheit is factor 5/9 and offset 45967/180.
//
// Affine units are deliberately restricted: they may not be prefixed, may not
// appear in a compound unit expression, and may not be multiplied, divided or
// raised to a power. See [Unit.Delta] for expressing differences.
func (r *Registry) DefineAffine(name, symbol string, factor, offset *big.Rat, base Unit, opts ...Option) (Unit, error) {
	if base.IsAffine() {
		return Unit{}, fmt.Errorf("cannot define %q relative to the affine unit %q: %w",
			symbol, base.symbol, ErrAffineUnit)
	}
	if factor == nil || factor.Sign() == 0 {
		return Unit{}, fmt.Errorf("unit %q must have a non-zero factor: %w", symbol, ErrInvalidUnit)
	}
	if offset == nil {
		offset = ratZero
	}
	return r.define(Unit{
		name:   name,
		symbol: symbol,
		dim:    base.dim,
		factor: new(big.Rat).Mul(factor, base.fac()),
		offset: new(big.Rat).Mul(offset, base.fac()),
	}, opts)
}

// Alias registers an additional symbol for an already-defined unit — "meter"
// for "m", say, or "sec" for "s". Aliases resolve to the same unit but do not
// appear in [Registry.Units].
func (r *Registry) Alias(symbol string, u Unit) error {
	if symbol == "" {
		return fmt.Errorf("empty alias symbol: %w", ErrInvalidUnit)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.symbols[symbol]; exists {
		return fmt.Errorf("symbol %q is already registered: %w", symbol, ErrDuplicateUnit)
	}
	r.symbols[symbol] = u
	return nil
}

// Unit looks up a single unit symbol, resolving SI and binary prefixes: "m"
// and "km" both succeed, and so does the alias "metre". An exactly registered
// symbol always wins over a prefixed reading, so "min" is the minute rather
// than a milli-inch.
//
// Unit does not parse expressions — there is no "m/s" here. Use
// [Registry.ParseUnit] for that.
func (r *Registry) Unit(symbol string) (Unit, bool) { return r.resolve(symbol) }

// lookup is the exact, prefix-free symbol lookup.
func (r *Registry) lookup(symbol string) (Unit, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.symbols[symbol]
	return u, ok
}

// MustUnit is [Registry.Unit] for symbols known to exist, such as package-level
// variables and tests. It panics if the symbol is not registered.
func (r *Registry) MustUnit(symbol string) Unit {
	u, ok := r.Unit(symbol)
	if !ok {
		panic(fmt.Errorf("%w: %q", ErrUnknownUnit, symbol))
	}
	return u
}

// Units returns every unit defined in the registry — excluding aliases — sorted
// by symbol, so the order is deterministic across runs and independent of
// definition order.
func (r *Registry) Units() []Unit {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Unit, 0, len(r.order))
	for _, s := range r.order {
		out = append(out, r.symbols[s])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].symbol < out[j].symbol })
	return out
}

// resolve turns a single identifier into a unit, trying the exact symbol first
// and only then prefixed readings of it. The exact match always wins: "min" is
// the minute, never a milli-inch, and "cd" is the candela, never a centi-day.
func (r *Registry) resolve(sym string) (Unit, bool) {
	if u, ok := r.lookup(sym); ok {
		return u, true
	}
	for _, p := range prefixes {
		rest, ok := strings.CutPrefix(sym, p.match)
		if !ok || rest == "" {
			continue
		}
		base, ok := r.lookup(rest)
		if !ok || !base.prefixable {
			continue
		}
		if p.binary && !base.binary {
			continue
		}
		return p.apply(base), true
	}
	return Unit{}, false
}
