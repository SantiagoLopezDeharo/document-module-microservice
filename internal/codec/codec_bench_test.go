package codec

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
)

func BenchmarkCodecConversions(b *testing.B) {
	largeListCount := getEnvInt("BENCH_LARGE_LIST_COUNT", 20000)
	blobRecordCount := getEnvInt("BENCH_BLOB_RECORDS", 200)
	blobSize := getEnvInt("BENCH_BLOB_SIZE", 256*1024)
	mediumListCount := getEnvInt("BENCH_MEDIUM_LIST_COUNT", 2000)

	largeListData := buildSequentialList(largeListCount, "Bench Item")
	hugeBlobList := buildBlobRecordList(blobRecordCount, blobSize)
	mediumListData := buildSequentialList(mediumListCount, "Medium Item")
	complexObject := buildComplexBenchmarkObject()

	scenarios := []struct {
		name    string
		data    interface{}
		formats []models.Format
	}{
		{
			name: "LargeList",
			data: largeListData,
			formats: []models.Format{
				models.FormatJSON,
				models.FormatXML,
				models.FormatYAML,
				models.FormatTOML,
				models.FormatMsgPack,
				models.FormatCSV,
				models.FormatTSV,
			},
		},
		{
			name: "ComplexObject",
			data: complexObject,
			formats: []models.Format{
				models.FormatJSON,
				models.FormatXML,
				models.FormatYAML,
				models.FormatTOML,
				models.FormatMsgPack,
				models.FormatProperties,
			},
		},
		{
			name: "HugeBlobList",
			data: hugeBlobList,
			formats: []models.Format{
				models.FormatJSON,
				models.FormatXML,
				models.FormatMsgPack,
			},
		},
		{
			name: "MediumTable",
			data: mediumListData,
			formats: []models.Format{
				models.FormatJSON,
				models.FormatCSV,
				models.FormatTSV,
				models.FormatXLSX,
			},
		},
	}

	for _, scenario := range scenarios {
		for _, inputFmt := range scenario.formats {
			for _, outputFmt := range scenario.formats {
				b.Run(fmt.Sprintf("%s/%s_to_%s", scenario.name, inputFmt, outputFmt), func(b *testing.B) {
					runConversionBenchmark(b, scenario.data, inputFmt, outputFmt)
				})
			}
		}
	}
}

func runConversionBenchmark(b *testing.B, data interface{}, inputFmt, outputFmt models.Format) {
	inputCodec, err := GetCodec(inputFmt)
	if err != nil {
		b.Fatalf("failed to get input codec %s: %v", inputFmt, err)
	}

	outputCodec, err := GetCodec(outputFmt)
	if err != nil {
		b.Fatalf("failed to get output codec %s: %v", outputFmt, err)
	}

	var probe bytes.Buffer
	if err := inputCodec.Encode(&probe, data); err != nil {
		b.Fatalf("probe encode failed for %s: %v", inputFmt, err)
	}
	bytesPerOp := probe.Len()
	probe.Reset()

	var encodeBuf bytes.Buffer
	var outputBuf bytes.Buffer

	b.SetBytes(int64(bytesPerOp))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		encodeBuf.Reset()
		if err := inputCodec.Encode(&encodeBuf, data); err != nil {
			b.Fatalf("encode failed for %s: %v", inputFmt, err)
		}

		decoded, err := inputCodec.Decode(bytes.NewReader(encodeBuf.Bytes()))
		if err != nil {
			b.Fatalf("decode failed for %s: %v", inputFmt, err)
		}

		outputBuf.Reset()
		if err := outputCodec.Encode(&outputBuf, decoded); err != nil {
			b.Fatalf("encode failed for %s: %v", outputFmt, err)
		}
	}
}

func buildBlobRecordList(count, blobSize int) []interface{} {
	if count <= 0 {
		count = 1
	}
	if blobSize <= 0 {
		blobSize = 1
	}

	blob := buildBlobString(blobSize)
	records := make([]interface{}, count)
	for i := 0; i < count; i++ {
		records[i] = map[string]interface{}{
			"id":      float64(i),
			"payload": blob,
			"label":   fmt.Sprintf("Blob %d", i),
			"active":  i%2 == 0,
		}
	}
	return records
}

func buildBlobString(size int) string {
	if size <= 0 {
		return ""
	}
	const chunk = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var sb strings.Builder
	sb.Grow(size)
	for sb.Len() < size {
		remaining := size - sb.Len()
		if remaining >= len(chunk) {
			sb.WriteString(chunk)
		} else {
			sb.WriteString(chunk[:remaining])
		}
	}
	return sb.String()
}

func buildComplexBenchmarkObject() map[string]interface{} {
	return map[string]interface{}{
		"id":      float64(42),
		"name":    "Benchmark Object",
		"enabled": true,
		"limits": map[string]interface{}{
			"cpu":    float64(750),
			"memory": float64(8192),
		},
		"endpoints": []interface{}{
			map[string]interface{}{"host": "api.internal", "port": float64(8080), "secure": true},
			map[string]interface{}{"host": "worker.internal", "port": float64(9090), "secure": false},
		},
		"metadata": map[string]interface{}{
			"owner":    "platform-team",
			"features": []interface{}{"rollout", "experiments", "audit"},
			"thresholds": map[string]interface{}{
				"warning":  float64(70),
				"critical": float64(90),
			},
		},
	}
}

func getEnvInt(key string, def int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			return parsed
		}
	}
	return def
}
