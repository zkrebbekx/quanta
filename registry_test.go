package quanta

import (
	"errors"
	"math/big"
	"sort"
	"sync"
	"testing"
)

func TestRegistryDefinition(t *testing.T) {
	t.Run("Given an empty registry", func(t *testing.T) {
		t.Run("When nothing has been defined", func(t *testing.T) {
			t.Run("Then no symbol resolves", func(t *testing.T) {
				r := NewRegistry()
				if _, ok := r.Unit("m"); ok {
					t.Error("an empty registry knows the metre")
				}
				if len(r.Units()) != 0 {
					t.Errorf("Units() = %v, want empty", r.Units())
				}
				if _, err := r.ParseUnit("m"); !errors.Is(err, ErrUnknownUnit) {
					t.Errorf("got %v, want ErrUnknownUnit", err)
				}
			})
		})

		t.Run("When a base unit and its multiples are defined", func(t *testing.T) {
			r := NewRegistry()
			metre, err := r.DefineBase("metre", "m", Length)
			if err != nil {
				t.Fatalf("DefineBase: %v", err)
			}
			league, err := r.Define("league", "lea", big.NewRat(5556, 1), metre)
			if err != nil {
				t.Fatalf("Define: %v", err)
			}

			t.Run("Then they resolve with the right dimension and factor", func(t *testing.T) {
				if metre.Dimension() != Length || metre.Factor().Cmp(ratOne) != 0 {
					t.Errorf("metre = %+v", metre)
				}
				if league.Dimension() != Length || league.Factor().Cmp(big.NewRat(5556, 1)) != 0 {
					t.Errorf("league = %+v", league)
				}
				if got := New(1, league).MustIn(metre).Value(); got != 5556 {
					t.Errorf("1 lea = %v m, want 5556", got)
				}
			})

			t.Run("Then a caller-defined dimension works too", func(t *testing.T) {
				r2 := NewRegistry()
				bogo, err := r2.DefineBase("bogomip", "bogomip", Dimension{IdxAmount: 1, IdxTime: -1})
				if err != nil {
					t.Fatalf("DefineBase: %v", err)
				}
				if got := bogo.Dimension().String(); got != "T⁻¹·N" {
					t.Errorf("dimension = %q", got)
				}
			})
		})

		t.Run("When a symbol is defined twice", func(t *testing.T) {
			t.Run("Then the second definition is refused", func(t *testing.T) {
				r := NewRegistry()
				m, err := r.DefineBase("metre", "m", Length)
				if err != nil {
					t.Fatalf("DefineBase: %v", err)
				}
				cases := []struct {
					name string
					err  error
				}{
					{"DefineBase", errOfUnit(r.DefineBase("meter", "m", Length))},
					{"Define", errOfUnit(r.Define("mile", "m", big.NewRat(1609, 1), m))},
					{"DefineAffine", errOfUnit(r.DefineAffine("weird", "m", ratOne, ratOne, m))},
					{"Alias", r.Alias("m", m)},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						if !errors.Is(tc.err, ErrDuplicateUnit) {
							t.Errorf("got %v, want ErrDuplicateUnit", tc.err)
						}
					})
				}
			})

			t.Run("Then the original definition survives", func(t *testing.T) {
				r := NewRegistry()
				if _, err := r.DefineBase("metre", "m", Length); err != nil {
					t.Fatalf("DefineBase: %v", err)
				}
				if _, err := r.DefineBase("minute", "m", Time); err == nil {
					t.Fatal("expected an error")
				}
				got, _ := r.Unit("m")
				if got.Dimension() != Length {
					t.Errorf("the metre was overwritten: %v", got.Dimension())
				}
				if n := len(r.Units()); n != 1 {
					t.Errorf("Units() has %d entries, want 1", n)
				}
			})
		})

		t.Run("When a definition is malformed", func(t *testing.T) {
			t.Run("Then it is refused with ErrInvalidUnit", func(t *testing.T) {
				r := NewRegistry()
				m, err := r.DefineBase("metre", "m", Length)
				if err != nil {
					t.Fatalf("DefineBase: %v", err)
				}
				cases := []struct {
					name string
					err  error
				}{
					{"empty base symbol", errOfUnit(r.DefineBase("", "", Length))},
					{"nil factor", errOfUnit(r.Define("x", "x", nil, m))},
					{"zero factor", errOfUnit(r.Define("x", "x", new(big.Rat), m))},
					{"empty derived symbol", errOfUnit(r.Define("x", "", big.NewRat(2, 1), m))},
					{"nil affine factor", errOfUnit(r.DefineAffine("x", "x", nil, ratOne, m))},
					{"zero affine factor", errOfUnit(r.DefineAffine("x", "x", new(big.Rat), ratOne, m))},
					{"empty alias", r.Alias("", m)},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						if !errors.Is(tc.err, ErrInvalidUnit) {
							t.Errorf("got %v, want ErrInvalidUnit", tc.err)
						}
					})
				}
			})

			t.Run("Then an affine base is refused with ErrAffineUnit", func(t *testing.T) {
				r := NewRegistry()
				k, err := r.DefineBase("kelvin", "K", Temperature)
				if err != nil {
					t.Fatalf("DefineBase: %v", err)
				}
				degC, err := r.DefineAffine("degree Celsius", "°C", ratOne, rat("5463/20"), k)
				if err != nil {
					t.Fatalf("DefineAffine: %v", err)
				}
				if _, err := r.Define("nonsense", "n", big.NewRat(2, 1), degC); !errors.Is(err, ErrAffineUnit) {
					t.Errorf("Define on an affine base = %v, want ErrAffineUnit", err)
				}
				if _, err := r.DefineAffine("nonsense", "n", ratOne, ratOne, degC); !errors.Is(err, ErrAffineUnit) {
					t.Errorf("DefineAffine on an affine base = %v, want ErrAffineUnit", err)
				}
			})

			t.Run("Then a nil affine offset is treated as zero", func(t *testing.T) {
				r := NewRegistry()
				k, err := r.DefineBase("kelvin", "K", Temperature)
				if err != nil {
					t.Fatalf("DefineBase: %v", err)
				}
				u, err := r.DefineAffine("rankine", "°R", big.NewRat(5, 9), nil, k)
				if err != nil {
					t.Fatalf("DefineAffine: %v", err)
				}
				if u.IsAffine() {
					t.Error("a zero offset produced an affine unit")
				}
			})
		})
	})
}

func errOfUnit(_ Unit, err error) error { return err }

func TestRegistryLookup(t *testing.T) {
	t.Run("Given the SI registry", func(t *testing.T) {
		t.Run("When a symbol is looked up", func(t *testing.T) {
			t.Run("Then exact symbols, aliases and prefixes all resolve", func(t *testing.T) {
				tests := []struct {
					in     string
					symbol string
					ok     bool
				}{
					{"m", "m", true},
					{"metre", "m", true},
					{"meter", "m", true},
					{"km", "km", true},
					{"µm", "µm", true},
					{"um", "µm", true},
					{"dam", "dam", true},
					{"nope", "", false},
					{"", "", false},
					{"k", "", false},
					{"da", "", false},
				}
				for _, tc := range tests {
					t.Run(tc.in, func(t *testing.T) {
						got, ok := testSI.Unit(tc.in)
						if ok != tc.ok {
							t.Fatalf("ok = %v, want %v", ok, tc.ok)
						}
						if ok && got.Symbol() != tc.symbol {
							t.Errorf("symbol = %q, want %q", got.Symbol(), tc.symbol)
						}
					})
				}
			})

			t.Run("Then MustUnit panics on an unknown symbol", func(t *testing.T) {
				defer func() {
					if recover() == nil {
						t.Error("MustUnit did not panic")
					}
				}()
				_ = testSI.MustUnit("definitely-not-a-unit")
			})

			t.Run("Then MustParseUnit and MustParse panic on bad input", func(t *testing.T) {
				for _, tc := range []struct {
					name string
					run  func()
				}{
					{"MustParseUnit", func() { _ = testSI.MustParseUnit("m/") }},
					{"MustParse", func() { _ = testSI.MustParse("not a quantity") }},
				} {
					t.Run(tc.name, func(t *testing.T) {
						defer func() {
							if recover() == nil {
								t.Error("did not panic")
							}
						}()
						tc.run()
					})
				}
			})
		})

		t.Run("When Units is listed", func(t *testing.T) {
			t.Run("Then the order is sorted by symbol and deterministic", func(t *testing.T) {
				first := testSI.Units()
				if len(first) == 0 {
					t.Fatal("Units() is empty")
				}
				syms := make([]string, len(first))
				for i, u := range first {
					syms[i] = u.Symbol()
				}
				if !sort.StringsAreSorted(syms) {
					t.Errorf("Units() is not sorted: %v", syms)
				}
				for i := 0; i < 20; i++ {
					again := testSI.Units()
					if len(again) != len(first) {
						t.Fatalf("length changed: %d vs %d", len(again), len(first))
					}
					for j := range again {
						if again[j].Symbol() != first[j].Symbol() {
							t.Fatalf("order changed at %d: %q vs %q", j, again[j].Symbol(), first[j].Symbol())
						}
					}
				}
			})

			t.Run("Then aliases are excluded", func(t *testing.T) {
				for _, u := range testSI.Units() {
					if u.Symbol() == "metre" || u.Symbol() == "meter" {
						t.Error("an alias appears in Units()")
					}
				}
			})

			t.Run("Then definition order does not affect listing order", func(t *testing.T) {
				build := func(order []string) []string {
					r := NewRegistry()
					for _, s := range order {
						if _, err := r.DefineBase(s, s, Length); err != nil {
							t.Fatalf("DefineBase: %v", err)
						}
					}
					out := make([]string, 0, len(order))
					for _, u := range r.Units() {
						out = append(out, u.Symbol())
					}
					return out
				}
				a := build([]string{"c", "a", "b"})
				b := build([]string{"b", "c", "a"})
				for i := range a {
					if a[i] != b[i] {
						t.Fatalf("listing differs: %v vs %v", a, b)
					}
				}
			})
		})
	})
}

func TestRegistryConcurrency(t *testing.T) {
	t.Run("Given a registry used from many goroutines", func(t *testing.T) {
		t.Run("When definitions and lookups race", func(t *testing.T) {
			t.Run("Then every definition succeeds exactly once", func(t *testing.T) {
				r := SI()
				var wg sync.WaitGroup
				const n = 16
				errs := make([]error, n)
				for i := 0; i < n; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						_, errs[i] = r.DefineBase("racy", "racy", Length)
					}(i)
					wg.Add(1)
					go func() {
						defer wg.Done()
						_, _ = r.ParseUnit("kg·m/s^2")
						_ = r.Units()
					}()
				}
				wg.Wait()
				successes := 0
				for _, err := range errs {
					if err == nil {
						successes++
					} else if !errors.Is(err, ErrDuplicateUnit) {
						t.Errorf("unexpected error: %v", err)
					}
				}
				if successes != 1 {
					t.Errorf("%d definitions succeeded, want 1", successes)
				}
			})
		})
	})
}

func TestRegistryIndependence(t *testing.T) {
	t.Run("Given two registries from SI", func(t *testing.T) {
		a, b := SI(), SI()

		t.Run("When one is extended", func(t *testing.T) {
			if _, err := a.DefineBase("widget", "wdg", Dimensionless); err != nil {
				t.Fatalf("DefineBase: %v", err)
			}

			t.Run("Then the other is unaffected", func(t *testing.T) {
				if _, ok := b.Unit("wdg"); ok {
					t.Error("a definition leaked between registries")
				}
				if _, ok := a.Unit("wdg"); !ok {
					t.Error("the definition did not take effect")
				}
			})
		})
	})
}
