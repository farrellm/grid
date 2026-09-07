package format

import (
	"math"
	"testing"
)

// Ported verbatim from ngrid's test/formatters.py, class FloatFormatterTest.

var (
	posInf = Float(math.Inf(1))
	negInf = Float(math.Inf(-1))
	nan    = Float(math.NaN())
)

func newFloatDefault(size, precision int) *FloatFormatter {
	return NewFloat(size, precision, ' ', "-", ".", "NaN", "Inf")
}

func TestFloatDefault(t *testing.T) {
	f := newFloatDefault(4, 2)
	checkWidth(t, f, 8)
	checkFmt(t, f, []fmtCase{
		{"    0.00", Float(0.0)},
		{"    1.00", Float(1.0)},
		{"   -1.00", Float(-1.0)},
		{"   12.34", Float(12.344)},
		{"   12.34", Float(12.3449999999)},
		{"   12.35", Float(12.3450000001)},
		{"  -12.34", Float(-12.3449999999)},
		{"  -12.35", Float(-12.3450000001)},
		{" 9999.99", Float(9999.99)},
		{"-9999.99", Float(-9999.99)},
		{"########", Float(9999.999)},
		{"########", Float(-9999.999)},
		{"     NaN", nan},
		{"     Inf", posInf},
		{"    -Inf", negInf},
	})
}

func TestFloatPrecisionNone(t *testing.T) {
	f := newFloatDefault(4, NoPrecision)
	checkWidth(t, f, 5)
	checkFmt(t, f, []fmtCase{
		{"    0", Float(0.0)},
		{"    1", Float(1.0)},
		{"   -1", Float(-1.0)},
		{"   12", Float(12.4)},
		{"   12", Float(12.49999999)},
		{"   13", Float(12.50000001)},
		{"  -12", Float(-12.49999999)},
		{"  -13", Float(-12.50000001)},
		{" 9999", Float(9998.99)},
		{"-9999", Float(-9998.99)},
		{"#####", Float(9999.999)},
		{"#####", Float(-9999.999)},
		{"  NaN", nan},
		{"  Inf", posInf},
		{" -Inf", negInf},
	})
}

func TestFloatPrecision0(t *testing.T) {
	f := newFloatDefault(4, 0)
	checkWidth(t, f, 6)
	checkFmt(t, f, []fmtCase{
		{"    0.", Float(0.0)},
		{"    1.", Float(1.0)},
		{"   -1.", Float(-1.0)},
		{"   12.", Float(12.4)},
		{"   12.", Float(12.49999999)},
		{"   13.", Float(12.50000001)},
		{"  -12.", Float(-12.49999999)},
		{"  -13.", Float(-12.50000001)},
		{" 9999.", Float(9998.99)},
		{"-9999.", Float(-9998.99)},
		{"######", Float(9999.999)},
		{"######", Float(-9999.999)},
		{"   NaN", nan},
		{"   Inf", posInf},
		{"  -Inf", negInf},
	})
}

func TestFloatPad0(t *testing.T) {
	f := NewFloat(4, 2, '0', "-", ".", "NaN", "Inf")
	checkWidth(t, f, 8)
	checkFmt(t, f, []fmtCase{
		{" 0000.00", Float(0.0)},
		{" 0001.00", Float(1.0)},
		{"-0001.00", Float(-1.0)},
		{" 0012.34", Float(12.344)},
		{" 0012.34", Float(12.3449999999)},
		{" 0012.35", Float(12.3450000001)},
		{"-0012.34", Float(-12.3449999999)},
		{"-0012.35", Float(-12.3450000001)},
		{" 9999.99", Float(9999.99)},
		{"-9999.99", Float(-9999.99)},
		{"########", Float(9999.999)},
		{"########", Float(-9999.999)},
		{"     NaN", nan},
		{"     Inf", posInf},
		{"    -Inf", negInf},
	})
}

func TestFloatSignPlus(t *testing.T) {
	f := NewFloat(4, 2, ' ', "+", ".", "NaN", "Inf")
	checkWidth(t, f, 8)
	checkFmt(t, f, []fmtCase{
		{"   +0.00", Float(0.0)},
		{"   +1.00", Float(1.0)},
		{"   -1.00", Float(-1.0)},
		{"  +12.34", Float(12.344)},
		{"  +12.34", Float(12.3449999999)},
		{"  +12.35", Float(12.3450000001)},
		{"  -12.34", Float(-12.3449999999)},
		{"  -12.35", Float(-12.3450000001)},
		{"+9999.99", Float(9999.99)},
		{"-9999.99", Float(-9999.99)},
		{"########", Float(9999.999)},
		{"########", Float(-9999.999)},
		{"     NaN", nan},
		{"    +Inf", posInf},
		{"    -Inf", negInf},
	})
}

func TestFloatSignNone(t *testing.T) {
	f := NewFloat(4, 2, ' ', "", ".", "NaN", "Inf")
	checkWidth(t, f, 7)
	checkFmt(t, f, []fmtCase{
		{"   0.00", Float(0.0)},
		{"   1.00", Float(1.0)},
		{"#######", Float(-1.0)},
		{"  12.34", Float(12.344)},
		{"  12.34", Float(12.3449999999)},
		{"  12.35", Float(12.3450000001)},
		{"#######", Float(-12.3449999999)},
		{"#######", Float(-12.3450000001)},
		{"9999.99", Float(9999.99)},
		{"#######", Float(-9999.99)},
		{"#######", Float(9999.999)},
		{"#######", Float(-9999.999)},
		{"    NaN", nan},
		{"    Inf", posInf},
		{"#######", negInf},
	})
}

func TestFloatPoint(t *testing.T) {
	f := NewFloat(4, 2, ' ', "-", ",", "NaN", "Inf")
	checkWidth(t, f, 8)
	checkFmt(t, f, []fmtCase{
		{"    0,00", Float(0.0)},
		{"   -1,00", Float(-1.0)},
		{"-9999,99", Float(-9999.99)},
		{"########", Float(9999.999)},
	})
}

func TestFloatNaNStr(t *testing.T) {
	f := NewFloat(4, 2, ' ', "-", ".", "INVALID", "Inf")
	checkWidth(t, f, 8)
	checkFmt(t, f, []fmtCase{
		{"    0.00", Float(0.0)},
		{"-9999.99", Float(-9999.99)},
		{" INVALID", nan},
	})

	f = NewFloat(4, 0, ' ', "-", ".", "INVALID", "Inf")
	checkFmt(t, f, []fmtCase{{" INVAL", nan}})
}

func TestFloatInfStr(t *testing.T) {
	f := NewFloat(4, 3, ' ', "-", ".", "NaN", "INFINITE")
	checkWidth(t, f, 9)
	checkFmt(t, f, []fmtCase{
		{"    0.000", Float(0.0)},
		{"-9999.990", Float(-9999.99)},
		{" INFINITE", posInf},
		{"-INFINITE", negInf},
	})

	f = NewFloat(4, 0, ' ', "-", ".", "NaN", "INFINITE")
	checkFmt(t, f, []fmtCase{
		{" INFIN", posInf},
		{"-INFIN", negInf},
	})
}

func checkWidth(t *testing.T, f Formatter, want int) {
	t.Helper()
	if got := f.Width(); got != want {
		t.Errorf("Width() = %d, want %d", got, want)
	}
}
