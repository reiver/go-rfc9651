package rfc9651

// A bare item is an int64, Decimal, string, Token, []byte, bool, Date, or DisplayString. (See the package documentation.)

// Token is a Structured Field token (ex: foo, text/html, *).
type Token string

// Date is a Structured Field date — seconds since the UNIX epoch.
type Date int64

// DisplayString is a Structured Field display string — Unicode text (ex: %"f%c3%bc%c3%bc" is "füü").
type DisplayString string

// Param is one parameter.
type Param struct {
	Key   string
	Value any // a bare item
}

// Params are an item's (or inner list's) parameters, in order.
type Params []Param

// Get returns the value of the parameter 'key'.
func (receiver Params) Get(key string) (any, bool) {
	for _, param := range receiver {
		if key == param.Key {
			return param.Value, true
		}
	}
	return nil, false
}

// set sets the parameter 'key' — overwriting it (in place) if it is already there.
func (receiver Params) set(key string, value any) Params {
	for index := range receiver {
		if key == receiver[index].Key {
			receiver[index].Value = value
			return receiver
		}
	}
	return append(receiver, Param{Key: key, Value: value})
}

// Member is a member of a [List] or a [Dictionary] — either an [Item] or an [InnerList].
type Member interface {
	member()
}

// Item is a bare item with parameters.
type Item struct {
	Value  any // a bare item
	Params Params
}

func (Item) member() {}

// InnerList is a list of items, with parameters.
type InnerList struct {
	Items  []Item
	Params Params
}

func (InnerList) member() {}

// List is a Structured Field list.
type List []Member

// DictionaryMember is one member of a [Dictionary].
type DictionaryMember struct {
	Key   string
	Value Member
}

// Dictionary is a Structured Field dictionary, in order.
type Dictionary []DictionaryMember

// Get returns the value of the member 'key'.
func (receiver Dictionary) Get(key string) (Member, bool) {
	for _, member := range receiver {
		if key == member.Key {
			return member.Value, true
		}
	}
	return nil, false
}

// set sets the member 'key' — overwriting it (in place) if it is already there.
func (receiver Dictionary) set(key string, value Member) Dictionary {
	for index := range receiver {
		if key == receiver[index].Key {
			receiver[index].Value = value
			return receiver
		}
	}
	return append(receiver, DictionaryMember{Key: key, Value: value})
}
