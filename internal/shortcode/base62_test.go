package shortcode

import (
	"testing"
)

func TestEncodeKnownValues(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "a"},
		{35, "z"},
		{36, "A"},
		{61, "Z"},
		{62, "10"},
		{63, "11"},
		{3843, "ZZ"},
		{3844, "100"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := Encode(tt.input)
			if result != tt.expected {
				t.Errorf("Encode(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDecodeKnownValues(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"0", 0},
		{"1", 1},
		{"Z", 61},
		{"10", 62},
		{"ZZ", 3843},
		{"100", 3844},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := Decode(tt.input)
			if err != nil {
				t.Fatalf("Decode(%q) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("Decode(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	for i := int64(0); i < 100000; i++ {
		encoded := Encode(i)
		decoded, err := Decode(encoded)
		if err != nil {
			t.Fatalf("Decode(Encode(%d)) error: %v", i, err)
		}
		if decoded != i {
			t.Fatalf("Decode(Encode(%d)) = %d", i, decoded)
		}
	}
}

func TestDecodeInvalidCharacter(t *testing.T) {
	_, err := Decode("abc!")
	if err == nil {
		t.Error("expected error for invalid character, got nil")
	}
}

func TestEncodeOutputIsURLSafe(t *testing.T) {
	for i := int64(0); i < 10000; i++ {
		code := Encode(i)
		for _, c := range code {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
				t.Fatalf("Encode(%d) = %q contains non-URL-safe char %q", i, code, c)
			}
		}
	}
}

func TestEncodeNoDuplicates(t *testing.T) {
	seen := make(map[string]int64)
	for i := int64(0); i < 100000; i++ {
		code := Encode(i)
		if prev, ok := seen[code]; ok {
			t.Fatalf("collision: Encode(%d) = Encode(%d) = %q", i, prev, code)
		}
		seen[code] = i
	}
}
