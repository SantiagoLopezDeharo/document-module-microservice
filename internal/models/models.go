package models

type Format string

const (
	FormatJSON       Format = "JSON"
	FormatXML        Format = "XML"
	FormatCSV        Format = "CSV"
	FormatTSV        Format = "TSV"
	FormatTOML       Format = "TOML"
	FormatYAML       Format = "YAML"
	FormatMsgPack    Format = "MSGPACK"
	FormatXLSX       Format = "XLSX"
	FormatProperties Format = "PROPERTIES"
)

// RequestPayload represents the structure if sent as a JSON envelope (optional support)
type RequestEnvelope struct {
	InputFormat  Format      `json:"input_format"`
	OutputFormat Format      `json:"output_format"`
	Data         interface{} `json:"data"`
}

// We will primarily use a generic interface for the internal data representation
// It will be either map[string]interface{} or []map[string]interface{}
