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
