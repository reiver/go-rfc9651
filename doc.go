/*
Package rfc9651 implements Structured Field Values for HTTP (IETF RFC 9651) — parsing and serializing — for the Go programming language.

Structured Fields give HTTP fields a common syntax. A field's value is one of three types:

	• an Item       — ex: 42;q=0.5
	• a List        — ex: sugar, tea, (rum cola);x
	• a Dictionary  — ex: a=1, b, c=("x" "y");p=?0

The field's definition says which. Items and inner lists may have parameters (the ";key=value" parts).

A bare item is one of these Go types:

	┌──────────────────┬──────────────────┬─────────────────────┐
	│ Structured Field │ Go               │ Example             │
	├──────────────────┼──────────────────┼─────────────────────┤
	│ Integer          │ int64            │ 42                  │
	│ Decimal          │ Decimal          │ 4.5                 │
	│ String           │ string           │ "hello"             │
	│ Token            │ Token            │ text/html           │
	│ Byte Sequence    │ []byte           │ :aGVsbG8=:          │
	│ Boolean          │ bool             │ ?1                  │
	│ Date             │ Date             │ @1659578233         │
	│ Display String   │ DisplayString    │ %"f%c3%bc%c3%bc"    │
	└──────────────────┴──────────────────┴─────────────────────┘

(When serializing, an int is accepted as an Integer too.)

RFC 9651 is here:

https://www.rfc-editor.org/rfc/rfc9651

# Parsing

[ParseItem], [ParseList], and [ParseDictionary] follow RFC 9651's parsing algorithms exactly.
On any error (always [ErrParse]), the whole field must be ignored.

For a field with more than one line, join the lines with ", " first:

	list, err := rfc9651.ParseList(strings.Join(request.Header.Values("Example"), ", "))

As RFC 9651 recommends, a byte sequence with missing "=" padding, or with non-zero pad bits, is accepted.

# Serializing

[Item.Serialize], [List.Serialize], and [Dictionary.Serialize] give the canonical form.
An empty List or Dictionary serializes to "" — which means the field should be left out.

# Example

	dictionary, err := rfc9651.ParseDictionary(`sig1=("@method" "@target-uri");created=1618884473;keyid="key-1"`)
	if nil != err {
		return err
	}

	member, found := dictionary.Get("sig1")
	innerList, casted := member.(rfc9651.InnerList)
	created, found := innerList.Params.Get("created") // int64(1618884473)

	q, err := rfc9651.DecimalFromFloat64(0.5)
	if nil != err {
		return err
	}

	s, err := rfc9651.Item{Value: rfc9651.Token("gzip"), Params: rfc9651.Params{{Key: "q", Value: q}}}.Serialize()
	// s == "gzip;q=0.5"
*/
package rfc9651
