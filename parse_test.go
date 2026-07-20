package quanta

import (
	"errors"
	"math/big"
	"strings"
	"testing"
)

func TestParseUnitShapes(t *testing.T) {
	t.Run("Given compound unit expressions", func(t *testing.T) {
		t.Run("When they are parsed", func(t *testing.T) {
			t.Run("Then the symbol, dimension and factor are as expected", func(t *testing.T) {
				tests := []struct {
					name    string
					in      string
					symbol  string
					dim     Dimension
					factor  string
					prefix2 bool
				}{
					{"simple", "m", "m", Length, "1", false},
					{"caret exponent", "m/s^2", "m/s²", Length.Div(Time.Pow(2)), "1", false},
					{"superscript exponent", "m/s²", "m/s²", Length.Div(Time.Pow(2)), "1", false},
					{"double star exponent", "m/s**2", "m/s²", Length.Div(Time.Pow(2)), "1", false},
					{"negative exponent", "m·s^-2", "m/s²", Length.Div(Time.Pow(2)), "1", false},
					{"superscript negative", "m·s⁻²", "m/s²", Length.Div(Time.Pow(2)), "1", false},
					{"explicit positive exponent", "m^+2", "m²", Length.Pow(2), "1", false},
					{"middle dot", "kW·h", "kW·h", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2)), "3600000", false},
					{"asterisk", "kW*h", "kW·h", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2)), "3600000", false},
					{"space", "kW h", "kW·h", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2)), "3600000", false},
					{"times sign", "kW×h", "kW·h", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2)), "3600000", false},
					{"full stop", "N.m", "N·m", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2)), "1", false},
					{"concentration", "mg/dL", "mg/dL", Mass.Div(Length.Pow(3)), "1/100", false},
					{"torque", "N·m", "N·m", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2)), "1", false},
					{"coherent force", "kg·m/s²", "kg·m/s²", Mass.Mul(Length).Div(Time.Pow(2)), "1", false},
					{"reciprocal", "1/s", "1/s", Time.Pow(-1), "1", false},
					{"reciprocal area", "m^-2", "1/m²", Length.Pow(-2), "1", false},
					{"parenthesised denominator", "J/(kg·K)", "J/(kg·K)", Length.Pow(2).Div(Time.Pow(2)).Div(Temperature), "1", false},
					{"nested parentheses", "((m))/((s))", "m/s", Length.Div(Time), "1", false},
					{"whitespace tolerated", "  kg · m / s^2 ", "kg·m/s²", Mass.Mul(Length).Div(Time.Pow(2)), "1", false},
					{"empty is one", "", "", Dimensionless, "1", false},
					{"blank is one", "   ", "", Dimensionless, "1", false},
					{"binary prefix", "MiB", "MiB", Dimensionless, "8388608", false},
					{"decimal prefix on byte", "MB", "MB", Dimensionless, "8000000", false},
					{"prefixed litre", "mL", "mL", Length.Pow(3), "1/1000000", false},
					{"micro sign", "µs", "µs", Time, "1/1000000", false},
					{"ascii micro", "us", "µs", Time, "1/1000000", false},
					{"deca", "dam", "dam", Length, "10", false},
					{"one over compound", "1/(m·s)", "1/(m·s)", Length.Pow(-1).Div(Time), "1", false},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						got, err := testSI.ParseUnit(tc.in)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						if got.Dimension() != tc.dim {
							t.Errorf("dimension = %v, want %v", got.Dimension(), tc.dim)
						}
						if got.Factor().RatString() != tc.factor {
							t.Errorf("factor = %v, want %v", got.Factor().RatString(), tc.factor)
						}
					})
				}
			})

			t.Run("Then the rendered symbol re-parses to the same unit", func(t *testing.T) {
				inputs := []string{
					"m", "m/s^2", "kW·h", "mg/dL", "N·m", "kg·m/s²", "1/s", "m^-2",
					"J/(kg·K)", "MiB", "mL", "µs", "dam", "m^3", "kg/m^3", "A·s",
					"°C", "Δ°C", "gal_US", "1/(m·s)", "W/(m^2·K^4)",
				}
				for _, in := range inputs {
					t.Run(in, func(t *testing.T) {
						first, err := testSI.ParseUnit(in)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						second, err := testSI.ParseUnit(first.Symbol())
						if err != nil {
							t.Fatalf("re-parsing %q: %v", first.Symbol(), err)
						}
						if !second.Equal(first) {
							t.Errorf("%q -> %q -> %q lost information", in, first.Symbol(), second.Symbol())
						}
					})
				}
			})
		})

		t.Run("When division is chained", func(t *testing.T) {
			t.Run("Then it associates to the left", func(t *testing.T) {
				got, err := testSI.ParseUnit("m/s/s")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				want := Length.Div(Time.Pow(2))
				if got.Dimension() != want {
					t.Errorf("m/s/s = %v, want %v", got.Dimension(), want)
				}
				grouped, err := testSI.ParseUnit("J/kg·K")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if grouped.Dimension() != Length.Pow(2).Div(Time.Pow(2)).Mul(Temperature) {
					t.Errorf("J/kg·K = %v, want (J/kg)·K", grouped.Dimension())
				}
			})
		})

		t.Run("When exponent notations are mixed", func(t *testing.T) {
			t.Run("Then a superscript may follow a caret and a sign", func(t *testing.T) {
				tests := []struct {
					in   string
					want Dimension
				}{
					{"m^²", Length.Pow(2)},
					{"m^-²", Length.Pow(-2)},
					{"m⁺²", Length.Pow(2)},
					{"m**-2", Length.Pow(-2)},
					{"m^1", Length},
				}
				for _, tc := range tests {
					t.Run(tc.in, func(t *testing.T) {
						got, err := testSI.ParseUnit(tc.in)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						if got.Dimension() != tc.want {
							t.Errorf("dimension = %v, want %v", got.Dimension(), tc.want)
						}
					})
				}
			})
		})

		t.Run("When a numeral other than one appears", func(t *testing.T) {
			t.Run("Then it is refused", func(t *testing.T) {
				if _, err := testSI.ParseUnit("2 m"); !errors.Is(err, ErrSyntax) {
					t.Errorf("got %v, want ErrSyntax", err)
				}
			})
		})
	})
}

func TestPrefixResolution(t *testing.T) {
	t.Run("Given symbols that could be read as prefixed units", func(t *testing.T) {
		t.Run("When they are resolved", func(t *testing.T) {
			t.Run("Then an exactly registered symbol always wins", func(t *testing.T) {
				tests := []struct {
					in     string
					dim    Dimension
					factor string
					why    string
				}{
					{"min", Time, "60", "the minute, not a milli-inch"},
					{"cd", LuminousIntensity, "1", "the candela, not a centi-day"},
					{"Pa", Mass.Div(Length).Div(Time.Pow(2)), "1", "the pascal, not a peta-year"},
					{"mol", Amount, "1", "the mole, not a milli-litre"},
					{"nmi", Length, "1852", "the nautical mile, not a nano-mile"},
					{"h", Time, "3600", "the hour, not the hecto prefix alone"},
					{"d", Time, "86400", "the day, not the deci prefix alone"},
					{"cal", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2)), "523/125", "the calorie"},
				}
				for _, tc := range tests {
					t.Run(tc.in, func(t *testing.T) {
						got, err := testSI.ParseUnit(tc.in)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						if got.Dimension() != tc.dim || got.Factor().RatString() != tc.factor {
							t.Errorf("%s resolved to %v (factor %v); expected %s",
								tc.in, got.Dimension(), got.Factor().RatString(), tc.why)
						}
					})
				}
			})

			t.Run("Then the kilogram is handled as SI intends", func(t *testing.T) {
				tests := []struct {
					in     string
					factor string
				}{
					{"kg", "1"},
					{"g", "1/1000"},
					{"mg", "1/1000000"},
					{"µg", "1/1000000000"},
					{"Mg", "1000"},
					{"t", "1000"},
				}
				for _, tc := range tests {
					t.Run(tc.in, func(t *testing.T) {
						got, err := testSI.ParseUnit(tc.in)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						if got.Dimension() != Mass {
							t.Fatalf("dimension = %v, want M", got.Dimension())
						}
						if got.Factor().RatString() != tc.factor {
							t.Errorf("factor = %v, want %v", got.Factor().RatString(), tc.factor)
						}
					})
				}
			})

			t.Run("Then the kilogram itself takes no further prefix", func(t *testing.T) {
				for _, in := range []string{"kkg", "mkg", "Mkg"} {
					if _, err := testSI.ParseUnit(in); !errors.Is(err, ErrUnknownUnit) {
						t.Errorf("ParseUnit(%q) = %v, want ErrUnknownUnit", in, err)
					}
				}
			})

			t.Run("Then binary prefixes attach only to information units", func(t *testing.T) {
				for _, in := range []string{"KiB", "MiB", "Gibit", "Tibit"} {
					if _, err := testSI.ParseUnit(in); err != nil {
						t.Errorf("ParseUnit(%q) = %v, want success", in, err)
					}
				}
				for _, in := range []string{"Mim", "Kis", "Gig"} {
					if _, err := testSI.ParseUnit(in); !errors.Is(err, ErrUnknownUnit) {
						t.Errorf("ParseUnit(%q) = %v, want ErrUnknownUnit", in, err)
					}
				}
			})

			t.Run("Then a bare prefix is not a unit", func(t *testing.T) {
				for _, in := range []string{"k", "da", "Ki", "µ"} {
					if _, err := testSI.ParseUnit(in); !errors.Is(err, ErrUnknownUnit) {
						t.Errorf("ParseUnit(%q) = %v, want ErrUnknownUnit", in, err)
					}
				}
			})

			t.Run("Then prefixes carry an exact rational factor", func(t *testing.T) {
				tests := []struct {
					in   string
					want *big.Rat
				}{
					{"Qm", pow10(30)},
					{"qm", pow10(-30)},
					{"Yim", nil},
					{"Yibit", pow2(80)},
					{"nm", pow10(-9)},
				}
				for _, tc := range tests {
					t.Run(tc.in, func(t *testing.T) {
						got, err := testSI.ParseUnit(tc.in)
						if tc.want == nil {
							if err == nil {
								t.Fatalf("ParseUnit(%q) unexpectedly succeeded", tc.in)
							}
							return
						}
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						if got.Factor().Cmp(tc.want) != 0 {
							t.Errorf("factor = %v, want %v", got.Factor(), tc.want)
						}
					})
				}
			})
		})
	})
}

func TestParseUnitErrors(t *testing.T) {
	t.Run("Given malformed unit expressions", func(t *testing.T) {
		t.Run("When they are parsed", func(t *testing.T) {
			t.Run("Then a positioned ParseError names the problem", func(t *testing.T) {
				tests := []struct {
					name    string
					in      string
					wantErr error
					offset  int
				}{
					{"unknown symbol", "zzz", ErrUnknownUnit, 0},
					{"unknown after operator", "m/zzz", ErrUnknownUnit, 2},
					{"trailing solidus", "m/", ErrSyntax, 2},
					{"leading solidus", "/s", ErrSyntax, 0},
					{"unclosed parenthesis", "(m", ErrSyntax, 2},
					{"unexpected close", "m)", ErrSyntax, 1},
					{"empty parentheses", "()", ErrSyntax, 1},
					{"exponent with no digits", "m^", ErrSyntax, 2},
					{"exponent with sign only", "m^-", ErrSyntax, 3},
					{"exponent of nothing", "^2", ErrSyntax, 0},
					{"bad character", "m@s", ErrSyntax, 1},
					{"numeral other than one", "3", ErrSyntax, 0},
					{"stray minus", "m-s", ErrSyntax, 1},
					{"stray plus", "m+s", ErrSyntax, 1},
					{"huge exponent", "m^999999999999", ErrExponentRange, 2},
					{"huge exponent literal", "m^99999", ErrExponentRange, 2},
					{"huge superscript", "m⁹⁹⁹⁹⁹", ErrExponentRange, 1},
					{"dimension overflow", "m^100·m^100", ErrExponentRange, 5},
					{"superscript sign alone", "m⁻", ErrSyntax, 1},
					{"invalid utf-8", "m/\xff", ErrSyntax, 2},
					{"affine in compound", "°C/m", ErrAffineUnit, 3},
					{"affine raised", "°C²", ErrAffineUnit, 3},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						_, err := testSI.ParseUnit(tc.in)
						if err == nil {
							t.Fatal("expected an error")
						}
						if !errors.Is(err, tc.wantErr) {
							t.Errorf("got %v, want %v", err, tc.wantErr)
						}
						var pe *ParseError
						if !errors.As(err, &pe) {
							t.Fatalf("error is not a *ParseError: %T", err)
						}
						if pe.Input != tc.in {
							t.Errorf("Input = %q, want %q", pe.Input, tc.in)
						}
						if pe.Offset != tc.offset {
							t.Errorf("Offset = %d, want %d", pe.Offset, tc.offset)
						}
						if !strings.Contains(pe.Error(), "quanta: parse") {
							t.Errorf("Error() = %q", pe.Error())
						}
					})
				}
			})

			t.Run("Then deeply nested input is refused rather than crashing", func(t *testing.T) {
				in := strings.Repeat("(", 200) + "m" + strings.Repeat(")", 200)
				if _, err := testSI.ParseUnit(in); !errors.Is(err, ErrSyntax) {
					t.Errorf("got %v, want ErrSyntax", err)
				}
			})
		})
	})
}

func TestParseQuantity(t *testing.T) {
	t.Run("Given quantity strings", func(t *testing.T) {
		t.Run("When they are parsed", func(t *testing.T) {
			t.Run("Then magnitude and unit are recovered", func(t *testing.T) {
				tests := []struct {
					name   string
					in     string
					value  float64
					symbol string
				}{
					{"with space", "9.81 m/s^2", 9.81, "m/s²"},
					{"without space", "9.81m/s^2", 9.81, "m/s²"},
					{"compound", "1.5 kW·h", 1.5, "kW·h"},
					{"negative", "-40 °C", -40, "°C"},
					{"explicit positive", "+3 m", 3, "m"},
					{"exponent notation", "1.5e3 m", 1500, "m"},
					{"negative exponent notation", "1.5e-3 m", 0.0015, "m"},
					{"capital exponent", "1E3 m", 1000, "m"},
					{"bare number", "42", 42, ""},
					{"bare decimal", "0.5", 0.5, ""},
					{"leading decimal point", "5. m", 5, "m"},
					{"unit starting with e", "5 eV", 5, "eV"},
					{"unit starting with e, no space", "5eV", 5, "eV"},
					{"surrounding whitespace", "  7 kg  ", 7, "kg"},
					{"delta unit", "20 Δ°C", 20, "Δ°C"},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						got, err := testSI.Parse(tc.in)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						if got.Value() != tc.value {
							t.Errorf("value = %v, want %v", got.Value(), tc.value)
						}
						if got.Unit().Symbol() != tc.symbol {
							t.Errorf("symbol = %q, want %q", got.Unit().Symbol(), tc.symbol)
						}
					})
				}
			})

			t.Run("Then malformed input yields a positioned error", func(t *testing.T) {
				tests := []struct {
					name    string
					in      string
					wantErr error
					offset  int
				}{
					{"empty", "", ErrSyntax, 0},
					{"blank", "   ", ErrSyntax, 0},
					{"no number", "m/s", ErrSyntax, 0},
					{"no number with sign", "-m", ErrSyntax, 0},
					{"just a sign", "-", ErrSyntax, 0},
					{"just a decimal point", ".", ErrSyntax, 0},
					{"unknown unit", "5 zzz", ErrUnknownUnit, 2},
					{"unknown unit offset", "  5 zzz", ErrUnknownUnit, 4},
					{"bad unit expression", "5 m/", ErrSyntax, 4},
					{"affine compound", "5 °C/m", ErrAffineUnit, 5},
					{"overflowing number", "1e999999 m", ErrSyntax, 0},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						_, err := testSI.Parse(tc.in)
						if err == nil {
							t.Fatal("expected an error")
						}
						if !errors.Is(err, tc.wantErr) {
							t.Errorf("got %v, want %v", err, tc.wantErr)
						}
						var pe *ParseError
						if !errors.As(err, &pe) {
							t.Fatalf("error is not a *ParseError: %T", err)
						}
						if pe.Input != tc.in {
							t.Errorf("Input = %q, want %q", pe.Input, tc.in)
						}
						if pe.Offset != tc.offset {
							t.Errorf("Offset = %d, want %d", pe.Offset, tc.offset)
						}
					})
				}
			})

			t.Run("Then the wrapped sentinel is reachable through Unwrap", func(t *testing.T) {
				_, err := testSI.Parse("5 zzz")
				var pe *ParseError
				if !errors.As(err, &pe) {
					t.Fatal("not a ParseError")
				}
				if !errors.Is(pe.Unwrap(), ErrUnknownUnit) {
					t.Errorf("Unwrap = %v, want ErrUnknownUnit", pe.Unwrap())
				}
			})
		})
	})
}

func TestCustomUnitParsing(t *testing.T) {
	t.Run("Given caller-defined units", func(t *testing.T) {
		r := SI()
		if _, err := r.DefineBase("US dollar", "USD", Dimensionless, NoPrefix()); err != nil {
			t.Fatalf("DefineBase: %v", err)
		}

		t.Run("When they appear in an expression", func(t *testing.T) {
			t.Run("Then they parse like any other unit", func(t *testing.T) {
				q, err := r.Parse("640 USD/tonne")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if q.Value() != 640 {
					t.Errorf("value = %v, want 640", q.Value())
				}
				if got := q.Unit().Symbol(); got != "USD/t" {
					t.Errorf("symbol = %q, want %q", got, "USD/t")
				}
				if q.Dimension() != Mass.Pow(-1) {
					t.Errorf("dimension = %v, want M⁻¹", q.Dimension())
				}
			})

			t.Run("Then arithmetic with them works", func(t *testing.T) {
				price := r.MustParse("640 USD/t")
				mass := r.MustParse("2.5 t")
				total, err := price.Mul(mass)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if total.Value() != 1600 {
					t.Errorf("value = %v, want 1600", total.Value())
				}
				if !total.Dimension().IsDimensionless() {
					t.Errorf("dimension = %v, want dimensionless", total.Dimension())
				}
			})
		})
	})
}

func TestLexer(t *testing.T) {
	t.Run("Given raw input to the lexer", func(t *testing.T) {
		t.Run("When it is tokenised", func(t *testing.T) {
			t.Run("Then positions are recorded on every token", func(t *testing.T) {
				toks, err := lex("kg·m/s^2")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(toks) == 0 || toks[len(toks)-1].kind != tkEOF {
					t.Fatalf("token stream does not end with EOF: %+v", toks)
				}
				for i, tok := range toks {
					if tok.pos < 0 || tok.pos > len("kg·m/s^2") {
						t.Errorf("token %d has position %d", i, tok.pos)
					}
				}
			})

			t.Run("Then an unknown token renders in the error message", func(t *testing.T) {
				_, err := testSI.ParseUnit("m ) s")
				var pe *ParseError
				if !errors.As(err, &pe) {
					t.Fatalf("not a ParseError: %v", err)
				}
				if !strings.Contains(pe.Msg, ")") {
					t.Errorf("Msg = %q, want it to mention %q", pe.Msg, ")")
				}
			})
		})
	})
}
