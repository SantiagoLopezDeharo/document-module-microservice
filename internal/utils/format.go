package utils

import (
	"strings"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
)

func MimeToFormat(mime string) models.Format {
	if strings.Contains(mime, "application/json") {
		return models.FormatJSON
	}
	if strings.Contains(mime, "xml") {
		return models.FormatXML
	}
	if strings.Contains(mime, "csv") {
		return models.FormatCSV
	}
	if strings.Contains(mime, "toml") {
		return models.FormatTOML
	}
	if strings.Contains(mime, "tab-separated-values") {
		return models.FormatTSV
	}
	if strings.Contains(mime, "application/x-yaml") || strings.Contains(mime, "text/yaml") {
		return models.FormatYAML
	}
	if strings.Contains(mime, "application/msgpack") || strings.Contains(mime, "application/x-msgpack") {
		return models.FormatMsgPack
	}
	if strings.Contains(mime, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") {
		return models.FormatXLSX
	}
	if strings.Contains(mime, "text/x-java-properties") || strings.Contains(mime, "text/properties") {
		return models.FormatProperties
	}
	return ""
}

func GetContentType(f models.Format) string {
	switch f {
	case models.FormatJSON:
		return "application/json"
	case models.FormatXML:
		return "application/xml"
	case models.FormatCSV:
		return "text/csv"
	case models.FormatTSV:
		return "text/tab-separated-values"
	case models.FormatTOML:
		return "application/toml"
	case models.FormatYAML:
		return "application/x-yaml"
	case models.FormatMsgPack:
		return "application/msgpack"
	case models.FormatXLSX:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case models.FormatProperties:
		return "text/x-java-properties"
	default:
		return "text/plain"
	}
}
