package codec

import (
	"bytes"
	"strings"
	"testing"
)

func TestCSVCodec_FlexibleStructure(t *testing.T) {
	codec := &csvCodec{comma: ','}

	// Input with varying keys and nested structures
	input := []map[string]interface{}{
		{"a": 1, "b": 2},
		{"a": 3, "c": 4},
		{"d": map[string]interface{}{"e": 5}},
	}

	var buf bytes.Buffer
	err := codec.Encode(&buf, input)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 4 { // Header + 3 rows
		t.Fatalf("Expected 4 lines, got %d\nOutput:\n%s", len(lines), output)
	}

	// Check Header
	// Sorted keys: a, b, c, d.e
	expectedHeader := "a,b,c,d.e"
	if lines[0] != expectedHeader {
		t.Errorf("Expected header %q, got %q", expectedHeader, lines[0])
	}

	// Check Rows
	// Row 1: {"a": 1, "b": 2} -> 1,2,,
	if lines[1] != "1,2,," {
		t.Errorf("Row 1 mismatch. Got: %q", lines[1])
	}

	// Row 2: {"a": 3, "c": 4} -> 3,,4,
	if lines[2] != "3,,4," {
		t.Errorf("Row 2 mismatch. Got: %q", lines[2])
	}

	// Row 3: {"d": {"e": 5}} -> ,,,5
	if lines[3] != ",,,5" {
		t.Errorf("Row 3 mismatch. Got: %q", lines[3])
	}
}
