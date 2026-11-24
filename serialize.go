package rfc9651

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"codeberg.org/reiver/go-erorr"
	"codeberg.org/reiver/go-field"
)

// Serialize returns the item's canonical field value.
func (receiver Item) Serialize() (string, error) {
	var builder strings.Builder
	if err := serializeItem(&builder, receiver); nil != err {
		return "", err
	}
	return builder.String(), nil
}

// Serialize returns the list's canonical field value.
//
// An empty list serializes to "" — which means the field should be left out.
func (receiver List) Serialize() (string, error) {
	var builder strings.Builder
	for index, member := range receiver {
		if 0 < index {
			builder.WriteString(", ")
		}
		if err := serializeMember(&builder, member); nil != err {
			return "", err
		}
	}
	return builder.String(), nil
}

// Serialize returns the dictionary's canonical field value.
//
// An empty dictionary serializes to "" — which means the field should be left out.
func (receiver Dictionary) Serialize() (string, error) {
	var builder strings.Builder
	for index, member := range receiver {
		if 0 < index {
			builder.WriteString(", ")
		}
		if err := serializeKey(&builder, member.Key); nil != err {
			return "", err
		}

		// A member whose value is the Boolean true is written as just its key (and parameters).
		if item, casted := member.Value.(Item); casted {
			if b, casted := item.Value.(bool); casted && b {
				if err := serializeParams(&builder, item.Params); nil != err {
					return "", err
				}
				continue
			}
		}

		builder.WriteByte('=')
		if err := serializeMember(&builder, member.Value); nil != err {
			return "", err
		}
	}
	return builder.String(), nil
}

func serializeMember(builder *strings.Builder, member Member) error {
	switch casted := member.(type) {
	case Item:
		return serializeItem(builder, casted)
	case InnerList:
		return serializeInnerList(builder, casted)
	default:
		return erorr.Wrap(ErrBadMember, "bad member", field.String("type", typeName(member)))
	}
}

func serializeInnerList(builder *strings.Builder, innerList InnerList) error {
	builder.WriteByte('(')
	for index, item := range innerList.Items {
		if 0 < index {
			builder.WriteByte(' ')
		}
		if err := serializeItem(builder, item); nil != err {
			return err
		}
	}
	builder.WriteByte(')')
	return serializeParams(builder, innerList.Params)
}

func serializeItem(builder *strings.Builder, item Item) error {
	if err := serializeBareItem(builder, item.Value); nil != err {
		return err
	}
	return serializeParams(builder, item.Params)
}

func serializeParams(builder *strings.Builder, params Params) error {
	for _, param := range params {
		builder.WriteByte(';')
		if err := serializeKey(builder, param.Key); nil != err {
			return err
		}
		if b, casted := param.Value.(bool); casted && b {
			continue
		}
		builder.WriteByte('=')
		if err := serializeBareItem(builder, param.Value); nil != err {
			return err
		}
	}
	return nil
}

func serializeKey(builder *strings.Builder, key string) error {
	if "" == key || (!isLCAlpha(key[0]) && '*' != key[0]) {
		return erorr.Wrap(ErrBadKey, "bad key", field.String("key", key))
	}
	for index := 1; index < len(key); index++ {
		if !isKeyChar(key[index]) {
			return erorr.Wrap(ErrBadKey, "bad key", field.String("key", key))
		}
	}
	builder.WriteString(key)
	return nil
}

func serializeBareItem(builder *strings.Builder, value any) error {
	switch casted := value.(type) {
	case int64:
		return serializeInteger(builder, casted)
	case int:
		return serializeInteger(builder, int64(casted))
	case Decimal:
		if casted.thousandths < -maxThousandths || maxThousandths < casted.thousandths {
			return erorr.Wrap(ErrDecimalOutOfRange, "decimal out of range", field.Int64("thousandths", casted.thousandths))
		}
		builder.WriteString(casted.String())
		return nil
	case string:
		return serializeString(builder, casted)
	case Token:
		return serializeToken(builder, casted)
	case []byte:
		builder.WriteByte(':')
		builder.WriteString(base64.StdEncoding.EncodeToString(casted))
		builder.WriteByte(':')
		return nil
	case bool:
		if casted {
			builder.WriteString("?1")
		} else {
			builder.WriteString("?0")
		}
		return nil
	case Date:
		builder.WriteByte('@')
		return serializeInteger(builder, int64(casted))
	case DisplayString:
		return serializeDisplayString(builder, casted)
	default:
		return erorr.Wrap(ErrBadBareItem, "bad bare item", field.String("type", typeName(value)))
	}
}

func serializeInteger(builder *strings.Builder, n int64) error {
	if n < -maxInteger || maxInteger < n {
		return erorr.Wrap(ErrIntegerOutOfRange, "integer out of range", field.Int64("integer", n))
	}
	builder.WriteString(strconv.FormatInt(n, 10))
	return nil
}

func serializeString(builder *strings.Builder, s string) error {
	for index := 0; index < len(s); index++ {
		if c := s[index]; c < 0x20 || 0x7e < c {
			return erorr.Wrap(ErrBadString, "bad character in string", field.Int("offset", index))
		}
	}

	builder.WriteByte('"')
	for index := 0; index < len(s); index++ {
		var c byte = s[index]
		if '"' == c || '\\' == c {
			builder.WriteByte('\\')
		}
		builder.WriteByte(c)
	}
	builder.WriteByte('"')
	return nil
}

func serializeToken(builder *strings.Builder, token Token) error {
	if "" == token || (!isAlpha(token[0]) && '*' != token[0]) {
		return erorr.Wrap(ErrBadToken, "bad token", field.String("token", string(token)))
	}
	for index := 1; index < len(token); index++ {
		if !isTokenChar(token[index]) {
			return erorr.Wrap(ErrBadToken, "bad token", field.String("token", string(token)))
		}
	}
	builder.WriteString(string(token))
	return nil
}

func serializeDisplayString(builder *strings.Builder, s DisplayString) error {
	if !utf8.ValidString(string(s)) {
		return erorr.Wrap(ErrBadString, "display string is not valid UTF-8")
	}

	const hex = "0123456789abcdef"

	builder.WriteString("%\"")
	for index := 0; index < len(s); index++ {
		var c byte = s[index]
		if '%' == c || '"' == c || c < 0x20 || 0x7e < c {
			builder.WriteByte('%')
			builder.WriteByte(hex[c>>4])
			builder.WriteByte(hex[c&0x0f])
			continue
		}
		builder.WriteByte(c)
	}
	builder.WriteByte('"')
	return nil
}

func typeName(value any) string {
	return fmt.Sprintf("%T", value)
}
