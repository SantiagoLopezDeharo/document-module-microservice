package codec

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
)

func TestYAMLCodec(t *testing.T) {
	codec := &yamlCodec{}
	input := "a: 1\nb: test\n"

	decoded, err := codec.Decode([]byte(input))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	var buf bytes.Buffer
	err = codec.Encode(&buf, decoded)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
}

func TestMsgPackCodec(t *testing.T) {
	codec := &msgpackCodec{}
	data := map[string]interface{}{"a": 1, "b": "test"}

	var buf bytes.Buffer
	err := codec.Encode(&buf, data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := codec.Decode(buf.Bytes())
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// MsgPack decodes numbers as int8/int64 etc, JSON as float64.
	// We just check if it's not nil for now.
	if decoded == nil {
		t.Fatal("Decoded nil")
	}
}

func TestPropertiesCodec(t *testing.T) {
	codec := &propertiesCodec{}
	input := "key=value\nnested.key=value2\n"

	decoded, err := codec.Decode([]byte(input))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	var buf bytes.Buffer
	err = codec.Encode(&buf, decoded)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "key=value") {
		t.Errorf("Expected output to contain key=value, got %s", output)
	}
}

func TestXLSXCodec(t *testing.T) {
	codec := &xlsxCodec{}
	data := []map[string]interface{}{
		{"Name": "Alice", "Age": 30},
		{"Name": "Bob", "Age": 25},
	}

	var buf bytes.Buffer
	err := codec.Encode(&buf, data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := codec.Decode(buf.Bytes())
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	res, ok := decoded.([]map[string]interface{})
	if !ok {
		t.Fatalf("Expected []map[string]interface{}, got %T", decoded)
	}

	if len(res) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(res))
	}
}

func TestNewFormatsRoundTrip(t *testing.T) {
	testData := []map[string]interface{}{
		{"id": 1, "name": "Alice"},
		{"id": 2, "name": "Bob"},
	}

	formats := []models.Format{
		models.FormatJSON,
		models.FormatYAML,
		models.FormatMsgPack,
		// XLSX is binary and strict about structure, tested separately or carefully
		// Properties is flat, tested separately
	}

	for _, from := range formats {
		for _, to := range formats {
			t.Run(string(from)+"_to_"+string(to), func(t *testing.T) {
				fromCodec, _ := GetCodec(from)
				toCodec, _ := GetCodec(to)

				var buf1 bytes.Buffer
				if err := fromCodec.Encode(&buf1, testData); err != nil {
					t.Fatalf("Encode %s failed: %v", from, err)
				}

				decoded, err := fromCodec.Decode(buf1.Bytes())
				if err != nil {
					t.Fatalf("Decode %s failed: %v", from, err)
				}

				var buf2 bytes.Buffer
				if err := toCodec.Encode(&buf2, decoded); err != nil {
					t.Fatalf("Encode %s failed: %v", to, err)
				}
			})
		}
	}
}
