package admin

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// PHP's serialize() format, for the one value Go and Laravel share as structured data.
//
// Laravel's cache stores anything non-numeric as serialize() output and reads it back
// through unserialize(), so a value written from Go has to be in that format or the Blade
// layout gets a warning and no announcement. Everywhere else this codebase avoids the
// problem by only ever writing serialize(true) or deleting the key — see
// publicpost.RedisFreshness — but the announcement is a record, and it is read by PHP and
// written by Go.
//
// Only what the announcement needs is implemented: a flat associative array whose values
// are strings, integers or null. Anything else in the stored value is an error rather than
// a guess, because a guess would be a silently wrong announcement.

var errUnsupportedPHPValue = errors.New("admin: unsupported php serialized value")

// phpArray is one flat associative array. The order is preserved because PHP arrays are
// ordered and the serialised form encodes them positionally.
type phpArray struct {
	keys   []string
	values map[string]any
}

func newPHPArray() *phpArray {
	return &phpArray{values: map[string]any{}}
}

func (array *phpArray) set(key string, value any) {
	if _, exists := array.values[key]; !exists {
		array.keys = append(array.keys, key)
	}
	array.values[key] = value
}

func (array *phpArray) string(key string) string {
	value, _ := array.values[key].(string)
	return value
}

func (array *phpArray) int(key string) int {
	value, _ := array.values[key].(int)
	return value
}

// encode writes the array as serialize() would.
func (array *phpArray) encode() (string, error) {
	var builder strings.Builder
	fmt.Fprintf(&builder, "a:%d:{", len(array.keys))
	for _, key := range array.keys {
		writePHPString(&builder, key)
		switch value := array.values[key].(type) {
		case nil:
			builder.WriteString("N;")
		case string:
			writePHPString(&builder, value)
		case int:
			fmt.Fprintf(&builder, "i:%d;", value)
		case bool:
			if value {
				builder.WriteString("b:1;")
			} else {
				builder.WriteString("b:0;")
			}
		default:
			return "", fmt.Errorf("%w: %T", errUnsupportedPHPValue, value)
		}
	}
	builder.WriteString("}")
	return builder.String(), nil
}

// writePHPString writes s:<byte length>:"<value>";.
//
// The length is in bytes, not characters, which matters for every announcement written in
// Chinese: a rune count here would make PHP read past the end of the string.
func writePHPString(builder *strings.Builder, value string) {
	fmt.Fprintf(builder, `s:%d:"%s";`, len(value), value)
}

// decodePHPArray reads what encode wrote, and what Laravel's own serialize() writes for
// the same shape.
func decodePHPArray(payload string) (*phpArray, error) {
	rest, ok := trimPrefix(payload, "a:")
	if !ok {
		return nil, fmt.Errorf("%w: not an array", errUnsupportedPHPValue)
	}
	count, rest, err := readInt(rest, ':')
	if err != nil {
		return nil, err
	}
	rest, ok = trimPrefix(rest, "{")
	if !ok {
		return nil, fmt.Errorf("%w: missing the array body", errUnsupportedPHPValue)
	}

	array := newPHPArray()
	for index := 0; index < count; index++ {
		key, remainder, err := readPHPString(rest)
		if err != nil {
			return nil, err
		}
		value, remainder, err := readPHPScalar(remainder)
		if err != nil {
			return nil, err
		}
		array.set(key, value)
		rest = remainder
	}
	if rest, _ = trimPrefix(rest, "}"); strings.TrimSpace(rest) != "" {
		return nil, fmt.Errorf("%w: trailing bytes after the array", errUnsupportedPHPValue)
	}
	return array, nil
}

func readPHPScalar(payload string) (any, string, error) {
	switch {
	case strings.HasPrefix(payload, "N;"):
		return nil, payload[len("N;"):], nil
	case strings.HasPrefix(payload, "s:"):
		return readPHPString(payload)
	case strings.HasPrefix(payload, "i:"):
		rest := payload[len("i:"):]
		value, rest, err := readInt(rest, ';')
		if err != nil {
			return nil, "", err
		}
		return value, rest, nil
	case strings.HasPrefix(payload, "b:"):
		rest := payload[len("b:"):]
		value, rest, err := readInt(rest, ';')
		if err != nil {
			return nil, "", err
		}
		return value == 1, rest, nil
	default:
		return nil, "", fmt.Errorf("%w: %.8s", errUnsupportedPHPValue, payload)
	}
}

// readPHPString reads s:<length>:"<value>"; and answers the value and what follows.
func readPHPString(payload string) (string, string, error) {
	rest, ok := trimPrefix(payload, "s:")
	if !ok {
		return "", "", fmt.Errorf("%w: expected a string", errUnsupportedPHPValue)
	}
	length, rest, err := readInt(rest, ':')
	if err != nil {
		return "", "", err
	}
	// The length is authoritative and the value may contain quotes, so the closing quote
	// is found by counting bytes rather than by searching.
	if length < 0 || len(rest) < length+3 || rest[0] != '"' {
		return "", "", fmt.Errorf("%w: malformed string", errUnsupportedPHPValue)
	}
	value := rest[1 : 1+length]
	if rest[1+length] != '"' || rest[2+length] != ';' {
		return "", "", fmt.Errorf("%w: string is not %d bytes long", errUnsupportedPHPValue, length)
	}
	return value, rest[3+length:], nil
}

func readInt(payload string, terminator byte) (int, string, error) {
	end := strings.IndexByte(payload, terminator)
	if end < 0 {
		return 0, "", fmt.Errorf("%w: unterminated number", errUnsupportedPHPValue)
	}
	value, err := strconv.Atoi(payload[:end])
	if err != nil {
		return 0, "", fmt.Errorf("%w: %v", errUnsupportedPHPValue, err)
	}
	return value, payload[end+1:], nil
}

func trimPrefix(payload, prefix string) (string, bool) {
	if !strings.HasPrefix(payload, prefix) {
		return payload, false
	}
	return payload[len(prefix):], true
}
