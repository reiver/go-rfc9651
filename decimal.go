package rfc9651

import (
	"math"
	"strconv"
	"strings"

	"codeberg.org/reiver/go-erorr"
	"codeberg.org/reiver/go-field"
)

const (
	maxInteger     = 999_999_999_999_999
	maxThousandths = 999_999_999_999_999 // 999,999,999,999.999
)

// Decimal is a Structured Field decimal: at most 12 integer digits and at most 3 fractional digits.
//
// It is held exactly, as a whole number of thousandths.
type Decimal struct {
	thousandths int64
}

// DecimalFromThousandths returns the decimal 'thousandths' / 1000 (ex: 1250 → 1.25).
func DecimalFromThousandths(thousandths int64) (Decimal, error) {
	if thousandths < -maxThousandths || maxThousandths < thousandths {
		return Decimal{}, erorr.Wrap(ErrDecimalOutOfRange, "decimal out of range", field.Int64("thousandths", thousandths))
	}
	return Decimal{thousandths: thousandths}, nil
}

// DecimalFromFloat64 returns 'value' rounded to 3 fractional digits (ties to even).
//
// The rounding is done on the shortest decimal form of 'value' (as strconv.FormatFloat gives it),
// not on its exact binary value — so 0.5015 rounds as the decimal 0.5015 (to 0.502), not as 0.50149999….
func DecimalFromFloat64(value float64) (Decimal, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Decimal{}, erorr.Wrap(ErrDecimalOutOfRange, "decimal is not a finite number", field.Float64("value", value))
	}

	var s string = strconv.FormatFloat(value, 'f', -1, 64)

	var negative bool = strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")

	integerPart, fractionPart, _ := strings.Cut(s, ".")
	if 13 < len(integerPart) {
		return Decimal{}, erorr.Wrap(ErrDecimalOutOfRange, "decimal out of range", field.Float64("value", value))
	}

	var kept string = (fractionPart + "000")[:3]
	var rest string = ""
	if 3 < len(fractionPart) {
		rest = fractionPart[3:]
	}

	n, err := strconv.ParseInt(integerPart+kept, 10, 64)
	if nil != err {
		return Decimal{}, erorr.Wrap(ErrDecimalOutOfRange, "decimal out of range", field.Float64("value", value))
	}

	// Round half to even.
	if "" != rest {
		switch {
		case '5' < rest[0]:
			n++
		case '5' == rest[0]:
			if "" != strings.TrimRight(rest[1:], "0") || 1 == n%2 {
				n++
			}
		}
	}

	if negative {
		n = -n
	}

	return DecimalFromThousandths(n)
}

// Thousandths returns the decimal as a whole number of thousandths (ex: 1.25 → 1250).
func (receiver Decimal) Thousandths() int64 {
	return receiver.thousandths
}

// Float64 returns the decimal as a float64.
func (receiver Decimal) Float64() float64 {
	return float64(receiver.thousandths) / 1000
}

// String returns the decimal's serialized form (ex: 1.25, 10.0, -0.5).
func (receiver Decimal) String() string {
	var n int64 = receiver.thousandths

	var sign string = ""
	if n < 0 {
		sign = "-"
		n = -n
	}

	var fraction string = strings.TrimRight(strconv.FormatInt(1000+n%1000, 10)[1:], "0")
	if "" == fraction {
		fraction = "0"
	}

	return sign + strconv.FormatInt(n/1000, 10) + "." + fraction
}
