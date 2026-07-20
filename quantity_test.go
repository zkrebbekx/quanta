package quanta

import (
	"errors"
	"math"
	"math/big"
	"math/rand"
	"strings"
	"testing"
)

// testSI is a registry shared by the tests. Registries are immutable in
// practice once built, and every test that mutates one builds its own.
var testSI = SI()

func u(t *testing.T, symbol string) Unit {
	t.Helper()
	unit, ok := testSI.Unit(symbol)
	if !ok {
		t.Fatalf("unit %q is not registered", symbol)
	}
	return unit
}

func TestQuantityAccessors(t *testing.T) {
	t.Run("Given a quantity built from a value and a unit", func(t *testing.T) {
		q := New(9.81, testSI.MustParseUnit("m/s^2"))

		t.Run("When its parts are read back", func(t *testing.T) {
			t.Run("Then the value, unit and dimension are as given", func(t *testing.T) {
				if q.Value() != 9.81 {
					t.Errorf("Value() = %v, want 9.81", q.Value())
				}
				if got := q.Unit().Symbol(); got != "m/s²" {
					t.Errorf("Unit().Symbol() = %q, want %q", got, "m/s²")
				}
				if got := q.Dimension(); got != Length.Div(Time.Pow(2)) {
					t.Errorf("Dimension() = %v, want L·T⁻²", got)
				}
				if q.IsZero() {
					t.Error("IsZero() = true for a non-zero quantity")
				}
				if !New(0, u(t, "m")).IsZero() {
					t.Error("IsZero() = false for zero metres")
				}
			})
		})

		t.Run("When the zero Quantity is used", func(t *testing.T) {
			t.Run("Then it is the dimensionless number zero", func(t *testing.T) {
				var zero Quantity
				if !zero.IsZero() || !zero.Dimension().IsDimensionless() {
					t.Errorf("zero Quantity = %v (%v)", zero, zero.Dimension())
				}
				if got := zero.String(); got != "0" {
					t.Errorf("String() = %q, want %q", got, "0")
				}
			})
		})
	})
}

func TestExactRoundTrips(t *testing.T) {
	t.Run("Given awkward unit chains with exact rational factors", func(t *testing.T) {
		chains := [][]string{
			{"mi", "ft", "in", "nmi", "m", "mi"},
			{"in", "yd", "mi", "km", "in"},
			{"lb", "oz", "st", "kg", "g", "lb"},
			{"L", "gal_US", "gal_UK", "mL", "m^3", "L"},
			{"Wh", "J", "cal", "eV", "kWh", "Wh"},
		}

		t.Run("When a value is converted around the chain and back", func(t *testing.T) {
			t.Run("Then the original float64 is recovered bit for bit", func(t *testing.T) {
				rng := rand.New(rand.NewSource(20260720))
				for _, chain := range chains {
					t.Run(chain[0], func(t *testing.T) {
						for i := 0; i < 2000; i++ {
							v := rng.Float64() * math.Pow(10, float64(rng.Intn(12)-6))
							q := New(v, testSI.MustParseUnit(chain[0]))
							for _, sym := range chain[1:] {
								q = q.MustIn(testSI.MustParseUnit(sym))
							}
							if q.Value() != v {
								t.Fatalf("chain %v: %v came back as %v", chain, v, q.Value())
							}
						}
					})
				}
			})

			t.Run("Then known conversions have their textbook values", func(t *testing.T) {
				tests := []struct {
					name string
					have string
					in   string
					want float64
				}{
					{"inch to metre", "1 in", "m", 0.0254},
					{"mile to metre", "1 mi", "m", 1609.344},
					{"nautical mile to metre", "1 nmi", "m", 1852},
					{"pound to kilogram", "1 lb", "kg", 0.45359237},
					{"stone to pound", "1 st", "lb", 14},
					{"ounce to gram", "1 oz", "g", 28.349523125},
					{"US gallon to litre", "1 gal_US", "L", 3.785411784},
					{"imperial gallon to litre", "1 gal_UK", "L", 4.54609},
					{"litre to cubic metre", "1 L", "m^3", 0.001},
					{"kWh to joule", "1 kWh", "J", 3600000},
					{"atmosphere to pascal", "1 atm", "Pa", 101325},
					{"bar to pascal", "1 bar", "Pa", 100000},
					{"calorie to joule", "1 cal", "J", 4.184},
					{"day to second", "1 d", "s", 86400},
					{"tonne to kilogram", "1 t", "kg", 1000},
					{"byte to bit", "1 B", "bit", 8},
					{"GiB to MiB", "1 GiB", "MiB", 1024},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						q := testSI.MustParse(tc.have).MustIn(testSI.MustParseUnit(tc.in))
						if q.Value() != tc.want {
							t.Errorf("%s in %s = %v, want %v", tc.have, tc.in, q.Value(), tc.want)
						}
					})
				}
			})
		})

		t.Run("When a very long chain is walked", func(t *testing.T) {
			t.Run("Then error still does not accumulate", func(t *testing.T) {
				const v = 1.2345678901234567
				q := New(v, u(t, "m"))
				hops := []string{"in", "ft", "yd", "mi", "nmi", "km", "mm", "m"}
				for i := 0; i < 100; i++ {
					q = q.MustIn(u(t, hops[i%len(hops)]))
				}
				if got := q.MustIn(u(t, "m")).Value(); got != v {
					t.Errorf("after 100 hops got %v, want %v", got, v)
				}
			})
		})
	})
}

func TestAffineConversion(t *testing.T) {
	t.Run("Given the affine temperature scales", func(t *testing.T) {
		degC, degF, kelvin := u(t, "°C"), u(t, "°F"), u(t, "K")

		t.Run("When absolute temperatures are converted", func(t *testing.T) {
			t.Run("Then the zero points line up", func(t *testing.T) {
				tests := []struct {
					name string
					have Quantity
					in   Unit
					want float64
					// tol is non-zero where the input decimal is not exactly
					// representable as a float64, so the exact answer is a few
					// ulps off the textbook one.
					tol float64
				}{
					{"0 °C in K", New(0, degC), kelvin, 273.15, 0},
					{"0 °C in °F", New(0, degC), degF, 32, 0},
					{"32 °F in °C", New(32, degF), degC, 0, 0},
					{"32 °F in K", New(32, degF), kelvin, 273.15, 0},
					{"273.15 K in °C", New(273.15, kelvin), degC, 0, 1e-12},
					{"-40 °C in °F", New(-40, degC), degF, -40, 0},
					{"-40 °F in °C", New(-40, degF), degC, -40, 0},
					{"100 °C in °F", New(100, degC), degF, 212, 0},
					{"212 °F in K", New(212, degF), kelvin, 373.15, 0},
					{"0 K in °C", New(0, kelvin), degC, -273.15, 0},
					{"absolute zero in °F", New(0, kelvin), degF, -459.67, 1e-12},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						got := tc.have.MustIn(tc.in).Value()
						if math.Abs(got-tc.want) > tc.tol {
							t.Errorf("got %v, want %v (tolerance %v)", got, tc.want, tc.tol)
						}
					})
				}
			})

			t.Run("Then round trips are exact", func(t *testing.T) {
				rng := rand.New(rand.NewSource(7))
				for i := 0; i < 5000; i++ {
					v := rng.Float64()*600 - 300
					q := New(v, degC)
					if got := q.MustIn(degF).MustIn(kelvin).MustIn(degC).Value(); got != v {
						t.Fatalf("%v °C came back as %v", v, got)
					}
				}
			})
		})

		t.Run("When affinity is queried", func(t *testing.T) {
			t.Run("Then only the offset units report it", func(t *testing.T) {
				for _, sym := range []string{"°C", "°F", "degC", "degF"} {
					if !u(t, sym).IsAffine() {
						t.Errorf("%s is not affine", sym)
					}
				}
				for _, sym := range []string{"K", "Δ°C", "Δ°F", "m", "s"} {
					if u(t, sym).IsAffine() {
						t.Errorf("%s is affine", sym)
					}
				}
			})
		})
	})
}

func TestAffineRejection(t *testing.T) {
	t.Run("Given a quantity in an affine unit", func(t *testing.T) {
		hot := New(20, u(t, "°C"))
		metre := New(2, u(t, "m"))

		t.Run("When it is used in multiplicative arithmetic", func(t *testing.T) {
			t.Run("Then the operation fails with ErrAffineUnit", func(t *testing.T) {
				ops := []struct {
					name string
					run  func() (Quantity, error)
				}{
					{"affine × linear", func() (Quantity, error) { return hot.Mul(metre) }},
					{"linear × affine", func() (Quantity, error) { return metre.Mul(hot) }},
					{"affine ÷ linear", func() (Quantity, error) { return hot.Div(metre) }},
					{"linear ÷ affine", func() (Quantity, error) { return metre.Div(hot) }},
					{"affine²", func() (Quantity, error) { return hot.Pow(2) }},
					{"affine⁻¹", func() (Quantity, error) { return hot.Pow(-1) }},
					{"affine⁰", func() (Quantity, error) { return hot.Pow(0) }},
					{"affine + affine", func() (Quantity, error) { return hot.Add(hot) }},
					{"linear + affine", func() (Quantity, error) { return metre.Add(New(1, u(t, "K"))) }},
				}
				for _, op := range ops {
					t.Run(op.name, func(t *testing.T) {
						_, err := op.run()
						if err == nil {
							t.Fatal("expected an error")
						}
						if op.name == "linear + affine" {
							// Dimensions differ here; checked separately below.
							return
						}
						if !errors.Is(err, ErrAffineUnit) {
							t.Errorf("got %v, want ErrAffineUnit", err)
						}
					})
				}
			})

			t.Run("Then the error names the dimensionless operand readably", func(t *testing.T) {
				_, err := New(2, One).Mul(hot)
				if !errors.Is(err, ErrAffineUnit) {
					t.Fatalf("got %v, want ErrAffineUnit", err)
				}
				if !strings.Contains(err.Error(), "dimensionless") {
					t.Errorf("error = %q, want it to mention the dimensionless operand", err)
				}
			})

			t.Run("Then a linear operand may not absorb an affine one", func(t *testing.T) {
				kelvin := New(5, u(t, "K"))
				for _, tc := range []struct {
					name string
					run  func() (Quantity, error)
				}{
					{"K + °C", func() (Quantity, error) { return kelvin.Add(hot) }},
					{"K - °C", func() (Quantity, error) { return kelvin.Sub(hot) }},
				} {
					t.Run(tc.name, func(t *testing.T) {
						if _, err := tc.run(); !errors.Is(err, ErrAffineUnit) {
							t.Errorf("got %v, want ErrAffineUnit", err)
						}
					})
				}
			})
		})

		t.Run("When it appears in a compound unit expression", func(t *testing.T) {
			t.Run("Then parsing fails with ErrAffineUnit", func(t *testing.T) {
				for _, expr := range []string{"°C/m", "m·°C", "°C^2", "°C²", "1/°C", "(°C)/s", "°F·m"} {
					t.Run(expr, func(t *testing.T) {
						if _, err := testSI.ParseUnit(expr); !errors.Is(err, ErrAffineUnit) {
							t.Errorf("ParseUnit(%q) = %v, want ErrAffineUnit", expr, err)
						}
					})
				}
			})

			t.Run("Then it may still stand alone", func(t *testing.T) {
				got, err := testSI.ParseUnit("°C")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !got.IsAffine() {
					t.Error("standalone °C lost its offset")
				}
			})
		})

		t.Run("When it is prefixed", func(t *testing.T) {
			t.Run("Then the prefix is refused", func(t *testing.T) {
				if _, err := testSI.ParseUnit("m°C"); !errors.Is(err, ErrUnknownUnit) {
					t.Errorf("got %v, want ErrUnknownUnit", err)
				}
			})
		})
	})
}

func TestTemperatureDifferences(t *testing.T) {
	t.Run("Given two absolute temperatures", func(t *testing.T) {
		hot, cold := New(30, u(t, "°C")), New(10, u(t, "°C"))

		t.Run("When one is subtracted from the other", func(t *testing.T) {
			diff, err := hot.Sub(cold)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			t.Run("Then the result is a difference in the delta unit", func(t *testing.T) {
				if diff.Value() != 20 {
					t.Errorf("value = %v, want 20", diff.Value())
				}
				if got := diff.Unit().Symbol(); got != "Δ°C" {
					t.Errorf("symbol = %q, want %q", got, "Δ°C")
				}
				if diff.Unit().IsAffine() {
					t.Error("the delta unit is affine")
				}
				if got := diff.MustIn(u(t, "K")).Value(); got != 20 {
					t.Errorf("in kelvin = %v, want 20", got)
				}
			})

			t.Run("Then it matches the unit registered by SI", func(t *testing.T) {
				if !diff.Unit().Equal(u(t, "Δ°C")) {
					t.Errorf("%v is not the registered Δ°C", diff.Unit())
				}
			})
		})

		t.Run("When a difference is added to an absolute temperature", func(t *testing.T) {
			t.Run("Then the result stays absolute on the same scale", func(t *testing.T) {
				tests := []struct {
					name  string
					delta Quantity
					want  float64
				}{
					{"Δ°C", New(5, u(t, "Δ°C")), 15},
					{"kelvin", New(5, u(t, "K")), 15},
					{"Δ°F", New(9, u(t, "Δ°F")), 15},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						got, err := cold.Add(tc.delta)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						if got.Value() != tc.want || got.Unit().Symbol() != "°C" {
							t.Errorf("got %v, want %v °C", got, tc.want)
						}
					})
				}
			})

			t.Run("Then subtracting a difference walks back down the scale", func(t *testing.T) {
				got, err := cold.Sub(New(5, u(t, "K")))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got.Value() != 5 || got.Unit().Symbol() != "°C" {
					t.Errorf("got %v, want 5 °C", got)
				}
			})
		})

		t.Run("When Delta is taken of a linear unit", func(t *testing.T) {
			t.Run("Then the unit is returned unchanged", func(t *testing.T) {
				m := u(t, "m")
				if !m.Delta().Equal(m) {
					t.Errorf("Delta() changed a linear unit: %v", m.Delta())
				}
			})
		})
	})
}

func TestQuantityArithmetic(t *testing.T) {
	t.Run("Given quantities of compatible dimensions", func(t *testing.T) {
		t.Run("When they are added or subtracted", func(t *testing.T) {
			t.Run("Then the result is expressed in the receiver's unit", func(t *testing.T) {
				tests := []struct {
					name     string
					a, b     string
					add, sub float64
					unit     string
				}{
					{"same unit", "2 m", "3 m", 5, -1, "m"},
					{"mixed scale", "1 km", "500 m", 1.5, 0.5, "km"},
					{"imperial into metric", "1 m", "1 ft", 1.3048, 0.6952, "m"},
					{"receiver keeps its unit", "1 ft", "1 in", 1 + 1.0/12, 1 - 1.0/12, "ft"},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						a, b := testSI.MustParse(tc.a), testSI.MustParse(tc.b)
						sum, err := a.Add(b)
						if err != nil {
							t.Fatalf("Add: %v", err)
						}
						if math.Abs(sum.Value()-tc.add) > 1e-12 || sum.Unit().Symbol() != tc.unit {
							t.Errorf("Add = %v, want %v %s", sum, tc.add, tc.unit)
						}
						diff, err := a.Sub(b)
						if err != nil {
							t.Fatalf("Sub: %v", err)
						}
						if math.Abs(diff.Value()-tc.sub) > 1e-12 || diff.Unit().Symbol() != tc.unit {
							t.Errorf("Sub = %v, want %v %s", diff, tc.sub, tc.unit)
						}
					})
				}
			})
		})

		t.Run("When they are multiplied or divided", func(t *testing.T) {
			t.Run("Then dimensions combine and units compound", func(t *testing.T) {
				tests := []struct {
					name       string
					a, b       string
					op         string
					wantValue  float64
					wantSymbol string
					wantDim    Dimension
				}{
					{"area", "3 m", "4 m", "*", 12, "m²", Length.Pow(2)},
					{"speed", "100 m", "9.58 s", "/", 100.0 / 9.58, "m/s", Length.Div(Time)},
					{"torque", "10 N", "2 m", "*", 20, "N·m", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2))},
					{"frequency", "1 ", "2 s", "/", 0.5, "1/s", Time.Pow(-1)},
					{"cancelling", "6 m", "2 m", "/", 3, "", Dimensionless},
					{"scale kept when symbols differ", "1 km", "1 m", "/", 1, "km/m", Dimensionless},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						a, b := testSI.MustParse(tc.a), testSI.MustParse(tc.b)
						var (
							got Quantity
							err error
						)
						if tc.op == "*" {
							got, err = a.Mul(b)
						} else {
							got, err = a.Div(b)
						}
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						if math.Abs(got.Value()-tc.wantValue) > 1e-12 {
							t.Errorf("value = %v, want %v", got.Value(), tc.wantValue)
						}
						if got.Unit().Symbol() != tc.wantSymbol {
							t.Errorf("symbol = %q, want %q", got.Unit().Symbol(), tc.wantSymbol)
						}
						if got.Dimension() != tc.wantDim {
							t.Errorf("dimension = %v, want %v", got.Dimension(), tc.wantDim)
						}
					})
				}
			})

			t.Run("Then a fully cancelled product keeps its magnitude", func(t *testing.T) {
				// 1 km / 1 m is dimensionless but is 1000, not 1.
				q, err := testSI.MustParse("1 km").Div(testSI.MustParse("1 m"))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got := q.MustIn(One).Value(); got != 1000 {
					t.Errorf("in the unit one = %v, want 1000", got)
				}
			})
		})

		t.Run("When they are raised to a power", func(t *testing.T) {
			t.Run("Then value and unit follow", func(t *testing.T) {
				tests := []struct {
					name       string
					have       string
					n          int
					wantValue  float64
					wantSymbol string
				}{
					{"square", "3 m", 2, 9, "m²"},
					{"cube", "2 m", 3, 8, "m³"},
					{"inverse", "4 s", -1, 0.25, "1/s"},
					{"zeroth", "7 m", 0, 1, ""},
					{"first", "7 m", 1, 7, "m"},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						got, err := testSI.MustParse(tc.have).Pow(tc.n)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						if got.Value() != tc.wantValue || got.Unit().Symbol() != tc.wantSymbol {
							t.Errorf("got %v, want %v %s", got, tc.wantValue, tc.wantSymbol)
						}
					})
				}
			})

			t.Run("Then the exact factor is carried into conversions", func(t *testing.T) {
				areaInFeet, err := testSI.MustParse("3 ft").Pow(2)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				got := areaInFeet.MustIn(testSI.MustParseUnit("m^2"))
				want := 9 * 0.3048 * 0.3048
				if math.Abs(got.Value()-want) > 1e-15 {
					t.Errorf("got %v, want %v", got.Value(), want)
				}
			})
		})

		t.Run("When Neg and Abs are applied", func(t *testing.T) {
			t.Run("Then only the magnitude changes", func(t *testing.T) {
				q := testSI.MustParse("-3 m")
				if got := q.Neg(); got.Value() != 3 || got.Unit().Symbol() != "m" {
					t.Errorf("Neg = %v", got)
				}
				if got := q.Abs(); got.Value() != 3 || got.Unit().Symbol() != "m" {
					t.Errorf("Abs = %v", got)
				}
				if got := testSI.MustParse("3 m").Abs(); got.Value() != 3 {
					t.Errorf("Abs of a positive value = %v", got)
				}
			})
		})
	})
}

func TestExponentRange(t *testing.T) {
	t.Run("Given a unit with a very large dimension exponent", func(t *testing.T) {
		big100 := testSI.MustParseUnit("m^100")

		t.Run("When an operation would push it past int8", func(t *testing.T) {
			t.Run("Then it fails with ErrExponentRange", func(t *testing.T) {
				q := New(1, big100)
				ops := []struct {
					name string
					run  func() (Quantity, error)
				}{
					{"Pow", func() (Quantity, error) { return q.Pow(2) }},
					{"Mul", func() (Quantity, error) { return q.Mul(q) }},
					{"Div", func() (Quantity, error) { return New(1, testSI.MustParseUnit("1/m^100")).Div(q) }},
				}
				for _, op := range ops {
					t.Run(op.name, func(t *testing.T) {
						if _, err := op.run(); !errors.Is(err, ErrExponentRange) {
							t.Errorf("got %v, want ErrExponentRange", err)
						}
					})
				}
			})
		})
	})
}

func TestDimensionMismatch(t *testing.T) {
	t.Run("Given quantities of different dimensions", func(t *testing.T) {
		length, duration := testSI.MustParse("5 m"), testSI.MustParse("3 s")

		t.Run("When they are combined or compared", func(t *testing.T) {
			t.Run("Then the operation fails with ErrDimensionMismatch", func(t *testing.T) {
				ops := []struct {
					name string
					err  error
				}{
					{"Add", errOf(length.Add(duration))},
					{"Sub", errOf(length.Sub(duration))},
					{"In", errOf(length.In(u(t, "s")))},
					{"Cmp", errOfCmp(length.Cmp(duration))},
				}
				for _, op := range ops {
					t.Run(op.name, func(t *testing.T) {
						if !errors.Is(op.err, ErrDimensionMismatch) {
							t.Errorf("got %v, want ErrDimensionMismatch", op.err)
						}
					})
				}
			})

			t.Run("Then Equal is false rather than an error", func(t *testing.T) {
				if length.Equal(duration) {
					t.Error("a length compared equal to a time")
				}
			})

			t.Run("Then MustIn panics", func(t *testing.T) {
				defer func() {
					if recover() == nil {
						t.Error("MustIn did not panic")
					}
				}()
				_ = length.MustIn(u(t, "s"))
			})
		})
	})
}

func errOf(_ Quantity, err error) error { return err }
func errOfCmp(_ int, err error) error   { return err }

func TestComparison(t *testing.T) {
	t.Run("Given quantities of the same dimension in different units", func(t *testing.T) {
		t.Run("When they are compared", func(t *testing.T) {
			t.Run("Then scale is taken into account", func(t *testing.T) {
				tests := []struct {
					name string
					a, b string
					want int
				}{
					{"km beats m", "1 km", "999 m", 1},
					{"m loses to km", "999 m", "1 km", -1},
					{"exactly equal", "1 km", "1000 m", 0},
					{"same unit less", "1 m", "2 m", -1},
					{"affine equal", "273.15 K", "0 °C", 0},
					{"affine less", "0 °C", "274 K", -1},
					{"affine greater", "100 °C", "212 K", 1},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						a, b := testSI.MustParse(tc.a), testSI.MustParse(tc.b)
						got, err := a.Cmp(b)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						if got != tc.want {
							t.Errorf("Cmp = %d, want %d", got, tc.want)
						}
						if wantEqual := tc.want == 0; a.Equal(b) != wantEqual {
							t.Errorf("Equal = %v, want %v", a.Equal(b), wantEqual)
						}
					})
				}
			})

			t.Run("Then NaN compares as equal, as float64 does", func(t *testing.T) {
				nan := New(math.NaN(), u(t, "m"))
				got, err := nan.Cmp(testSI.MustParse("1 m"))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != 0 {
					t.Errorf("Cmp with NaN = %d, want 0", got)
				}
				if nan.Equal(testSI.MustParse("1 m")) {
					t.Error("NaN compared equal to 1 m")
				}
			})
		})
	})
}

func TestNonFiniteValues(t *testing.T) {
	t.Run("Given a non-finite magnitude", func(t *testing.T) {
		t.Run("When it is converted", func(t *testing.T) {
			t.Run("Then the conversion falls back to float64 and preserves it", func(t *testing.T) {
				tests := []struct {
					name string
					v    float64
					want func(float64) bool
				}{
					{"positive infinity", math.Inf(1), func(f float64) bool { return math.IsInf(f, 1) }},
					{"negative infinity", math.Inf(-1), func(f float64) bool { return math.IsInf(f, -1) }},
					{"NaN", math.NaN(), math.IsNaN},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						got := New(tc.v, u(t, "km")).MustIn(u(t, "m"))
						if !tc.want(got.Value()) {
							t.Errorf("got %v", got.Value())
						}
					})
				}
			})

			t.Run("Then an affine conversion also survives", func(t *testing.T) {
				got := New(math.Inf(1), u(t, "°C")).MustIn(u(t, "K"))
				if !math.IsInf(got.Value(), 1) {
					t.Errorf("got %v, want +Inf", got.Value())
				}
			})

			t.Run("Then a converted infinity still compares", func(t *testing.T) {
				inf := New(math.Inf(1), u(t, "m"))
				got, err := inf.Cmp(testSI.MustParse("1 m"))
				if err != nil || got != 1 {
					t.Errorf("Cmp = %d, %v; want 1, nil", got, err)
				}
				if inf.Equal(testSI.MustParse("1 m")) {
					t.Error("infinity compared equal to 1 m")
				}
			})
		})
	})
}

func TestQuantityFormatting(t *testing.T) {
	t.Run("Given quantities to render", func(t *testing.T) {
		t.Run("When String and Format are called", func(t *testing.T) {
			t.Run("Then the magnitude and symbol are joined by a space", func(t *testing.T) {
				tests := []struct {
					name   string
					q      Quantity
					prec   int
					str    string
					format string
				}{
					{"acceleration", New(9.80665, testSI.MustParseUnit("m/s^2")), 2, "9.80665 m/s²", "9.81 m/s²"},
					{"whole number", New(3, u(t, "m")), 1, "3 m", "3.0 m"},
					{"dimensionless", New(2.5, One), 3, "2.5", "2.500"},
					{"negative", New(-40, u(t, "°C")), 0, "-40 °C", "-40 °C"},
					{"exponent", New(1e21, u(t, "m")), -1, "1e+21 m", "1000000000000000000000 m"},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						if got := tc.q.String(); got != tc.str {
							t.Errorf("String = %q, want %q", got, tc.str)
						}
						if got := tc.q.Format(tc.prec); got != tc.format {
							t.Errorf("Format(%d) = %q, want %q", tc.prec, got, tc.format)
						}
					})
				}
			})
		})

		t.Run("When marshalled as text", func(t *testing.T) {
			t.Run("Then the text matches String", func(t *testing.T) {
				q := New(9.81, testSI.MustParseUnit("m/s^2"))
				b, err := q.MarshalText()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if string(b) != q.String() {
					t.Errorf("MarshalText = %q, want %q", b, q.String())
				}
			})

			t.Run("Then it round-trips through the registry parser", func(t *testing.T) {
				q := New(9.81, testSI.MustParseUnit("m/s^2"))
				b, _ := q.MarshalText()
				back, err := testSI.Parse(string(b))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !back.Equal(q) {
					t.Errorf("round trip gave %v, want %v", back, q)
				}
			})
		})
	})
}

func TestUnitInternals(t *testing.T) {
	t.Run("Given the unexported unit helpers", func(t *testing.T) {
		t.Run("When a degenerate factor reaches them", func(t *testing.T) {
			t.Run("Then they report an invalid unit rather than dividing by zero", func(t *testing.T) {
				zero := Unit{symbol: "z", factor: new(big.Rat)}
				if _, err := combine(u(t, "m"), zero, -1); !errors.Is(err, ErrInvalidUnit) {
					t.Errorf("combine by a zero-factor unit = %v, want ErrInvalidUnit", err)
				}
				if _, err := powUnit(zero, -1); !errors.Is(err, ErrInvalidUnit) {
					t.Errorf("powUnit by a zero-factor unit = %v, want ErrInvalidUnit", err)
				}
			})
		})

		t.Run("When ratPow is asked for the zeroth power", func(t *testing.T) {
			t.Run("Then it returns exactly one", func(t *testing.T) {
				got, err := ratPow(big.NewRat(7, 3), 0)
				if err != nil || got.Cmp(ratOne) != 0 {
					t.Errorf("ratPow(7/3, 0) = %v, %v", got, err)
				}
			})
		})

		t.Run("When accessors are used on the zero Unit", func(t *testing.T) {
			t.Run("Then it behaves as the dimensionless unit one", func(t *testing.T) {
				var z Unit
				if z.Factor().Cmp(ratOne) != 0 {
					t.Errorf("Factor = %v, want 1", z.Factor())
				}
				if z.Offset().Sign() != 0 {
					t.Errorf("Offset = %v, want 0", z.Offset())
				}
				if z.Symbol() != "" || z.Name() != "" || z.IsAffine() || !z.Dimension().IsDimensionless() {
					t.Errorf("zero Unit is not the unit one: %+v", z)
				}
				if got := New(2, z).String(); got != "2" {
					t.Errorf("New(2, Unit{}) = %q, want %q", got, "2")
				}
				if !z.Equal(One) {
					t.Error("the zero Unit is not One")
				}
			})

			t.Run("Then the returned factor is a copy the caller may mutate", func(t *testing.T) {
				m := u(t, "km")
				f := m.Factor()
				f.SetInt64(999)
				if m.Factor().Cmp(big.NewRat(1000, 1)) != 0 {
					t.Errorf("mutating the returned factor changed the unit: %v", m.Factor())
				}
				o := u(t, "°C")
				off := o.Offset()
				off.SetInt64(999)
				if o.Offset().Cmp(rat("5463/20")) != 0 {
					t.Errorf("mutating the returned offset changed the unit: %v", o.Offset())
				}
			})
		})

		t.Run("When units are compared", func(t *testing.T) {
			t.Run("Then Equal is by symbol, dimension, factor and offset", func(t *testing.T) {
				if !u(t, "m").Equal(u(t, "metre")) {
					t.Error("an alias is not equal to its unit")
				}
				if u(t, "m").Equal(u(t, "km")) {
					t.Error("m equals km")
				}
				if u(t, "°C").Equal(u(t, "Δ°C")) {
					t.Error("°C equals Δ°C")
				}
				if got := u(t, "m").String(); got != "m" {
					t.Errorf("String = %q, want %q", got, "m")
				}
				if got := u(t, "m").Name(); got != "metre" {
					t.Errorf("Name = %q, want %q", got, "metre")
				}
			})
		})
	})
}
