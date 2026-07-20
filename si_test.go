package quanta

import (
	"math"
	"testing"
)

func TestSIRegistryContents(t *testing.T) {
	t.Run("Given the registry returned by SI", func(t *testing.T) {
		t.Run("When the promised units are looked for", func(t *testing.T) {
			t.Run("Then every one of them is present with the right dimension", func(t *testing.T) {
				tests := []struct {
					symbol string
					dim    Dimension
				}{
					{"m", Length},
					{"kg", Mass},
					{"s", Time},
					{"A", Current},
					{"K", Temperature},
					{"mol", Amount},
					{"cd", LuminousIntensity},
					{"N", Mass.Mul(Length).Div(Time.Pow(2))},
					{"J", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2))},
					{"W", Mass.Mul(Length.Pow(2)).Div(Time.Pow(3))},
					{"Pa", Mass.Div(Length).Div(Time.Pow(2))},
					{"Hz", Time.Pow(-1)},
					{"C", Current.Mul(Time)},
					{"V", Mass.Mul(Length.Pow(2)).Div(Time.Pow(3)).Div(Current)},
					{"Ω", Mass.Mul(Length.Pow(2)).Div(Time.Pow(3)).Div(Current.Pow(2))},
					{"F", Current.Pow(2).Mul(Time.Pow(4)).Div(Mass).Div(Length.Pow(2))},
					{"L", Length.Pow(3)},
					{"min", Time},
					{"h", Time},
					{"d", Time},
					{"°C", Temperature},
					{"°F", Temperature},
					{"in", Length},
					{"ft", Length},
					{"yd", Length},
					{"mi", Length},
					{"nmi", Length},
					{"g", Mass},
					{"t", Mass},
					{"lb", Mass},
					{"oz", Mass},
					{"st", Mass},
					{"gal_US", Length.Pow(3)},
					{"gal_UK", Length.Pow(3)},
					{"bar", Mass.Div(Length).Div(Time.Pow(2))},
					{"atm", Mass.Div(Length).Div(Time.Pow(2))},
					{"cal", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2))},
					{"Wh", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2))},
					{"eV", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2))},
					{"B", Dimensionless},
					{"bit", Dimensionless},
					{"Δ°C", Temperature},
					{"Δ°F", Temperature},
				}
				for _, tc := range tests {
					t.Run(tc.symbol, func(t *testing.T) {
						got, ok := testSI.Unit(tc.symbol)
						if !ok {
							t.Fatalf("%q is not registered", tc.symbol)
						}
						if got.Dimension() != tc.dim {
							t.Errorf("dimension = %v, want %v", got.Dimension(), tc.dim)
						}
						if got.Name() == "" {
							t.Errorf("%q has no name", tc.symbol)
						}
					})
				}
			})

			t.Run("Then the aliases resolve to the same units", func(t *testing.T) {
				pairs := [][2]string{
					{"meter", "m"}, {"metre", "m"}, {"second", "s"}, {"kelvin", "K"},
					{"ohm", "Ω"}, {"inch", "in"}, {"foot", "ft"}, {"yard", "yd"},
					{"gram", "g"}, {"tonne", "t"}, {"pound", "lb"}, {"litre", "L"},
					{"liter", "L"}, {"l", "L"}, {"minute", "min"}, {"hour", "h"},
					{"day", "d"}, {"degC", "°C"}, {"degF", "°F"},
				}
				for _, pair := range pairs {
					t.Run(pair[0], func(t *testing.T) {
						alias, ok := testSI.Unit(pair[0])
						if !ok {
							t.Fatalf("%q is not registered", pair[0])
						}
						canonical, ok := testSI.Unit(pair[1])
						if !ok {
							t.Fatalf("%q is not registered", pair[1])
						}
						if !alias.Equal(canonical) {
							t.Errorf("%q resolved to %v, want %v", pair[0], alias, canonical)
						}
					})
				}
			})
		})

		t.Run("When derived units are compared with their definitions", func(t *testing.T) {
			t.Run("Then the coherent ones have factor 1", func(t *testing.T) {
				checks := []struct {
					unit string
					expr string
				}{
					{"N", "kg·m/s^2"},
					{"J", "kg·m^2/s^2"},
					{"W", "kg·m^2/s^3"},
					{"Pa", "kg/(m·s^2)"},
					{"Hz", "1/s"},
					{"C", "A·s"},
					{"V", "kg·m^2/(s^3·A)"},
					{"Ω", "kg·m^2/(s^3·A^2)"},
					{"F", "s^4·A^2/(kg·m^2)"},
				}
				for _, tc := range checks {
					t.Run(tc.unit, func(t *testing.T) {
						a := testSI.MustUnit(tc.unit)
						b := testSI.MustParseUnit(tc.expr)
						if a.Dimension() != b.Dimension() {
							t.Errorf("dimension %v != %v", a.Dimension(), b.Dimension())
						}
						if a.Factor().Cmp(b.Factor()) != 0 {
							t.Errorf("factor %v != %v", a.Factor(), b.Factor())
						}
					})
				}
			})
		})

		t.Run("When a physical calculation is performed end to end", func(t *testing.T) {
			t.Run("Then the units carry through", func(t *testing.T) {
				// Kinetic energy of a 1500 kg car at 100 km/h.
				mass := testSI.MustParse("1500 kg")
				speed := testSI.MustParse("100 km/h")
				v2, err := speed.Pow(2)
				if err != nil {
					t.Fatalf("Pow: %v", err)
				}
				energy, err := mass.Mul(v2)
				if err != nil {
					t.Fatalf("Mul: %v", err)
				}
				half, err := energy.Div(testSI.MustParse("2"))
				if err != nil {
					t.Fatalf("Div: %v", err)
				}
				joules := half.MustIn(testSI.MustUnit("J"))
				mps := 100000.0 / 3600
				want := 0.5 * 1500 * mps * mps
				if math.Abs(joules.Value()-want) > 1e-6 {
					t.Errorf("got %v J, want %v J", joules.Value(), want)
				}
				if joules.Dimension() != testSI.MustUnit("J").Dimension() {
					t.Errorf("dimension = %v", joules.Dimension())
				}
			})

			t.Run("Then a rate calculation converts cleanly", func(t *testing.T) {
				distance := testSI.MustParse("42.195 km")
				duration := testSI.MustParse("2 h")
				pace, err := distance.Div(duration)
				if err != nil {
					t.Fatalf("Div: %v", err)
				}
				got := pace.MustIn(testSI.MustParseUnit("m/s"))
				want := 42195.0 / 7200
				if math.Abs(got.Value()-want) > 1e-12 {
					t.Errorf("got %v, want %v", got.Value(), want)
				}
			})
		})
	})
}

func TestRatHelper(t *testing.T) {
	t.Run("Given the internal rational literal helper", func(t *testing.T) {
		t.Run("When it is handed nonsense", func(t *testing.T) {
			t.Run("Then it panics rather than silently producing zero", func(t *testing.T) {
				defer func() {
					if recover() == nil {
						t.Error("rat did not panic")
					}
				}()
				_ = rat("not a number")
			})
		})

		t.Run("When it is handed valid forms", func(t *testing.T) {
			t.Run("Then fractions and decimals both work", func(t *testing.T) {
				if rat("5/9").RatString() != "5/9" {
					t.Errorf("5/9 = %v", rat("5/9"))
				}
				if rat("0.25").RatString() != "1/4" {
					t.Errorf("0.25 = %v", rat("0.25"))
				}
				if rat("1.602176634e-19").Sign() != 1 {
					t.Error("exponent notation failed")
				}
			})
		})
	})
}
