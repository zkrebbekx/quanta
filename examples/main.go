// Command examples works a realistic problem end to end with quanta: sizing a
// heat pump from a house's fabric losses, in a mix of metric, imperial and
// caller-defined units, with the dimensions checked at every step.
package main

import (
	"errors"
	"fmt"
	"log"
	"math/big"

	"github.com/zkrebbekx/quanta"
)

func main() {
	si := quanta.SI()

	// A unit quanta does not ship: the therm, an energy unit still used on UK
	// gas bills. It is exactly 105,505,585.257348 J by definition — the kind
	// of number that has no business being a float64 constant.
	therm, err := si.Define("therm", "thm",
		mustRat("105505585257348/1000000"), si.MustUnit("J"))
	if err != nil {
		log.Fatal(err)
	}

	// --- Fabric heat loss -------------------------------------------------
	// U-values come in W/(m²·K); areas here are a mix of metric and imperial.
	uValue := si.MustParse("0.28 W/(m^2·K)")
	elements := []struct {
		name string
		area string
	}{
		{"walls", "1450 ft^2"},
		{"roof", "78 m^2"},
		{"windows", "14.5 m^2"},
	}

	total := quanta.New(0, si.MustParseUnit("m^2"))
	for _, e := range elements {
		area, err := si.Parse(e.area)
		if err != nil {
			log.Fatal(err)
		}
		total, err = total.Add(area)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%-8s %10s = %s\n", e.name, e.area, area.MustIn(si.MustParseUnit("m^2")).Format(1))
	}
	fmt.Printf("%-8s %10s   %s\n\n", "total", "", total.Format(1))

	// Design temperature difference: inside 21 °C, outside -3 °C. Subtracting
	// two absolute temperatures gives a difference, in delta units.
	inside, outside := si.MustParse("21 °C"), si.MustParse("-3 °C")
	deltaT, err := inside.Sub(outside)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("design ΔT           = %s (%s)\n", deltaT, deltaT.MustIn(si.MustUnit("K")))

	// Heat loss = U · A · ΔT. The units multiply out to watts on their own.
	ua, err := uValue.Mul(total)
	if err != nil {
		log.Fatal(err)
	}
	loss, err := ua.Mul(deltaT)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("fabric heat loss    = %s\n", loss.MustIn(si.MustUnit("kW")).Format(2))

	// --- Running cost -----------------------------------------------------
	// Over a 120-day heating season at a seasonal COP of 3.4.
	season := si.MustParse("120 d")
	heat, err := loss.Mul(season)
	if err != nil {
		log.Fatal(err)
	}
	electricity, err := heat.Div(si.MustParse("3.4"))
	if err != nil {
		log.Fatal(err)
	}
	kWh := electricity.MustIn(si.MustParseUnit("kWh"))
	fmt.Printf("seasonal heat       = %s\n", heat.MustIn(therm).Format(1))
	fmt.Printf("electricity         = %s\n", kWh.Format(0))

	// --- Dimensional analysis catches mistakes ----------------------------
	if _, err := loss.Add(si.MustParse("3 kg")); errors.Is(err, quanta.ErrDimensionMismatch) {
		fmt.Printf("\nrefused             = %v\n", err)
	}
	if _, err := si.ParseUnit("W/(m^2·°C)"); errors.Is(err, quanta.ErrAffineUnit) {
		var pe *quanta.ParseError
		if errors.As(err, &pe) {
			fmt.Printf("refused             = %s (offset %d)\n", pe.Msg, pe.Offset)
		}
	}

	// --- Exactness --------------------------------------------------------
	// A conversion carries an exact rational, so a chain comes back unchanged.
	start := quanta.New(1450, si.MustUnit("ft"))
	back := start.
		MustIn(si.MustUnit("in")).
		MustIn(si.MustUnit("mi")).
		MustIn(si.MustUnit("nmi")).
		MustIn(si.MustUnit("m")).
		MustIn(si.MustUnit("ft"))
	fmt.Printf("\nft→in→mi→nmi→m→ft   = %v (exact: %t)\n", back.Value(), back.Value() == start.Value())
	fmt.Printf("1 in                = %s m exactly\n", si.MustUnit("in").Factor().RatString())
}

func mustRat(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic("bad rational: " + s)
	}
	return r
}
