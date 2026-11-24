package rfc9651_test

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/reiver/go-rfc9651"
)

// The official Structured Field tests (https://github.com/httpwg/structured-field-tests).

type sfTest struct {
	Name       string          `json:"name"`
	Raw        []string        `json:"raw"`
	HeaderType string          `json:"header_type"`
	Expected   json.RawMessage `json:"expected"`
	MustFail   bool            `json:"must_fail"`
	CanFail    bool            `json:"can_fail"`
	Canonical  []string        `json:"canonical"`
}

const sfTestsDir = "testdata/structured-field-tests"

func loadSFTests(t *testing.T, pattern string) map[string][]sfTest {
	t.Helper()

	filenames, err := filepath.Glob(filepath.Join(sfTestsDir, pattern))
	if nil != err {
		t.Fatalf("Did not expect an error but actually got one: (%T) %s", err, err)
	}
	if 0 == len(filenames) {
		t.Fatalf("Expected some test files matching %q.", pattern)
	}

	var result map[string][]sfTest = map[string][]sfTest{}
	for _, filename := range filenames {
		data, err := os.ReadFile(filename)
		if nil != err {
			t.Fatalf("Did not expect an error but actually got one: (%T) %s", err, err)
		}

		var tests []sfTest
		if err := json.Unmarshal(data, &tests); nil != err {
			t.Fatalf("For %q, did not expect an error but actually got one: (%T) %s", filename, err, err)
		}
		result[filepath.Base(filename)] = tests
	}
	return result
}

func decodeJSON(data []byte) (any, error) {
	var decoder *json.Decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var value any
	err := decoder.Decode(&value)
	return value, err
}

// fromJSON* turn the tests' JSON form into this package's types.

func fromJSONBareItem(value any) (any, error) {
	switch casted := value.(type) {
	case json.Number:
		if !strings.ContainsAny(string(casted), ".eE") {
			return casted.Int64()
		}
		f, err := casted.Float64()
		if nil != err {
			return nil, err
		}
		return rfc9651.DecimalFromFloat64(f)
	case string:
		return casted, nil
	case bool:
		return casted, nil
	case map[string]any:
		switch casted["__type"] {
		case "token":
			return rfc9651.Token(casted["value"].(string)), nil
		case "binary":
			return base32.StdEncoding.DecodeString(casted["value"].(string))
		case "displaystring":
			return rfc9651.DisplayString(casted["value"].(string)), nil
		}
	}
	return nil, fmt.Errorf("unknown bare item: %#v", value)
}

func fromJSONParams(value any) (rfc9651.Params, error) {
	var params rfc9651.Params = rfc9651.Params{}
	for _, entry := range value.([]any) {
		pair := entry.([]any)
		bare, err := fromJSONBareItemOrDate(pair[1])
		if nil != err {
			return nil, err
		}
		params = append(params, rfc9651.Param{Key: pair[0].(string), Value: bare})
	}
	return params, nil
}

// fromJSONBareItemOrDate is fromJSONBareItem, with dates turned into rfc9651.Date.
func fromJSONBareItemOrDate(value any) (any, error) {
	if m, casted := value.(map[string]any); casted && "date" == m["__type"] {
		n, err := m["value"].(json.Number).Int64()
		if nil != err {
			return nil, err
		}
		return rfc9651.Date(n), nil
	}
	return fromJSONBareItem(value)
}

func fromJSONItem(value any) (rfc9651.Item, error) {
	pair := value.([]any)
	bare, err := fromJSONBareItemOrDate(pair[0])
	if nil != err {
		return rfc9651.Item{}, err
	}
	params, err := fromJSONParams(pair[1])
	if nil != err {
		return rfc9651.Item{}, err
	}
	return rfc9651.Item{Value: bare, Params: params}, nil
}

func fromJSONMember(value any) (rfc9651.Member, error) {
	pair := value.([]any)

	// An inner list's first element is an array (of items); an item's is a bare item.
	if items, casted := pair[0].([]any); casted {
		var innerList rfc9651.InnerList = rfc9651.InnerList{Items: []rfc9651.Item{}}
		for _, itemValue := range items {
			item, err := fromJSONItem(itemValue)
			if nil != err {
				return nil, err
			}
			innerList.Items = append(innerList.Items, item)
		}
		params, err := fromJSONParams(pair[1])
		if nil != err {
			return nil, err
		}
		innerList.Params = params
		return innerList, nil
	}

	return fromJSONItem(value)
}

// fromJSON turns the tests' JSON form of a whole field into this package's type.
func fromJSON(headerType string, data []byte) (any, error) {
	value, err := decodeJSON(data)
	if nil != err {
		return nil, err
	}

	switch headerType {
	case "item":
		return fromJSONItem(value)
	case "list":
		var list rfc9651.List = rfc9651.List{}
		for _, memberValue := range value.([]any) {
			member, err := fromJSONMember(memberValue)
			if nil != err {
				return nil, err
			}
			list = append(list, member)
		}
		return list, nil
	case "dictionary":
		var dictionary rfc9651.Dictionary = rfc9651.Dictionary{}
		for _, entry := range value.([]any) {
			pair := entry.([]any)
			member, err := fromJSONMember(pair[1])
			if nil != err {
				return nil, err
			}
			dictionary = append(dictionary, rfc9651.DictionaryMember{Key: pair[0].(string), Value: member})
		}
		return dictionary, nil
	default:
		return nil, fmt.Errorf("unknown header type %q", headerType)
	}
}

func parse(headerType string, raw []string) (any, error) {
	var s string = strings.Join(raw, ", ")

	switch headerType {
	case "item":
		return rfc9651.ParseItem(s)
	case "list":
		return rfc9651.ParseList(s)
	case "dictionary":
		return rfc9651.ParseDictionary(s)
	default:
		return nil, fmt.Errorf("unknown header type %q", headerType)
	}
}

func serialize(value any) (string, error) {
	switch casted := value.(type) {
	case rfc9651.Item:
		return casted.Serialize()
	case rfc9651.List:
		return casted.Serialize()
	case rfc9651.Dictionary:
		return casted.Serialize()
	default:
		return "", fmt.Errorf("unknown type %T", value)
	}
}

func TestStructuredFieldTests_parse(t *testing.T) {
	var count, canFailFailed int

	for filename, tests := range loadSFTests(t, "*.json") {
		for _, test := range tests {
			count++

			actual, err := parse(test.HeaderType, test.Raw)

			if test.MustFail {
				if nil == err {
					t.Errorf("%s: %q: expected a parse error, but parsing succeeded: %#v", filename, test.Name, actual)
				}
				continue
			}
			if nil != err {
				if test.CanFail {
					canFailFailed++
					t.Logf("%s: %q: failed (which is allowed): %s", filename, test.Name, err)
					continue
				}
				t.Errorf("%s: %q: did not expect an error but actually got one: (%T) %s", filename, test.Name, err, err)
				continue
			}

			expected, err := fromJSON(test.HeaderType, test.Expected)
			if nil != err {
				t.Errorf("%s: %q: bad test expectation: %s", filename, test.Name, err)
				continue
			}
			if !reflect.DeepEqual(expected, actual) {
				t.Errorf("%s: %q: the parsed value is not what was expected.", filename, test.Name)
				t.Logf("EXPECTED: %#v", expected)
				t.Logf("ACTUAL:   %#v", actual)
				continue
			}

			// The parsed value serializes to the canonical form.
			var canonical string = strings.Join(test.Raw, ", ")
			if nil != test.Canonical {
				canonical = strings.Join(test.Canonical, "")
			}
			serialized, err := serialize(actual)
			if nil != err {
				t.Errorf("%s: %q: did not expect a serialization error but actually got one: (%T) %s", filename, test.Name, err, err)
				continue
			}
			if canonical != serialized {
				t.Errorf("%s: %q: the serialized value is not what was expected.", filename, test.Name)
				t.Logf("EXPECTED: %q", canonical)
				t.Logf("ACTUAL:   %q", serialized)
			}
		}
	}

	t.Logf("%d parse tests (%d allowed failures taken).", count, canFailFailed)
}

func TestStructuredFieldTests_serialize(t *testing.T) {
	var count int

	for filename, tests := range loadSFTests(t, "serialisation-tests/*.json") {
		for _, test := range tests {
			count++

			value, err := fromJSON(test.HeaderType, test.Expected)
			if nil != err {
				// A value that this package's types cannot even hold (ex: a decimal out of range) cannot be serialized.
				if test.MustFail {
					continue
				}
				t.Errorf("%s: %q: did not expect an error but actually got one: (%T) %s", filename, test.Name, err, err)
				continue
			}

			serialized, err := serialize(value)
			if test.MustFail {
				if nil == err {
					t.Errorf("%s: %q: expected a serialization error, but got %q.", filename, test.Name, serialized)
				}
				continue
			}
			if nil != err {
				t.Errorf("%s: %q: did not expect an error but actually got one: (%T) %s", filename, test.Name, err, err)
				continue
			}
			if expected := strings.Join(test.Canonical, ""); expected != serialized {
				t.Errorf("%s: %q: expected %q but actually got %q.", filename, test.Name, expected, serialized)
			}
		}
	}

	t.Logf("%d serialization tests.", count)
}
