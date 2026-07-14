### Task 1: Project Scaffolding + Base62 Encoding

**Files:**
- Create: `go.mod`
- Create: `internal/shortcode/base62.go`
- Create: `internal/shortcode/base62_test.go`
- Create: `.gitignore`

**Interfaces:**
- Produces: `shortcode.Encode(id int64) string`, `shortcode.Decode(code string) (int64, error)`

- [ ] **Step 1: Initialize Go module and .gitignore**

```bash
cd d:\Projects\paytm-assignment
go mod init url-shortener
```

Create `.gitignore`:
```gitignore
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
/server

# Test binary
*.test

# Go workspace
go.work

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
Thumbs.db

# Environment
.env
```

- [ ] **Step 2: Write Base62 tests**

Create `internal/shortcode/base62_test.go`:

```go
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
```

- [ ] **Step 3: Run tests â€” verify they fail**

```bash
go test ./internal/shortcode/ -v
```

Expected: compilation error (functions not defined)

- [ ] **Step 4: Implement Base62 encode/decode**

Create `internal/shortcode/base62.go`:

```go
package shortcode

import (
	"fmt"
	"strings"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var base = int64(len(alphabet))

// Encode converts a positive integer ID to a Base62 string.
func Encode(id int64) string {
	if id == 0 {
		return string(alphabet[0])
	}

	var chars []byte
	for id > 0 {
		remainder := id % base
		chars = append([]byte{alphabet[remainder]}, chars...)
		id /= base
	}
	return string(chars)
}

// Decode converts a Base62 string back to an integer ID.
func Decode(code string) (int64, error) {
	var id int64
	for _, c := range code {
		idx := strings.IndexRune(alphabet, c)
		if idx == -1 {
			return 0, fmt.Errorf("invalid character in short code: %q", c)
		}
		id = id*base + int64(idx)
	}
	return id, nil
}
```

- [ ] **Step 5: Run tests â€” verify they pass**

```bash
go test ./internal/shortcode/ -v
```

Expected: all 6 tests PASS

- [ ] **Step 6: Commit**

```bash
git add .
git commit -m "feat: project scaffolding and Base62 encoding with tests"
```
