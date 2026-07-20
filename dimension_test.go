package quanta

import "testing"

func TestDimensionAlgebra(t *testing.T) {
	t.Run("Given the seven base dimensions", func(t *testing.T) {
		t.Run("When they are combined", func(t *testing.T) {
			t.Run("Then exponents add and subtract element-wise", func(t *testing.T) {
				tests := []struct {
					name string
					got  Dimension
					want Dimension
				}{
					{"velocity", Length.Div(Time), Dimension{IdxLength: 1, IdxTime: -1}},
					{"acceleration", Length.Div(Time.Pow(2)), Dimension{IdxLength: 1, IdxTime: -2}},
					{"area", Length.Mul(Length), Dimension{IdxLength: 2}},
					{"force", Mass.Mul(Length).Div(Time.Pow(2)), Dimension{IdxLength: 1, IdxMass: 1, IdxTime: -2}},
					{"cancellation", Length.Div(Length), Dimensionless},
					{"zeroth power", Length.Pow(0), Dimensionless},
					{"negative power", Time.Pow(-1), Dimension{IdxTime: -1}},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						if tc.got != tc.want {
							t.Errorf("got %v, want %v", tc.got, tc.want)
						}
					})
				}
			})

			t.Run("Then the result is comparable and usable as a map key", func(t *testing.T) {
				m := map[Dimension]string{
					Length.Div(Time): "velocity",
					Mass:             "mass",
				}
				if got := m[Length.Div(Time)]; got != "velocity" {
					t.Errorf("got %q, want %q", got, "velocity")
				}
				if Length == Mass {
					t.Error("distinct base dimensions compared equal")
				}
			})
		})

		t.Run("When IsDimensionless is asked", func(t *testing.T) {
			t.Run("Then only the zero dimension qualifies", func(t *testing.T) {
				if !Dimensionless.IsDimensionless() {
					t.Error("Dimensionless is not dimensionless")
				}
				if !(Dimension{}).IsDimensionless() {
					t.Error("the zero value is not dimensionless")
				}
				if Length.IsDimensionless() {
					t.Error("Length reported as dimensionless")
				}
			})
		})
	})
}

func TestDimensionExponentSaturation(t *testing.T) {
	t.Run("Given a dimension near the int8 limit", func(t *testing.T) {
		big := Dimension{IdxLength: 120}

		t.Run("When the unchecked operations overflow", func(t *testing.T) {
			t.Run("Then the exponent saturates rather than wrapping", func(t *testing.T) {
				tests := []struct {
					name string
					got  int8
					want int8
				}{
					{"Mul saturates high", big.Mul(big)[IdxLength], 127},
					{"Div saturates low", Dimension{IdxLength: -120}.Div(big)[IdxLength], -128},
					{"Pow saturates high", big.Pow(3)[IdxLength], 127},
					{"Pow saturates low", big.Pow(-3)[IdxLength], -128},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						if tc.got != tc.want {
							t.Errorf("got %d, want %d", tc.got, tc.want)
						}
					})
				}
			})
		})

		t.Run("When the checked operations overflow", func(t *testing.T) {
			t.Run("Then they report it", func(t *testing.T) {
				if _, ok := big.CheckedMul(big); ok {
					t.Error("CheckedMul reported no overflow")
				}
				neg := Dimension{IdxLength: -120}
				if _, ok := neg.CheckedDiv(big); ok {
					t.Error("CheckedDiv reported no overflow")
				}
				if _, ok := big.CheckedPow(3); ok {
					t.Error("CheckedPow reported no overflow")
				}
				if _, ok := big.CheckedPow(-3); ok {
					t.Error("CheckedPow reported no overflow for a negative power")
				}
			})
		})

		t.Run("When the checked operations stay in range", func(t *testing.T) {
			t.Run("Then they report success and match the unchecked form", func(t *testing.T) {
				for _, tc := range []struct {
					name string
					got  Dimension
					ok   bool
					want Dimension
				}{
					{"mul", mustChecked(Length.CheckedMul(Time)), true, Length.Mul(Time)},
					{"div", mustChecked(Length.CheckedDiv(Time)), true, Length.Div(Time)},
					{"pow", mustChecked(Length.CheckedPow(3)), true, Length.Pow(3)},
				} {
					t.Run(tc.name, func(t *testing.T) {
						if tc.got != tc.want {
							t.Errorf("got %v, want %v", tc.got, tc.want)
						}
					})
				}
				if _, ok := Length.CheckedMul(Time); !ok {
					t.Error("CheckedMul reported a spurious overflow")
				}
				if _, ok := Length.CheckedDiv(Time); !ok {
					t.Error("CheckedDiv reported a spurious overflow")
				}
				if _, ok := Length.CheckedPow(3); !ok {
					t.Error("CheckedPow reported a spurious overflow")
				}
			})
		})
	})
}

func mustChecked(d Dimension, _ bool) Dimension { return d }

func TestDimensionString(t *testing.T) {
	t.Run("Given dimensions of familiar quantities", func(t *testing.T) {
		t.Run("When they are rendered", func(t *testing.T) {
			t.Run("Then the ordering is M·L·T·I·Θ·N·J and exponents are superscripts", func(t *testing.T) {
				tests := []struct {
					name string
					d    Dimension
					want string
				}{
					{"dimensionless", Dimensionless, "1"},
					{"length", Length, "L"},
					{"mass", Mass, "M"},
					{"time", Time, "T"},
					{"current", Current, "I"},
					{"temperature", Temperature, "Θ"},
					{"amount", Amount, "N"},
					{"luminous intensity", LuminousIntensity, "J"},
					{"acceleration", Length.Div(Time.Pow(2)), "L·T⁻²"},
					{"energy", Mass.Mul(Length.Pow(2)).Div(Time.Pow(2)), "M·L²·T⁻²"},
					{"volume", Length.Pow(3), "L³"},
					{"two-digit exponent", Length.Pow(12), "L¹²"},
					{"two-digit negative", Length.Pow(-12), "L⁻¹²"},
					{"saturated", Length.Pow(127), "L¹²⁷"},
					{"every base", Length.Mul(Mass).Mul(Time).Mul(Current).
						Mul(Temperature).Mul(Amount).Mul(LuminousIntensity), "M·L·T·I·Θ·N·J"},
				}
				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						if got := tc.d.String(); got != tc.want {
							t.Errorf("got %q, want %q", got, tc.want)
						}
					})
				}
			})

			t.Run("Then the rendering is deterministic across repeated calls", func(t *testing.T) {
				d := Mass.Mul(Length.Pow(2)).Div(Time.Pow(3)).Div(Current)
				first := d.String()
				for i := 0; i < 100; i++ {
					if got := d.String(); got != first {
						t.Fatalf("call %d gave %q, want %q", i, got, first)
					}
				}
			})
		})
	})
}

func TestSuperscript(t *testing.T) {
	t.Run("Given an integer exponent", func(t *testing.T) {
		t.Run("When it is rendered as a superscript", func(t *testing.T) {
			t.Run("Then digits and sign are translated", func(t *testing.T) {
				tests := []struct {
					n    int
					want string
				}{
					{0, "⁰"}, {1, "¹"}, {5, "⁵"}, {10, "¹⁰"},
					{-1, "⁻¹"}, {-42, "⁻⁴²"}, {1234567890, "¹²³⁴⁵⁶⁷⁸⁹⁰"},
				}
				for _, tc := range tests {
					if got := superscript(tc.n); got != tc.want {
						t.Errorf("superscript(%d) = %q, want %q", tc.n, got, tc.want)
					}
				}
			})
		})
	})
}
