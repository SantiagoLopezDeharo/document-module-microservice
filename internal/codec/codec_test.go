package codec

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
)

func TestJSONCodec(t *testing.T) {
	codec := &jsonCodec{}

	tests := []struct {
		name  string
		input string
		want  interface{}
	}{
		{
			name:  "simple object",
			input: `{"a": 1, "b": "test"}`,
			want:  map[string]interface{}{"a": float64(1), "b": "test"},
		},
		{
			name:  "array of objects",
			input: `[{"a": 1}, {"a": 2}]`,
		},
		{
			name:  "nested structure",
			input: `{"outer": {"inner": {"value": 42}}}`,
		},
		{
			name:  "empty object",
			input: `{}`,
			want:  map[string]interface{}{},
		},
		{
			name:  "empty array",
			input: `[]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := codec.Decode(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			var buf bytes.Buffer
			err = codec.Encode(&buf, decoded)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			decoded2, err := codec.Decode(&buf)
			if err != nil {
				t.Fatalf("Second decode failed: %v", err)
			}

			if decoded2 == nil {
				t.Fatal("Decoded result is nil")
			}
		})
	}
}

func TestXMLCodec(t *testing.T) {
	codec := &xmlCodec{}

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple element",
			input: `<root><a>1</a><b>test</b></root>`,
		},
		{
			name:  "nested elements",
			input: `<root><outer><inner>value</inner></outer></root>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := codec.Decode(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			var buf bytes.Buffer
			err = codec.Encode(&buf, decoded)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
		})
	}
}

func TestCSVCodec(t *testing.T) {
	codec := &csvCodec{comma: ','}

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple csv",
			input: "a,b\n1,2\n3,4\n",
		},
		{
			name:  "single row",
			input: "name,age\nJohn,30\n",
		},
		{
			name:  "empty values",
			input: "a,b,c\n1,,3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := codec.Decode(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			var buf bytes.Buffer
			err = codec.Encode(&buf, decoded)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
		})
	}
}

func TestTSVCodec(t *testing.T) {
	codec := &csvCodec{comma: '\t'}

	input := "a\tb\n1\t2\n3\t4\n"

	decoded, err := codec.Decode(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	var buf bytes.Buffer
	err = codec.Encode(&buf, decoded)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
}

func TestTOMLCodec(t *testing.T) {
	codec := &tomlCodec{}

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple table",
			input: "a = 1\nb = \"test\"\n",
		},
		{
			name:  "nested table",
			input: "[outer.inner]\nvalue = 42\n",
		},
		{
			name:  "array of tables",
			input: "[[items]]\na = 1\n\n[[items]]\na = 2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := codec.Decode(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			var buf bytes.Buffer
			err = codec.Encode(&buf, decoded)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
		})
	}
}

func TestConversionRoundTrips(t *testing.T) {
	testData := []map[string]interface{}{
		{"id": 1.0, "name": "Alice", "active": true},
		{"id": 2.0, "name": "Bob", "active": false},
	}

	formats := []models.Format{
		models.FormatJSON,
		models.FormatCSV,
		models.FormatTSV,
		models.FormatTOML,
	}

	for _, fromFormat := range formats {
		for _, toFormat := range formats {
			t.Run(string(fromFormat)+"_to_"+string(toFormat), func(t *testing.T) {
				fromCodec, err := GetCodec(fromFormat)
				if err != nil {
					t.Fatalf("Failed to get source codec: %v", err)
				}

				toCodec, err := GetCodec(toFormat)
				if err != nil {
					t.Fatalf("Failed to get target codec: %v", err)
				}

				var buf1 bytes.Buffer
				err = fromCodec.Encode(&buf1, testData)
				if err != nil {
					t.Fatalf("Failed to encode to %s: %v", fromFormat, err)
				}

				decoded, err := fromCodec.Decode(&buf1)
				if err != nil {
					t.Fatalf("Failed to decode from %s: %v", fromFormat, err)
				}

				var buf2 bytes.Buffer
				err = toCodec.Encode(&buf2, decoded)
				if err != nil {
					t.Fatalf("Failed to encode to %s: %v", toFormat, err)
				}

				_, err = toCodec.Decode(&buf2)
				if err != nil {
					t.Fatalf("Failed to decode from %s: %v", toFormat, err)
				}
			})
		}
	}

	xmlTests := []struct {
		from models.Format
		to   models.Format
	}{
		{models.FormatJSON, models.FormatXML},
		{models.FormatCSV, models.FormatXML},
		{models.FormatTSV, models.FormatXML},
		{models.FormatTOML, models.FormatXML},
	}

	for _, test := range xmlTests {
		t.Run(string(test.from)+"_to_XML_oneway", func(t *testing.T) {
			fromCodec, _ := GetCodec(test.from)
			xmlCodec, _ := GetCodec(models.FormatXML)

			var buf1 bytes.Buffer
			err := fromCodec.Encode(&buf1, testData)
			if err != nil {
				t.Fatalf("Failed to encode to %s: %v", test.from, err)
			}

			decoded, err := fromCodec.Decode(&buf1)
			if err != nil {
				t.Fatalf("Failed to decode from %s: %v", test.from, err)
			}

			var buf2 bytes.Buffer
			err = xmlCodec.Encode(&buf2, decoded)
			if err != nil {
				t.Fatalf("Failed to encode to XML: %v", err)
			}

			if buf2.Len() == 0 {
				t.Fatal("XML output is empty")
			}
		})
	}
}

func TestComplexStructures(t *testing.T) {
	complexData := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"id":    1.0,
				"name":  "Alice",
				"email": "alice@example.com",
				"tags":  []interface{}{"admin", "developer"},
			},
			map[string]interface{}{
				"id":    2.0,
				"name":  "Bob",
				"email": "bob@example.com",
				"tags":  []interface{}{"user"},
			},
		},
		"metadata": map[string]interface{}{
			"version": "1.0",
			"created": "2025-12-20",
		},
	}

	jsonCodec := &jsonCodec{}
	var buf bytes.Buffer
	err := jsonCodec.Encode(&buf, complexData)
	if err != nil {
		t.Fatalf("Failed to encode complex data: %v", err)
	}

	decoded, err := jsonCodec.Decode(&buf)
	if err != nil {
		t.Fatalf("Failed to decode complex data: %v", err)
	}

	xmlCodec := &xmlCodec{}
	var xmlBuf bytes.Buffer
	err = xmlCodec.Encode(&xmlBuf, decoded)
	if err != nil {
		t.Fatalf("Failed to encode to XML: %v", err)
	}

	tomlCodec := &tomlCodec{}
	var tomlBuf bytes.Buffer
	err = tomlCodec.Encode(&tomlBuf, decoded)
	if err != nil {
		t.Fatalf("Failed to encode to TOML: %v", err)
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("empty csv array", func(t *testing.T) {
		codec := &csvCodec{comma: ','}
		var buf bytes.Buffer
		err := codec.Encode(&buf, []map[string]interface{}{})
		if err != nil {
			t.Fatalf("Failed to encode empty array: %v", err)
		}
	})

	t.Run("single object to csv", func(t *testing.T) {
		codec := &csvCodec{comma: ','}
		data := map[string]interface{}{"a": 1, "b": 2}
		var buf bytes.Buffer
		err := codec.Encode(&buf, data)
		if err != nil {
			t.Fatalf("Failed to encode single object: %v", err)
		}
	})

	t.Run("array to toml wraps in root", func(t *testing.T) {
		codec := &tomlCodec{}
		data := []interface{}{
			map[string]interface{}{"a": 1},
			map[string]interface{}{"a": 2},
		}
		var buf bytes.Buffer
		err := codec.Encode(&buf, data)
		if err != nil {
			t.Fatalf("Failed to encode array to TOML: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "[[root]]") {
			t.Errorf("Expected TOML output to wrap array in [[root]], got: %s", output)
		}
	})
}
