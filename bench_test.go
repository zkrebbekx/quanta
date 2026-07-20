package quanta

import "testing"

var (
	benchSI       = SI()
	benchQuantity Quantity
	benchUnit     Unit
	benchString   string
	benchBool     bool
)

func BenchmarkConvertSameDimension(b *testing.B) {
	from, to := benchSI.MustUnit("mi"), benchSI.MustUnit("m")
	q := New(26.2, from)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchQuantity = q.MustIn(to)
	}
}

func BenchmarkConvertChain(b *testing.B) {
	q := New(26.2, benchSI.MustUnit("mi"))
	ft, nmi, m := benchSI.MustUnit("ft"), benchSI.MustUnit("nmi"), benchSI.MustUnit("m")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchQuantity = q.MustIn(ft).MustIn(nmi).MustIn(m)
	}
}

func BenchmarkConvertAffine(b *testing.B) {
	degC, degF := benchSI.MustUnit("°C"), benchSI.MustUnit("°F")
	q := New(21.5, degC)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchQuantity = q.MustIn(degF)
	}
}

func BenchmarkAdd(b *testing.B) {
	x := New(1, benchSI.MustUnit("km"))
	y := New(500, benchSI.MustUnit("m"))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchQuantity, _ = x.Add(y)
	}
}

func BenchmarkMul(b *testing.B) {
	x := New(3, benchSI.MustUnit("m"))
	y := New(4, benchSI.MustUnit("s"))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchQuantity, _ = x.Mul(y)
	}
}

func BenchmarkCmp(b *testing.B) {
	x := New(1, benchSI.MustUnit("km"))
	y := New(999, benchSI.MustUnit("m"))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchBool = x.Equal(y)
	}
}

func BenchmarkLookupSymbol(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchUnit, _ = benchSI.Unit("m")
	}
}

func BenchmarkLookupPrefixed(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchUnit, _ = benchSI.Unit("km")
	}
}

func BenchmarkParseUnitSimple(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchUnit = benchSI.MustParseUnit("m")
	}
}

func BenchmarkParseUnitCompound(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchUnit = benchSI.MustParseUnit("kg·m/s^2")
	}
}

func BenchmarkParseQuantity(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchQuantity = benchSI.MustParse("9.81 m/s^2")
	}
}

func BenchmarkQuantityString(b *testing.B) {
	q := New(9.81, benchSI.MustParseUnit("m/s^2"))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchString = q.String()
	}
}

func BenchmarkDimensionMul(b *testing.B) {
	x, y := Length.Div(Time), Mass.Mul(Time)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = x.Mul(y)
	}
}

func BenchmarkSIRegistry(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchUnit = SI().MustUnit("m")
	}
}
