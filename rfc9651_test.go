package rfc9651_test

import (
	"errors"
	"testing"

	"github.com/reiver/go-rfc9651"
)

// A %-escape in a display string must be lower-case hex — even when it would decode to valid UTF-8.
func TestParseItem_displayStringUpperCaseHex(t *testing.T) {
	for _, s := range []string{`%"%4A"`, `%"%4a%2F"`} {
		if _, err := rfc9651.ParseItem(s); !errors.Is(err, rfc9651.ErrParse) {
			t.Errorf("For %q, expected ErrParse but actually got: %v", s, err)
		}
	}

	item, err := rfc9651.ParseItem(`%"%4a"`)
	if nil != err {
		t.Fatalf("Did not expect an error but actually got one: (%T) %s", err, err)
	}
	if rfc9651.DisplayString("J") != item.Value {
		t.Errorf("Expected J but actually got %#v.", item.Value)
	}
}

// An example like a "Signature-Input" field (RFC 9421).
func TestParseDictionary_example(t *testing.T) {
	const s = `sig1=("@method" "@target-uri" "content-digest");created=1618884473;keyid="test-key-ecc-p256", sig2=:aGVsbG8=:`

	dictionary, err := rfc9651.ParseDictionary(s)
	if nil != err {
		t.Fatalf("Did not expect an error but actually got one: (%T) %s", err, err)
	}

	member, found := dictionary.Get("sig1")
	if !found {
		t.Fatal("Expected sig1.")
	}
	innerList, casted := member.(rfc9651.InnerList)
	if !casted || 3 != len(innerList.Items) || "@target-uri" != innerList.Items[1].Value {
		t.Fatalf("Unexpected sig1: %#v", member)
	}
	if created, _ := innerList.Params.Get("created"); int64(1618884473) != created {
		t.Errorf("Unexpected created: %#v", created)
	}
	if keyid, _ := innerList.Params.Get("keyid"); "test-key-ecc-p256" != keyid {
		t.Errorf("Unexpected keyid: %#v", keyid)
	}
	if _, found := innerList.Params.Get("nonce"); found {
		t.Error("Did not expect nonce.")
	}

	member, _ = dictionary.Get("sig2")
	if item, casted := member.(rfc9651.Item); !casted || "hello" != string(item.Value.([]byte)) {
		t.Errorf("Unexpected sig2: %#v", member)
	}
	if _, found := dictionary.Get("sig3"); found {
		t.Error("Did not expect sig3.")
	}

	serialized, err := dictionary.Serialize()
	if nil != err {
		t.Fatalf("Did not expect an error but actually got one: (%T) %s", err, err)
	}
	if s != serialized {
		t.Errorf("Expected %q but actually got %q.", s, serialized)
	}
}

func TestDecimal(t *testing.T) {
	tests := []struct {
		Thousandths int64
		Expected    string
		Float       float64
	}{
		{Thousandths: 0, Expected: "0.0", Float: 0},
		{Thousandths: 1250, Expected: "1.25", Float: 1.25},
		{Thousandths: -500, Expected: "-0.5", Float: -0.5},
		{Thousandths: 7, Expected: "0.007", Float: 0.007},
		{Thousandths: 999_999_999_999_999, Expected: "999999999999.999", Float: 999999999999.999},
	}

	for _, test := range tests {
		decimal, err := rfc9651.DecimalFromThousandths(test.Thousandths)
		if nil != err {
			t.Errorf("For %d, did not expect an error but actually got one: (%T) %s", test.Thousandths, err, err)
			continue
		}
		if test.Expected != decimal.String() {
			t.Errorf("For %d, expected %q but actually got %q.", test.Thousandths, test.Expected, decimal.String())
		}
		if test.Float != decimal.Float64() || test.Thousandths != decimal.Thousandths() {
			t.Errorf("For %d, unexpected Float64 %v or Thousandths %d.", test.Thousandths, decimal.Float64(), decimal.Thousandths())
		}
	}

	if _, err := rfc9651.DecimalFromThousandths(1_000_000_000_000_000); !errors.Is(err, rfc9651.ErrDecimalOutOfRange) {
		t.Errorf("Expected ErrDecimalOutOfRange but actually got: %v", err)
	}
}

func TestSerialize_errors(t *testing.T) {
	tests := []struct {
		Value    rfc9651.Item
		Expected error
	}{
		{Value: rfc9651.Item{Value: nil},                                                                  Expected: rfc9651.ErrBadBareItem},
		{Value: rfc9651.Item{Value: 1.5},                                                                  Expected: rfc9651.ErrBadBareItem},
		{Value: rfc9651.Item{Value: "tab\t"},                                                              Expected: rfc9651.ErrBadString},
		{Value: rfc9651.Item{Value: rfc9651.DisplayString("\xff")},                                        Expected: rfc9651.ErrBadString},
		{Value: rfc9651.Item{Value: rfc9651.Token("1abc")},                                                Expected: rfc9651.ErrBadToken},
		{Value: rfc9651.Item{Value: rfc9651.Date(1_000_000_000_000_000)},                                  Expected: rfc9651.ErrIntegerOutOfRange},
		{Value: rfc9651.Item{Value: true, Params: rfc9651.Params{{Key: "Upper", Value: true}}},            Expected: rfc9651.ErrBadKey},
	}

	for testNumber, test := range tests {
		if _, err := test.Value.Serialize(); !errors.Is(err, test.Expected) {
			t.Errorf("For test #%d, expected %v but actually got: %v", testNumber, test.Expected, err)
		}
	}

	if _, err := (rfc9651.List{nil}).Serialize(); !errors.Is(err, rfc9651.ErrBadMember) {
		t.Errorf("Expected ErrBadMember but actually got: %v", err)
	}
}

// An int is accepted (as an Integer) when serializing.
func TestSerialize_int(t *testing.T) {
	serialized, err := rfc9651.Item{Value: 42, Params: rfc9651.Params{{Key: "a", Value: -7}}}.Serialize()
	if nil != err {
		t.Fatalf("Did not expect an error but actually got one: (%T) %s", err, err)
	}
	if "42;a=-7" != serialized {
		t.Errorf("Expected %q but actually got %q.", "42;a=-7", serialized)
	}
}

func TestParse_errorKind(t *testing.T) {
	for _, s := range []string{"é", "1,", "(1", `"unterminated`} {
		if _, err := rfc9651.ParseList(s); !errors.Is(err, rfc9651.ErrParse) {
			t.Errorf("For %q, expected ErrParse but actually got: %v", s, err)
		}
	}
}

// An empty field is an empty List (or Dictionary) — but not an Item.
func TestParse_empty(t *testing.T) {
	if list, err := rfc9651.ParseList(""); nil != err || 0 != len(list) {
		t.Errorf("Expected an empty list, but actually got %#v (error: %v).", list, err)
	}
	if dictionary, err := rfc9651.ParseDictionary("  "); nil != err || 0 != len(dictionary) {
		t.Errorf("Expected an empty dictionary, but actually got %#v (error: %v).", dictionary, err)
	}
	if _, err := rfc9651.ParseItem(""); !errors.Is(err, rfc9651.ErrParse) {
		t.Errorf("Expected ErrParse but actually got: %v", err)
	}
}
