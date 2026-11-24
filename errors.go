package rfc9651

import (
	"codeberg.org/reiver/go-erorr"
)

const (
	ErrBadBareItem         = erorr.Error("rfc9651: bad bare item — must be an int64, int, Decimal, string, Token, []byte, bool, Date, or DisplayString")
	ErrBadKey              = erorr.Error("rfc9651: bad key — must be lcalpha or \"*\", then lcalpha, DIGIT, \"_\", \"-\", \".\", or \"*\"")
	ErrBadMember           = erorr.Error("rfc9651: bad member — must be an Item or an InnerList")
	ErrBadString           = erorr.Error("rfc9651: bad string — must be printable ASCII (%x20-7E)")
	ErrBadToken            = erorr.Error("rfc9651: bad token")
	ErrDecimalOutOfRange   = erorr.Error("rfc9651: decimal out of range — must have at most 12 integer digits")
	ErrIntegerOutOfRange   = erorr.Error("rfc9651: integer out of range — must be in -999,999,999,999,999..999,999,999,999,999")
	ErrParse               = erorr.Error("rfc9651: parse error")
)
