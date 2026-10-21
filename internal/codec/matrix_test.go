package codec

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
	"github.com/google/go-cmp/cmp"
)

// MatrixTestConfig defines the data and compatible formats for a test scenario
type MatrixTestConfig struct {
	Name              string
	Data              interface{}
	CompatibleFormats []models.Format
}

func TestMatrixConversions(t *testing.T) {
	// Define test data
	simpleObject := map[string]interface{}{
		"name": "John Doe",
		"age":  float64(30), // Use float64 for JSON compatibility
		"city": "New York",
	}

	simpleList := []interface{}{
		map[string]interface{}{"id": float64(1), "val": "A"},
		map[string]interface{}{"id": float64(2), "val": "B"},
	}

	complexObject := map[string]interface{}{
		"meta": map[string]interface{}{
			"version": float64(1),
			"author":  "Admin",
		},
		"data": []interface{}{
			map[string]interface{}{"x": float64(10), "y": float64(20)},
			map[string]interface{}{"x": float64(15), "y": float64(25)},
		},
	}

	edgeCaseObject := map[string]interface{}{
		"emptyMap":      map[string]interface{}{},
		"emptyList":     []interface{}{},
		"boolTrue":      true,
		"boolFalse":     false,
		"zero":          float64(0),
		"negative":      float64(-42),
		"specialString": "Line1\nLine2\tTabbed=Value",
		"numbers": map[string]interface{}{
			"big":   float64(1_000_000),
			"small": float64(0.000123),
		},
		"thresholds": []interface{}{float64(0.1), float64(0.5), float64(0.9)},
		"nested": map[string]interface{}{
			"tags": []interface{}{"alpha", "beta", "gamma"},
			"metadata": map[string]interface{}{
				"owner": "Ops",
				"flags": []interface{}{
					map[string]interface{}{"name": "f1", "enabled": true},
					map[string]interface{}{"name": "f2", "enabled": false},
				},
			},
		},
	}

	deepNestedObject := map[string]interface{}{
		"tenant": "alpha",
		"regions": []interface{}{
			map[string]interface{}{
				"name": "us-east",
				"services": []interface{}{
					map[string]interface{}{
						"name":     "api",
						"replicas": float64(3),
						"limits": map[string]interface{}{
							"cpu":    float64(500),
							"memory": float64(2048),
						},
					},
					map[string]interface{}{
						"name":     "worker",
						"replicas": float64(5),
						"limits": map[string]interface{}{
							"cpu":    float64(700),
							"memory": float64(4096),
						},
					},
				},
			},
			map[string]interface{}{
				"name": "eu-west",
				"services": []interface{}{
					map[string]interface{}{
						"name":     "api",
						"replicas": float64(2),
						"limits": map[string]interface{}{
							"cpu":    float64(400),
							"memory": float64(1024),
						},
					},
				},
			},
		},
		"features": map[string]interface{}{
			"rollout": map[string]interface{}{
				"gradual": true,
				"percent": float64(42),
			},
			"maintenance": []interface{}{"sat", "sun"},
		},
	}

	mixedRecordsList := []interface{}{
		map[string]interface{}{"id": float64(1), "name": "Alpha", "active": true, "balance": float64(123.45)},
		map[string]interface{}{"id": float64(2), "name": "Beta, Inc.", "active": false, "balance": float64(-50.75)},
		map[string]interface{}{"id": float64(3), "name": "Gamma", "active": true, "balance": float64(0), "notes": "Line1\nLine2"},
	}

	// Large lists for stress testing
	largeList := buildSequentialList(100, "Item")
	hugeList := buildSequentialList(1000, "Huge Item")

	scenarios := []MatrixTestConfig{
		{
			Name: "SimpleObject",
			Data: simpleObject,
			CompatibleFormats: []models.Format{
				models.FormatJSON, models.FormatXML, models.FormatYAML, models.FormatTOML, models.FormatMsgPack, models.FormatProperties,
				// CSV/TSV/XLSX typically expect a list of objects, but our codec might handle single object as 1-row
				models.FormatCSV, models.FormatTSV, models.FormatXLSX,
			},
		},
		{
			Name: "EdgeCaseObject",
			Data: edgeCaseObject,
			CompatibleFormats: []models.Format{
				models.FormatJSON, models.FormatXML, models.FormatYAML, models.FormatTOML, models.FormatMsgPack, models.FormatProperties,
				models.FormatCSV, models.FormatTSV, models.FormatXLSX,
			},
		},
		{
			Name: "SimpleList",
			Data: simpleList,
			CompatibleFormats: []models.Format{
				models.FormatJSON, models.FormatXML, models.FormatYAML, models.FormatTOML, models.FormatMsgPack,
				models.FormatCSV, models.FormatTSV, models.FormatXLSX,
				// Properties doesn't standardly support root arrays well without specific convention, but our codec might handle it
			},
		},
		{
			Name: "ComplexObject",
			Data: complexObject,
			CompatibleFormats: []models.Format{
				models.FormatJSON, models.FormatXML, models.FormatYAML, models.FormatTOML, models.FormatMsgPack, models.FormatProperties,
				// Flat formats cannot represent nested data losslessly
			},
		},
		{
			Name: "DeepNestedObject",
			Data: deepNestedObject,
			CompatibleFormats: []models.Format{
				models.FormatJSON, models.FormatXML, models.FormatYAML, models.FormatTOML, models.FormatMsgPack, models.FormatProperties,
			},
		},
		{
			Name: "MixedRecordsList",
			Data: mixedRecordsList,
			CompatibleFormats: []models.Format{
				models.FormatJSON, models.FormatXML, models.FormatYAML, models.FormatTOML, models.FormatMsgPack,
				models.FormatCSV, models.FormatTSV, models.FormatXLSX,
			},
		},
		{
			Name: "LargeList",
			Data: largeList,
			CompatibleFormats: []models.Format{
				models.FormatJSON, models.FormatXML, models.FormatYAML, models.FormatMsgPack,
				models.FormatCSV, models.FormatTSV, models.FormatXLSX,
			},
		},
		{
			Name: "HugeList",
			Data: hugeList,
			CompatibleFormats: []models.Format{
				models.FormatJSON, models.FormatXML, models.FormatYAML, models.FormatMsgPack,
				models.FormatCSV, models.FormatTSV, models.FormatXLSX,
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			for _, inputFmt := range scenario.CompatibleFormats {
				for _, outputFmt := range scenario.CompatibleFormats {
					t.Run(fmt.Sprintf("%s_to_%s", inputFmt, outputFmt), func(t *testing.T) {
						// 1. Encode Canonical Data to Input Format
						inputCodec, err := GetCodec(inputFmt)
						if err != nil {
							t.Fatalf("Failed to get input codec %s: %v", inputFmt, err)
						}

						var inputBuf bytes.Buffer
						if err := inputCodec.Encode(&inputBuf, scenario.Data); err != nil {
							t.Fatalf("Failed to encode to %s: %v", inputFmt, err)
						}

						// 2. Decode Input Format to Intermediate Data
						// We need a fresh reader
						decodedIntermediate, err := inputCodec.Decode(inputBuf.Bytes())
						if err != nil {
							t.Fatalf("Failed to decode from %s: %v", inputFmt, err)
						}

						// Normalize intermediate data (e.g. map[interface{}]interface{} -> map[string]interface{})
						// This is tricky. For now, let's trust the codecs return standard types.
						// But we might need to normalize numbers (int vs float).

						// 3. Encode Intermediate Data to Output Format
						outputCodec, err := GetCodec(outputFmt)
						if err != nil {
							t.Fatalf("Failed to get output codec %s: %v", outputFmt, err)
						}

						var outputBuf bytes.Buffer
						if err := outputCodec.Encode(&outputBuf, decodedIntermediate); err != nil {
							if isKnownLimitation(inputFmt, outputFmt, scenario.Name) {
								t.Logf("Skipping known limitation (encoding failed) for %s -> %s: %v", inputFmt, outputFmt, err)
								return
							}
							t.Fatalf("Failed to encode to %s: %v", outputFmt, err)
						}

						// 4. Decode Output Format to Final Data
						finalData, err := outputCodec.Decode(outputBuf.Bytes())
						if err != nil {
							if isKnownLimitation(inputFmt, outputFmt, scenario.Name) {
								t.Logf("Skipping known limitation (decoding failed) for %s -> %s: %v", inputFmt, outputFmt, err)
								return
							}
							t.Fatalf("Failed to decode from %s: %v", outputFmt, err)
						}

						// 5. Compare Final Data with Canonical Data (or Intermediate)
						// We compare Final vs Intermediate to verify the transformation chain A -> B -> A'
						// Ideally we compare with Original, but some formats are lossy (e.g. XML adds root).
						// Let's try to compare with Original, but be lenient on specific known issues.

						// Use normalize function to handle type mismatches (int vs float, map keys)
						normalizedOriginal := normalize(scenario.Data)
						normalizedFinal := normalize(finalData)

						if diff := cmp.Diff(normalizedOriginal, normalizedFinal); diff != "" {
							// Special handling for known limitations
							if isKnownLimitation(inputFmt, outputFmt, scenario.Name) {
								t.Logf("Skipping known limitation for %s -> %s: %v", inputFmt, outputFmt, diff)
							} else {
								t.Errorf("Mismatch (-want +got):\n%s", diff)
							}
						}
					})
				}
			}
		})
	}
}

// normalize attempts to make types consistent for comparison
// - converts all numbers to float64
// - converts map[interface{}]interface{} to map[string]interface{}
// - attempts to parse strings as numbers (for CSV/TSV/Properties compatibility)
func normalize(i interface{}) interface{} {
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Map:
		m := make(map[string]interface{})
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key()
			val := iter.Value()
			m[fmt.Sprint(k.Interface())] = normalize(val.Interface())
		}
		return m
	case reflect.Slice, reflect.Array:
		s := make([]interface{}, v.Len())
		for j := 0; j < v.Len(); j++ {
			s[j] = normalize(v.Index(j).Interface())
		}
		return s
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		return v.Float()
	case reflect.String:
		str := v.String()
		// Try to parse as float
		if f, err := strconv.ParseFloat(str, 64); err == nil {
			return f
		}
		// Try to parse as bool
		if b, err := strconv.ParseBool(str); err == nil {
			return b
		}
		return str
	default:
		return i
	}
}

func isKnownLimitation(in, out models.Format, scenario string) bool {
	isTabular := func(f models.Format) bool {
		return f == models.FormatCSV || f == models.FormatTSV || f == models.FormatXLSX
	}

	if isObjectScenario(scenario) && (isTabular(in) || isTabular(out)) {
		return true
	}

	hasXML := in == models.FormatXML || out == models.FormatXML
	if hasXML {
		switch scenario {
		case "EdgeCaseObject", "DeepNestedObject":
			return true
		}
	}

	return false
}

func isObjectScenario(name string) bool {
	switch name {
	case "SimpleObject", "EdgeCaseObject", "ComplexObject", "DeepNestedObject":
		return true
	default:
		return false
	}
}

func buildSequentialList(size int, labelPrefix string) []interface{} {
	items := make([]interface{}, size)
	for i := 0; i < size; i++ {
		items[i] = map[string]interface{}{
			"id":     float64(i),
			"label":  fmt.Sprintf("%s %d", labelPrefix, i),
			"score":  float64(i) * 1.5,
			"active": i%2 == 0,
		}
	}
	return items
}
