package rfc9651

import (
	"encoding/base64"
	"strconv"
	"strings"
	"unicode/utf8"

	"codeberg.org/reiver/go-erorr"
	"codeberg.org/reiver/go-field"
)

// ParseItem parses a field value whose type is Item.
//
// For a field with more than one line, join the lines with ", " first (ex: strings.Join(values, ", ")).
// On any error the whole field must be ignored.
func ParseItem(s string) (Item, error) {
	var item Item
	err := parseField(s, func(p *parser) error {
		var err error
		item, err = p.parseItem()
		return err
	})
	if nil != err {
		return Item{}, err
	}
	return item, nil
}

// ParseList parses a field value whose type is List.
//
// For a field with more than one line, join the lines with ", " first (ex: strings.Join(values, ", ")).
// On any error the whole field must be ignored.
func ParseList(s string) (List, error) {
	var list List
	err := parseField(s, func(p *parser) error {
		var err error
		list, err = p.parseList()
		return err
	})
	if nil != err {
		return nil, err
	}
	return list, nil
}

// ParseDictionary parses a field value whose type is Dictionary.
//
// For a field with more than one line, join the lines with ", " first (ex: strings.Join(values, ", ")).
// On any error the whole field must be ignored.
func ParseDictionary(s string) (Dictionary, error) {
	var dictionary Dictionary
	err := parseField(s, func(p *parser) error {
		var err error
		dictionary, err = p.parseDictionary()
		return err
	})
	if nil != err {
		return nil, err
	}
	return dictionary, nil
}

func parseField(s string, parse func(*parser) error) error {
	for index := 0; index < len(s); index++ {
		if 0x80 <= s[index] {
			return erorr.Wrap(ErrParse, "non-ASCII character", field.Int("offset", index))
		}
	}

	var p *parser = &parser{input: s}

	p.discardSP()
	if err := parse(p); nil != err {
		return err
	}
	p.discardSP()

	if !p.empty() {
		return p.fail("unexpected character after the field value")
	}
	return nil
}

type parser struct {
	input  string
	offset int
}

func (receiver *parser) empty() bool {
	return len(receiver.input) <= receiver.offset
}

// peek returns the next character, or 0 at the end. (The input never holds 0x00 where it matters: every caller rejects it.)
func (receiver *parser) peek() byte {
	if receiver.empty() {
		return 0
	}
	return receiver.input[receiver.offset]
}

func (receiver *parser) fail(msg string) error {
	return erorr.Wrap(ErrParse, msg, field.Int("offset", receiver.offset))
}

func (receiver *parser) discardSP() {
	for !receiver.empty() && ' ' == receiver.peek() {
		receiver.offset++
	}
}

func (receiver *parser) discardOWS() {
	for !receiver.empty() && (' ' == receiver.peek() || '\t' == receiver.peek()) {
		receiver.offset++
	}
}

// RFC 9651 §4.2.1
func (receiver *parser) parseList() (List, error) {
	var list List = List{}

	for !receiver.empty() {
		member, err := receiver.parseItemOrInnerList()
		if nil != err {
			return nil, err
		}
		list = append(list, member)

		receiver.discardOWS()
		if receiver.empty() {
			return list, nil
		}
		if ',' != receiver.peek() {
			return nil, receiver.fail("expected \",\" between list members")
		}
		receiver.offset++
		receiver.discardOWS()
		if receiver.empty() {
			return nil, receiver.fail("trailing \",\" in list")
		}
	}

	return list, nil
}

func (receiver *parser) parseItemOrInnerList() (Member, error) {
	if '(' == receiver.peek() {
		return receiver.parseInnerList()
	}
	return receiver.parseItem()
}

// RFC 9651 §4.2.1.2
func (receiver *parser) parseInnerList() (InnerList, error) {
	if '(' != receiver.peek() {
		return InnerList{}, receiver.fail("expected \"(\"")
	}
	receiver.offset++

	var items []Item = []Item{}

	for !receiver.empty() {
		receiver.discardSP()

		if ')' == receiver.peek() {
			receiver.offset++
			params, err := receiver.parseParams()
			if nil != err {
				return InnerList{}, err
			}
			return InnerList{Items: items, Params: params}, nil
		}

		item, err := receiver.parseItem()
		if nil != err {
			return InnerList{}, err
		}
		items = append(items, item)

		if c := receiver.peek(); ' ' != c && ')' != c {
			return InnerList{}, receiver.fail("expected \" \" or \")\" in inner list")
		}
	}

	return InnerList{}, receiver.fail("inner list not closed")
}

// RFC 9651 §4.2.2
func (receiver *parser) parseDictionary() (Dictionary, error) {
	var dictionary Dictionary = Dictionary{}

	for !receiver.empty() {
		key, err := receiver.parseKey()
		if nil != err {
			return nil, err
		}

		var member Member
		if '=' == receiver.peek() {
			receiver.offset++
			member, err = receiver.parseItemOrInnerList()
			if nil != err {
				return nil, err
			}
		} else {
			params, err := receiver.parseParams()
			if nil != err {
				return nil, err
			}
			member = Item{Value: true, Params: params}
		}
		dictionary = dictionary.set(key, member)

		receiver.discardOWS()
		if receiver.empty() {
			return dictionary, nil
		}
		if ',' != receiver.peek() {
			return nil, receiver.fail("expected \",\" between dictionary members")
		}
		receiver.offset++
		receiver.discardOWS()
		if receiver.empty() {
			return nil, receiver.fail("trailing \",\" in dictionary")
		}
	}

	return dictionary, nil
}

// RFC 9651 §4.2.3
func (receiver *parser) parseItem() (Item, error) {
	value, err := receiver.parseBareItem()
	if nil != err {
		return Item{}, err
	}

	params, err := receiver.parseParams()
	if nil != err {
		return Item{}, err
	}

	return Item{Value: value, Params: params}, nil
}

// RFC 9651 §4.2.3.1
func (receiver *parser) parseBareItem() (any, error) {
	var c byte = receiver.peek()

	switch {
	case receiver.empty():
		return nil, receiver.fail("expected a bare item")
	case '-' == c || isDigit(c):
		return receiver.parseNumber()
	case '"' == c:
		return receiver.parseString()
	case '*' == c || isAlpha(c):
		return receiver.parseToken()
	case ':' == c:
		return receiver.parseByteSequence()
	case '?' == c:
		return receiver.parseBoolean()
	case '@' == c:
		return receiver.parseDate()
	case '%' == c:
		return receiver.parseDisplayString()
	default:
		return nil, receiver.fail("unexpected character at the start of a bare item")
	}
}

// RFC 9651 §4.2.3.2
func (receiver *parser) parseParams() (Params, error) {
	var params Params = Params{}

	for !receiver.empty() {
		if ';' != receiver.peek() {
			break
		}
		receiver.offset++
		receiver.discardSP()

		key, err := receiver.parseKey()
		if nil != err {
			return nil, err
		}

		var value any = true
		if '=' == receiver.peek() {
			receiver.offset++
			value, err = receiver.parseBareItem()
			if nil != err {
				return nil, err
			}
		}

		params = params.set(key, value)
	}

	return params, nil
}

// RFC 9651 §4.2.3.3
func (receiver *parser) parseKey() (string, error) {
	if c := receiver.peek(); !isLCAlpha(c) && '*' != c {
		return "", receiver.fail("a key must start with lcalpha or \"*\"")
	}

	var start int = receiver.offset
	for !receiver.empty() && isKeyChar(receiver.peek()) {
		receiver.offset++
	}

	return receiver.input[start:receiver.offset], nil
}

// RFC 9651 §4.2.4
func (receiver *parser) parseNumber() (any, error) {
	var isDecimal bool = false

	var negative bool = false
	if '-' == receiver.peek() {
		receiver.offset++
		negative = true
	}

	if !isDigit(receiver.peek()) {
		return nil, receiver.fail("expected a digit")
	}

	var start int = receiver.offset
	for !receiver.empty() {
		var c byte = receiver.peek()
		if isDigit(c) {
			receiver.offset++
		} else if !isDecimal && '.' == c {
			if 12 < receiver.offset-start {
				return nil, receiver.fail("too many integer digits in a decimal")
			}
			receiver.offset++
			isDecimal = true
		} else {
			break
		}

		if !isDecimal && 15 < receiver.offset-start {
			return nil, receiver.fail("too many digits in an integer")
		}
		if isDecimal && 16 < receiver.offset-start {
			return nil, receiver.fail("too many characters in a decimal")
		}
	}

	var number string = receiver.input[start:receiver.offset]

	if !isDecimal {
		n, err := strconv.ParseInt(number, 10, 64)
		if nil != err {
			return nil, receiver.fail("bad integer")
		}
		if negative {
			n = -n
		}
		return n, nil
	}

	integerPart, fractionPart, _ := strings.Cut(number, ".")
	if "" == fractionPart {
		return nil, receiver.fail("a decimal must have a digit after the \".\"")
	}
	if 3 < len(fractionPart) {
		return nil, receiver.fail("too many fractional digits in a decimal")
	}

	n, err := strconv.ParseInt(integerPart+(fractionPart+"00")[:3], 10, 64)
	if nil != err {
		return nil, receiver.fail("bad decimal")
	}
	if negative {
		n = -n
	}
	return Decimal{thousandths: n}, nil
}

// RFC 9651 §4.2.5
func (receiver *parser) parseString() (string, error) {
	if '"' != receiver.peek() {
		return "", receiver.fail("expected \"\\\"\"")
	}
	receiver.offset++

	var builder strings.Builder
	for !receiver.empty() {
		var c byte = receiver.peek()
		receiver.offset++

		switch {
		case '\\' == c:
			if receiver.empty() {
				return "", receiver.fail("string ends after \"\\\"")
			}
			var next byte = receiver.peek()
			if '"' != next && '\\' != next {
				return "", receiver.fail("bad escape in string")
			}
			receiver.offset++
			builder.WriteByte(next)
		case '"' == c:
			return builder.String(), nil
		case c < 0x20 || 0x7e < c:
			return "", receiver.fail("bad character in string")
		default:
			builder.WriteByte(c)
		}
	}

	return "", receiver.fail("string not closed")
}

// RFC 9651 §4.2.6
func (receiver *parser) parseToken() (Token, error) {
	if c := receiver.peek(); !isAlpha(c) && '*' != c {
		return "", receiver.fail("a token must start with ALPHA or \"*\"")
	}

	var start int = receiver.offset
	for !receiver.empty() && isTokenChar(receiver.peek()) {
		receiver.offset++
	}

	return Token(receiver.input[start:receiver.offset]), nil
}

// RFC 9651 §4.2.7
//
// As RFC 9651 recommends, missing (or partial) "=" padding and non-zero pad bits are accepted.
func (receiver *parser) parseByteSequence() ([]byte, error) {
	if ':' != receiver.peek() {
		return nil, receiver.fail("expected \":\"")
	}
	receiver.offset++

	var end int = strings.IndexByte(receiver.input[receiver.offset:], ':')
	if end < 0 {
		return nil, receiver.fail("byte sequence not closed")
	}

	var content string = receiver.input[receiver.offset : receiver.offset+end]
	for index := 0; index < len(content); index++ {
		if c := content[index]; !isAlpha(c) && !isDigit(c) && '+' != c && '/' != c && '=' != c {
			return nil, receiver.fail("bad character in byte sequence")
		}
	}

	decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(content, "="))
	if nil != err {
		return nil, receiver.fail("bad base64 in byte sequence")
	}

	receiver.offset += end + 1
	return decoded, nil
}

// RFC 9651 §4.2.8
func (receiver *parser) parseBoolean() (bool, error) {
	if '?' != receiver.peek() {
		return false, receiver.fail("expected \"?\"")
	}
	receiver.offset++

	switch receiver.peek() {
	case '1':
		receiver.offset++
		return true, nil
	case '0':
		receiver.offset++
		return false, nil
	default:
		return false, receiver.fail("a boolean must be ?1 or ?0")
	}
}

// RFC 9651 §4.2.9
func (receiver *parser) parseDate() (Date, error) {
	if '@' != receiver.peek() {
		return 0, receiver.fail("expected \"@\"")
	}
	receiver.offset++

	number, err := receiver.parseNumber()
	if nil != err {
		return 0, err
	}

	n, casted := number.(int64)
	if !casted {
		return 0, receiver.fail("a date must be an integer")
	}
	return Date(n), nil
}

// RFC 9651 §4.2.10
func (receiver *parser) parseDisplayString() (DisplayString, error) {
	if '%' != receiver.peek() {
		return "", receiver.fail("expected \"%\"")
	}
	receiver.offset++
	if '"' != receiver.peek() {
		return "", receiver.fail("expected \"\\\"\" after \"%\"")
	}
	receiver.offset++

	var bytes []byte
	for !receiver.empty() {
		var c byte = receiver.peek()
		receiver.offset++

		switch {
		case c < 0x20 || 0x7e < c:
			return "", receiver.fail("bad character in display string")
		case '%' == c:
			if len(receiver.input) < receiver.offset+2 {
				return "", receiver.fail("display string ends inside a %-escape")
			}
			high, highOK := lowerHexValue(receiver.input[receiver.offset])
			low, lowOK := lowerHexValue(receiver.input[receiver.offset+1])
			if !highOK || !lowOK {
				return "", receiver.fail("bad %-escape in display string — must be 2 lower-case hex digits")
			}
			receiver.offset += 2
			bytes = append(bytes, high<<4|low)
		case '"' == c:
			if !utf8.Valid(bytes) {
				return "", receiver.fail("display string is not valid UTF-8")
			}
			return DisplayString(bytes), nil
		default:
			bytes = append(bytes, c)
		}
	}

	return "", receiver.fail("display string not closed")
}

func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

func isLCAlpha(c byte) bool {
	return 'a' <= c && c <= 'z'
}

func isAlpha(c byte) bool {
	return isLCAlpha(c) || ('A' <= c && c <= 'Z')
}

func isKeyChar(c byte) bool {
	return isLCAlpha(c) || isDigit(c) || '_' == c || '-' == c || '.' == c || '*' == c
}

// isTokenChar returns whether 'c' is a tchar (RFC 9110), ":", or "/".
func isTokenChar(c byte) bool {
	if isAlpha(c) || isDigit(c) {
		return true
	}
	return strings.IndexByte("!#$%&'*+-.^_`|~:/", c) >= 0
}

func lowerHexValue(c byte) (byte, bool) {
	switch {
	case isDigit(c):
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	default:
		return 0, false
	}
}
