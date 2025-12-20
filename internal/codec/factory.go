package codec

import (
	"fmt"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
)

func GetCodec(format models.Format) (Codec, error) {
	switch format {
	case models.FormatJSON:
		return &jsonCodec{}, nil
	case models.FormatXML:
		return &xmlCodec{}, nil
	case models.FormatTOML:
		return &tomlCodec{}, nil
	case models.FormatCSV:
		return &csvCodec{comma: ','}, nil
	case models.FormatTSV:
		return &csvCodec{comma: '\t'}, nil
	case models.FormatYAML:
		return &yamlCodec{}, nil
	case models.FormatMsgPack:
		return &msgpackCodec{}, nil
	case models.FormatProperties:
		return &propertiesCodec{}, nil
	case models.FormatXLSX:
		return &xlsxCodec{}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}
