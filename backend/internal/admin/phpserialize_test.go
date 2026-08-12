package admin

import (
	"errors"
	"strings"
	"testing"
)

// The length prefix counts bytes. PHP's unserialize() reads exactly that many, so a rune
// count on a Chinese string would leave it reading into the next field.
func TestEncodingCountsBytesNotRunes(t *testing.T) {
	array := newPHPArray()
	array.set("content", "嗨嗨")

	payload, err := array.encode()
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}
	if payload != `a:1:{s:7:"content";s:6:"嗨嗨";}` {
		t.Errorf("payload = %s, want a 6-byte length", payload)
	}
}

func TestEncodedValuesRoundTrip(t *testing.T) {
	array := newPHPArray()
	array.set("id", "9f1c")
	array.set("image_url", nil)
	array.set("keep_minutes", 60)
	array.set("content", "a\"quoted\" string; with :colons:")

	payload, err := array.encode()
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}
	decoded, err := decodePHPArray(payload)
	if err != nil {
		t.Fatalf("decodePHPArray() error = %v", err)
	}

	if got := decoded.string("id"); got != "9f1c" {
		t.Errorf("id = %q, want 9f1c", got)
	}
	if got := decoded.string("content"); got != `a"quoted" string; with :colons:` {
		t.Errorf("content = %q, want the quotes and semicolons intact", got)
	}
	if got := decoded.int("keep_minutes"); got != 60 {
		t.Errorf("keep minutes = %d, want 60", got)
	}
	if value, exists := decoded.values["image_url"]; !exists || value != nil {
		t.Errorf("image url = %v (present %v), want a present null", value, exists)
	}
}

// The order is part of the format: PHP arrays are ordered and the serialised form encodes
// them positionally.
func TestSettingAKeyTwiceKeepsItsFirstPosition(t *testing.T) {
	array := newPHPArray()
	array.set("a", 1)
	array.set("b", 2)
	array.set("a", 3)

	payload, err := array.encode()
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}
	if payload != `a:2:{s:1:"a";i:3;s:1:"b";i:2;}` {
		t.Errorf("payload = %s, want a first with its new value", payload)
	}
}

func TestDecodingWhatLaravelWrites(t *testing.T) {
	// serialize(['id' => 'abc', 'keep_minutes' => 60, 'image_url' => null]) from PHP.
	decoded, err := decodePHPArray(`a:3:{s:2:"id";s:3:"abc";s:12:"keep_minutes";i:60;s:9:"image_url";N;}`)
	if err != nil {
		t.Fatalf("decodePHPArray() error = %v", err)
	}
	if decoded.string("id") != "abc" || decoded.int("keep_minutes") != 60 {
		t.Errorf("decoded = %+v, want id abc and 60 minutes", decoded.values)
	}
}

// A boolean is what every other shared cache entry in this codebase holds, so the decoder
// reads one rather than refusing a value Laravel might have written.
func TestDecodingABoolean(t *testing.T) {
	decoded, err := decodePHPArray(`a:2:{s:1:"t";b:1;s:1:"f";b:0;}`)
	if err != nil {
		t.Fatalf("decodePHPArray() error = %v", err)
	}
	if decoded.values["t"] != true || decoded.values["f"] != false {
		t.Errorf("decoded = %+v, want true and false", decoded.values)
	}
}

// Anything this decoder does not cover is an error, not a guess.
func TestUnsupportedPayloadsAreRefused(t *testing.T) {
	payloads := map[string]string{
		"an object":            `O:8:"stdClass":1:{s:2:"id";s:1:"a";}`,
		"a nested array":       `a:1:{s:1:"a";a:0:{}}`,
		"a float":              `a:1:{s:1:"a";d:1.5;}`,
		"a scalar":             `s:3:"abc";`,
		"a truncated string":   `a:1:{s:1:"a";s:9:"short";}`,
		"a missing body":       `a:1:`,
		"trailing bytes":       `a:1:{s:1:"a";i:1;}garbage`,
		"a non-numeric length": `a:x:{}`,
	}
	for name, payload := range payloads {
		if _, err := decodePHPArray(payload); err == nil {
			t.Errorf("%s: decodePHPArray(%s) error = nil, want an error", name, payload)
		}
	}
}

func TestEncodingRefusesAValueItCannotWrite(t *testing.T) {
	array := newPHPArray()
	array.set("ratio", 1.5)

	_, err := array.encode()
	if !errors.Is(err, errUnsupportedPHPValue) {
		t.Errorf("encode() error = %v, want errUnsupportedPHPValue", err)
	}
}

// Reading a field the array does not hold, or holds as another type, is the zero value:
// the announcement's own validation is what refuses an incomplete one.
func TestReadingAMissingOrMistypedFieldGivesTheZeroValue(t *testing.T) {
	array := newPHPArray()
	array.set("keep_minutes", "60")

	if got := array.string("absent"); got != "" {
		t.Errorf("string(absent) = %q, want empty", got)
	}
	if got := array.int("keep_minutes"); got != 0 {
		t.Errorf("int(keep_minutes) = %d, want 0 for a string value", got)
	}
}

func TestALongStringSurvivesTheRoundTrip(t *testing.T) {
	long := strings.Repeat("公告", 1000)
	array := newPHPArray()
	array.set("content", long)

	payload, err := array.encode()
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}
	decoded, err := decodePHPArray(payload)
	if err != nil {
		t.Fatalf("decodePHPArray() error = %v", err)
	}
	if decoded.string("content") != long {
		t.Error("content did not survive the round trip")
	}
}
