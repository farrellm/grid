package format

import "testing"

// Ported verbatim from ngrid's test/formatters.py, class EFloatFormatterTest.

func newEFloatDefault(size, precision int) *EFloatFormatter {
	return NewEFloat(size, precision, "-", ".", "E", "NaN", "Inf")
}

func TestEFloatDefault(t *testing.T) {
	f := newEFloatDefault(2, 2)
	checkWidth(t, f, 9)
	checkFmt(t, f, []fmtCase{
		{" 0.00E+00", Float(0.0)},
		{" 1.00E+00", Float(1.0)},
		{"-1.00E+00", Float(-1.0)},
		{" 9.90E-01", Float(0.99)},
		{" 9.95E-01", Float(0.994999)},
		{" 9.95E-01", Float(0.995001)},
		{" 1.00E+00", Float(0.999501)},
		{"-9.90E-01", Float(-0.99)},
		{"-9.95E-01", Float(-0.994999)},
		{"-9.95E-01", Float(-0.995001)},
		{"-1.00E+00", Float(-0.999501)},
		{" 1.00E+03", Float(1000.0)},
		{"-1.00E+03", Float(-1000.0)},
		{" 1.23E+11", Float(123456789012)},
		{"-1.23E+11", Float(-123456789012)},
		{" 1.24E+11", Float(123556789012)},
		{"-1.24E+11", Float(-123556789012)},
		{" 1.23E-13", Float(0.0000000000001234)},
		{"-1.23E-13", Float(-0.0000000000001234)},
		{" 1.24E-13", Float(0.000000000000123501)},
		{"-1.24E-13", Float(-0.000000000000123501)},
		{" 9.99E+99", Float(9.99e+99)},
		{"-9.99E+99", Float(-9.99e+99)},
		{"#########", Float(9.996e+99)},
		{"#########", Float(-9.996e+99)},
		{"#########", Float(1e+100)},
		{"#########", Float(-1e+100)},
		{" 1.00E-99", Float(1.00e-99)},
		{" 1.00E-99", Float(0.999501e-99)},
		{"-1.00E-99", Float(-1.00e-99)},
		{"-1.00E-99", Float(-0.999501e-99)},
		{"#########", Float(0.994e-99)},
		{"#########", Float(-0.994e-99)},
		{"#########", Float(1e-100)},
		{"#########", Float(-1e-100)},
		{"      NaN", nan},
		{"      Inf", posInf},
		{"     -Inf", negInf},
	})
}

func TestEFloatSize1(t *testing.T) {
	f := newEFloatDefault(1, 2)
	checkWidth(t, f, 8)
	checkFmt(t, f, []fmtCase{
		{" 0.00E+0", Float(0.0)},
		{" 9.95E-1", Float(0.995001)},
		{" 1.00E+0", Float(0.999501)},
		{"-9.95E-1", Float(-0.995001)},
		{"-1.00E+0", Float(-0.999501)},
		{" 9.99E+9", Float(9994999999.99)},
		{"-9.99E+9", Float(-9994999999.99)},
		{"########", Float(9995000001.0)},
		{"########", Float(-9995000001.0)},
		{" 1.00E-9", Float(0.0000000009995001)},
		{"-1.00E-9", Float(-0.0000000009995001)},
		{"########", Float(0.0000000009994999)},
		{"########", Float(-0.0000000009994999)},
		{"     NaN", nan},
		{"     Inf", posInf},
		{"    -Inf", negInf},
	})
}

func TestEFloatSize4(t *testing.T) {
	f := newEFloatDefault(4, 2)
	checkWidth(t, f, 11)
	checkFmt(t, f, []fmtCase{
		{" 0.00E+0000", Float(0.0)},
		{" 9.95E-0001", Float(0.995001)},
		{" 1.00E+0000", Float(0.999501)},
		{" 1.23E+0110", Float(1.23e+110)},
		{"-1.23E-0110", Float(-1.23e-110)},
	})
}

func TestEFloatPrecisionNone(t *testing.T) {
	f := newEFloatDefault(2, NoPrecision)
	checkWidth(t, f, 6)
	checkFmt(t, f, []fmtCase{
		{" 0E+00", Float(0.0)},
		{" 1E+00", Float(1.0)},
		{"-1E+00", Float(-1.0)},
		{" 5E-01", Float(0.500001)},
		{"-5E-01", Float(-0.500001)},
		{"-5E-01", Float(-0.4999)},
		{"-1E+00", Float(-0.995001)},
		{"-1E+00", Float(-0.999501)},
		{" 1E+03", Float(1000.0)},
		{"-1E+03", Float(-1000.0)},
		{" 1E+11", Float(123456789012)},
		{"-1E+11", Float(-123456789012)},
		{" 2E+11", Float(150000000001)},
		{"-2E+11", Float(-150000000001)},
		{" 1E-13", Float(0.0000000000001234)},
		{"-1E-13", Float(-0.0000000000001234)},
		{" 1E+99", Float(1e+99)},
		{"-1E+99", Float(-1e+99)},
		{"######", Float(9.50001e+99)},
		{"######", Float(-9.50001e+99)},
		{"   NaN", nan},
		{"   Inf", posInf},
		{"  -Inf", negInf},
	})
}

func TestEFloatPrecision0(t *testing.T) {
	f := newEFloatDefault(2, 0)
	checkWidth(t, f, 7)
	checkFmt(t, f, []fmtCase{
		{" 0.E+00", Float(0.0)},
		{" 1.E+00", Float(1.0)},
		{"-1.E+00", Float(-1.0)},
		{" 5.E-01", Float(0.500001)},
		{"-5.E-01", Float(-0.500001)},
		{"-5.E-01", Float(-0.4999)},
		{"-1.E+00", Float(-0.995001)},
		{"-1.E+00", Float(-0.999501)},
		{" 1.E+03", Float(1000.0)},
		{"-1.E+03", Float(-1000.0)},
		{" 1.E+11", Float(123456789012)},
		{"-1.E+11", Float(-123456789012)},
		{" 2.E+11", Float(150000000001)},
		{"-2.E+11", Float(-150000000001)},
		{" 1.E-13", Float(0.0000000000001234)},
		{"-1.E-13", Float(-0.0000000000001234)},
		{" 1.E+99", Float(1e+99)},
		{"-1.E+99", Float(-1e+99)},
		{"#######", Float(9.50001e+99)},
		{"#######", Float(-9.50001e+99)},
		{"    NaN", nan},
		{"    Inf", posInf},
		{"   -Inf", negInf},
	})
}

func TestEFloatSignPlus(t *testing.T) {
	f := NewEFloat(2, 2, "+", ".", "E", "NaN", "Inf")
	checkWidth(t, f, 9)
	checkFmt(t, f, []fmtCase{
		{"+0.00E+00", Float(0.0)},
		{"+1.00E+00", Float(1.0)},
		{"-1.00E+00", Float(-1.0)},
		{"+9.95E-01", Float(0.995001)},
		{"+1.00E+00", Float(0.999501)},
		{"-9.90E-01", Float(-0.99)},
		{"+1.00E+03", Float(1000.0)},
		{"-1.00E+03", Float(-1000.0)},
		{"+1.23E+11", Float(123456789012)},
		{"-1.23E+11", Float(-123456789012)},
		{"+1.24E+11", Float(123556789012)},
		{"-1.24E+11", Float(-123556789012)},
		{"+9.99E+99", Float(9.99e+99)},
		{"-9.99E+99", Float(-9.99e+99)},
		{"+1.00E-99", Float(1.00e-99)},
		{"-1.00E-99", Float(-1.00e-99)},
		{"      NaN", nan},
		{"     +Inf", posInf},
		{"     -Inf", negInf},
	})
}

func TestEFloatSignNone(t *testing.T) {
	f := NewEFloat(2, 2, "", ".", "E", "NaN", "Inf")
	checkWidth(t, f, 8)
	checkFmt(t, f, []fmtCase{
		{"0.00E+00", Float(0.0)},
		{"1.00E+00", Float(1.0)},
		{"########", Float(-1.0)},
		{"9.95E-01", Float(0.995001)},
		{"1.00E+00", Float(0.999501)},
		{"########", Float(-0.99)},
		{"1.00E+03", Float(1000.0)},
		{"########", Float(-1000.0)},
		{"1.23E+11", Float(123456789012)},
		{"########", Float(-123456789012)},
		{"1.24E+11", Float(123556789012)},
		{"########", Float(-123556789012)},
		{"9.99E+99", Float(9.99e+99)},
		{"########", Float(-9.99e+99)},
		{"1.00E-99", Float(1.00e-99)},
		{"########", Float(-1.00e-99)},
		{"     NaN", nan},
		{"     Inf", posInf},
		{"########", negInf},
	})
}

func TestEFloatPoint(t *testing.T) {
	f := NewEFloat(2, 2, "-", ",", "E", "NaN", "Inf")
	checkWidth(t, f, 9)
	checkFmt(t, f, []fmtCase{
		{" 0,00E+00", Float(0.0)},
		{" 1,00E+00", Float(1.0)},
		{"-1,00E+00", Float(-1.0)},
		{" 1,00E+03", Float(1000.0)},
		{"-1,00E+03", Float(-1000.0)},
		{" 9,99E+99", Float(9.99e+99)},
		{"-9,99E+99", Float(-9.99e+99)},
	})
}

func TestEFloatExp(t *testing.T) {
	f := NewEFloat(2, 2, "-", ".", " e", "NaN", "Inf")
	checkWidth(t, f, 10)
	checkFmt(t, f, []fmtCase{
		{" 0.00 e+00", Float(0.0)},
		{" 1.00 e+00", Float(1.0)},
		{"-1.00 e+00", Float(-1.0)},
		{" 1.00 e+03", Float(1000.0)},
		{"-1.00 e+03", Float(-1000.0)},
		{" 9.99 e+99", Float(9.99e+99)},
		{"-9.99 e+99", Float(-9.99e+99)},
	})
}

func TestEFloatNaNStr(t *testing.T) {
	f := NewEFloat(2, 2, "-", ".", "E", "INVALID", "Inf")
	checkWidth(t, f, 9)
	checkFmt(t, f, []fmtCase{
		{" 0.00E+00", Float(0.0)},
		{"-1.00E+04", Float(-9999.99)},
		{"  INVALID", nan},
		{"      Inf", posInf},
		{"     -Inf", negInf},
	})

	f = NewEFloat(2, NoPrecision, "-", ".", "E", "INVALID", "Inf")
	checkFmt(t, f, []fmtCase{{" INVAL", nan}})
}

func TestEFloatInfStr(t *testing.T) {
	f := NewEFloat(2, 3, "-", ".", "E", "NaN", "INFINITY")
	checkWidth(t, f, 10)
	checkFmt(t, f, []fmtCase{
		{" 0.000E+00", Float(0.0)},
		{"-1.000E+04", Float(-9999.99)},
		{"       NaN", nan},
		{"  INFINITY", posInf},
		{" -INFINITY", negInf},
	})

	f = NewEFloat(2, NoPrecision, "-", ".", "E", "NaN", "INFINITY")
	checkFmt(t, f, []fmtCase{
		{" INFIN", posInf},
		{"-INFIN", negInf},
	})
}
