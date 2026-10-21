package codec

import (
	"bytes"
	"testing"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
)

func BenchmarkCodecs(b *testing.B) {
	data := make([]map[string]interface{}, 100)
	for i := 0; i < 100; i++ {
		data[i] = map[string]interface{}{
			"id":          i,
			"name":        "Benchmark User",
			"description": "This is a test description for benchmarking purposes.",
			"active":      true,
			"score":       99.9,
		}
	}

	formats := []models.Format{
		models.FormatJSON,
		models.FormatXML,
		models.FormatCSV,
		models.FormatYAML,
		models.FormatMsgPack,
		models.FormatTOML,
	}

	for _, f := range formats {
		c, _ := GetCodec(f)

		b.Run("Encode_"+string(f), func(b *testing.B) {
			var buf bytes.Buffer
			// Pre-allocate buffer to avoid allocation noise if possible,
			// but we want to measure write speed too.
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf.Reset()
				if err := c.Encode(&buf, data); err != nil {
					b.Fatal(err)
				}
			}
		})

		// Prepare data for decode benchmark
		var buf bytes.Buffer
		c.Encode(&buf, data)
		inputBytes := buf.Bytes()

		b.Run("Decode_"+string(f), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := c.Decode(inputBytes); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
