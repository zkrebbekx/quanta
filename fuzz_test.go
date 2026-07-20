package quanta

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzParseUnit asserts that the unit parser never panics, always terminates,
// and only ever fails with a well-formed *ParseError wrapping one of the
// documented sentinels.
func FuzzParseUnit(f *testing.F) {
	seeds := []string{
		"", " ", "m", "m/s", "m/s^2", "m/s²", "kW·h", "mg/dL", "N·m",
		"kg·m/s²", "1/s", "m^-2", "J/(kg·K)", "°C", "Δ°C", "MiB", "µs",
		"((m))", "m**2", "m^+2", "m/", "/m", "^2", "m@s", "3", "m)", "(m",
		"m^999999999999", "m⁻", "\xff", "gal_US", "USD/tonne", "m m m",
		"m·", "·m", "m^-", "1", "1/1", "d", "min", "cd", "kkg", "Mim",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	r := SI()
	f.Fuzz(func(t *testing.T, in string) {
		u, err := r.ParseUnit(in)
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("error is not a *ParseError: %T %v", err, err)
			}
			if pe.Input != in {
				t.Fatalf("ParseError.Input = %q, want %q", pe.Input, in)
			}
			if pe.Offset < 0 || pe.Offset > len(in) {
				t.Fatalf("ParseError.Offset = %d, out of range for input of length %d",
					pe.Offset, len(in))
			}
			switch {
			case errors.Is(err, ErrSyntax),
				errors.Is(err, ErrUnknownUnit),
				errors.Is(err, ErrAffineUnit),
				errors.Is(err, ErrExponentRange),
				errors.Is(err, ErrInvalidUnit):
			default:
				t.Fatalf("error wraps no documented sentinel: %v", err)
			}
			return
		}

		// A successful parse must produce a usable unit whose symbol is valid
		// UTF-8 and which re-parses to the same unit.
		if !utf8.ValidString(u.Symbol()) {
			t.Fatalf("unit symbol is not valid UTF-8: %q", u.Symbol())
		}
		again, err := r.ParseUnit(u.Symbol())
		if err != nil {
			t.Fatalf("symbol %q from input %q does not re-parse: %v", u.Symbol(), in, err)
		}
		if !again.Equal(u) {
			t.Fatalf("input %q: %q re-parsed to %q", in, u.Symbol(), again.Symbol())
		}
		// Every parsed unit must be usable in a conversion of its own kind.
		q := New(1, u)
		if _, err := q.In(u); err != nil {
			t.Fatalf("self-conversion failed for %q: %v", u.Symbol(), err)
		}
	})
}

// FuzzParse asserts the same properties for the quantity parser.
func FuzzParse(f *testing.F) {
	seeds := []string{
		"9.81 m/s^2", "1.5 kW·h", "-40 °C", "42", "5eV", "1e3 m", "  7 kg  ",
		"", "m", "-", ".", "1e999999 m", "5 zzz", "+3 m", "5. m", "20 Δ°C",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	r := SI()
	f.Fuzz(func(t *testing.T, in string) {
		q, err := r.Parse(in)
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("error is not a *ParseError: %T %v", err, err)
			}
			if pe.Offset < 0 || pe.Offset > len(pe.Input) {
				t.Fatalf("ParseError.Offset = %d, out of range for %q", pe.Offset, pe.Input)
			}
			return
		}
		// String must always be renderable and must not lose the unit symbol.
		s := q.String()
		if !utf8.ValidString(s) {
			t.Fatalf("String() is not valid UTF-8: %q", s)
		}
		if sym := q.Unit().Symbol(); sym != "" && !strings.HasSuffix(s, sym) {
			t.Fatalf("String() = %q does not end with the symbol %q", s, sym)
		}
	})
}
