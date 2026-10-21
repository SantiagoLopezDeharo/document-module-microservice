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

// We will primarily use a generic interface for the internal data representation
// It will be either map[string]interface{} or []map[string]interface{}
