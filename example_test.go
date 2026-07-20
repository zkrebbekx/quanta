package quanta_test

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/zkrebbekx/quanta"
)

// Example converts a marathon from miles to kilometres.
func Example() {
	si := quanta.SI()

	d, _ := si.Parse("26.2 mi")
	km, _ := d.In(si.MustUnit("km"))

	fmt.Println(km.Format(3))
	// Output: 42.165 km
}

// Example_exactRoundTrip shows that a conversion chain returns the original
// float64: factors are exact rationals, so nothing drifts.
func Example_exactRoundTrip() {
	si := quanta.SI()
	start := quanta.New(3.7, si.MustUnit("mi"))

	there := start.
		MustIn(si.MustUnit("ft")).
		MustIn(si.MustUnit("nmi")).
		MustIn(si.MustUnit("m"))
	back := there.MustIn(si.MustUnit("mi"))

	fmt.Println(back.Value() == start.Value())
	// Output: true
}

// Example_dimensionalAnalysis derives a unit by dividing quantities.
func Example_dimensionalAnalysis() {
	si := quanta.SI()

	distance := si.MustParse("100 m")
	elapsed := si.MustParse("9.58 s")
	speed, _ := distance.Div(elapsed)

	fmt.Println(speed.Format(2))
	fmt.Println(speed.Dimension())
	fmt.Println(speed.MustIn(si.MustParseUnit("km/h")).Format(1))
	// Output:
	// 10.44 m/s
	// L·T⁻¹
	// 37.6 km/h
}

// Example_temperature shows affine conversion and the delta unit that
// subtraction produces.
func Example_temperature() {
	si := quanta.SI()
	freezing := quanta.New(0, si.MustUnit("°C"))

	fmt.Println(freezing.MustIn(si.MustUnit("K")))
	fmt.Println(freezing.MustIn(si.MustUnit("°F")))

	diff, _ := si.MustParse("30 °C").Sub(si.MustParse("10 °C"))
	fmt.Println(diff)
	// Output:
	// 273.15 K
	// 32 °F
	// 20 Δ°C
}

// Example_affineRejected shows that an affine unit cannot enter a compound
// expression.
func Example_affineRejected() {
	si := quanta.SI()

	_, err := si.ParseUnit("°C/m")
	fmt.Println(errors.Is(err, quanta.ErrAffineUnit))

	var pe *quanta.ParseError
	if errors.As(err, &pe) {
		fmt.Println(pe.Offset)
	}
	// Output:
	// true
	// 3
}

// Example_dimensionMismatch shows the error returned when dimensions disagree.
func Example_dimensionMismatch() {
	si := quanta.SI()

	_, err := si.MustParse("5 m").Add(si.MustParse("3 s"))
	fmt.Println(errors.Is(err, quanta.ErrDimensionMismatch))
	fmt.Println(err)
	// Output:
	// true
	// cannot add L and T: quanta: dimension mismatch
}

// Example_customUnit defines a unit the library does not ship and parses an
// expression that uses it.
func Example_customUnit() {
	si := quanta.SI()
	usd, err := si.DefineBase("US dollar", "USD", quanta.Dimensionless)
	if err != nil {
		panic(err)
	}
	// A furlong is exactly 660 feet.
	if _, err := si.Define("furlong", "fur", big.NewRat(660, 1), si.MustUnit("ft")); err != nil {
		panic(err)
	}

	price, _ := si.Parse("640 USD/tonne")
	fmt.Println(price)
	fmt.Println(usd.Dimension().IsDimensionless())
	fmt.Println(si.MustParse("2 fur").MustIn(si.MustUnit("m")).Format(1))
	// Output:
	// 640 USD/t
	// true
	// 402.3 m
}

// Example_binaryPrefix shows IEC prefixes on the information units.
func Example_binaryPrefix() {
	si := quanta.SI()

	size := si.MustParse("1.5 GiB")
	fmt.Println(size.MustIn(si.MustUnit("MiB")))
	fmt.Println(size.MustIn(si.MustParseUnit("MB")).Format(1))
	// Output:
	// 1536 MiB
	// 1610.6 MB
}
