# Document Module Microservice

A high-performance Go web service for transforming data payloads between various formats (JSON, XML, CSV, TSV, TOML, XLSX). Designed for heavy data processing pipelines.

Also has built in Documents (pdf, docx, excel) generation functionality that ensures a very high performance and reliability.

## 🛠️ Tech Stack

| Golang | Docker |
| ------ | ------ |
| <img height="60" src="https://raw.githubusercontent.com/marwin1991/profile-technology-icons/refs/heads/main/icons/go.png"> | <img height="60" src="https://raw.githubusercontent.com/marwin1991/profile-technology-icons/refs/heads/main/icons/docker.png"> |



## Features

- **Supported Formats**: JSON, XML, CSV, TSV, TOML, YAML, MsgPack, XLSX, Properties.
- **Concurrency**: Built on Go's efficient HTTP server, capable of handling concurrent requests.
- **Streaming**: Streams data directly from request to response to minimize memory footprint (though RAM is assumed sufficient for the payload).
- **Flexible Negotiation**: Supports format specification via Query Parameters or HTTP Headers.

## Usage

### Endpoints

#### Data Conversion
`POST /convert`

#### Document Generation
- `POST /convert/pdf`: Generates a PDF file from the JSON body.
- `POST /convert/docx`: Generates a DOCX file from the JSON body.
- `POST /convert/xlsx`: Generates an XLSX file from the JSON body.

### Specifying Formats

You can specify the input and output formats using **Query Parameters** (recommended for clarity) or **HTTP Headers**.

#### 1. Using Query Parameters

- `input`: The format of the request body.
- `output`: The desired format of the response body.

**Example:** Convert CSV to JSON

```bash
curl -X POST "http://localhost:8080/convert?input=csv&output=json" \
     --data-binary @data.csv
```

#### 2. Using Headers

- `Content-Type`: Specifies the input format.
- `Accept`: Specifies the output format.

**Example:** Convert JSON to TOML

```bash
curl -X POST "http://localhost:8080/convert" \
     -H "Content-Type: application/json" \
     -H "Accept: application/toml" \
     -d '{"key": "value"}'
```

### Supported Format Identifiers

| Format | Query Param Value | Content-Type / Accept Header |
|--------|-------------------|------------------------------|
| JSON   | `json`            | `application/json`           |
| XML    | `xml`             | `application/xml`            |
| CSV    | `csv`             | `text/csv`                   |
| TSV    | `tsv`             | `text/tab-separated-values`  |
| TOML   | `toml`            | `application/toml`           |
| YAML   | `yaml`            | `application/x-yaml`         |
| MsgPack| `msgpack`         | `application/msgpack`        |
| XLSX   | `xlsx`            | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` |
| Properties | `properties`  | `text/x-java-properties`     |

### Document Generation Examples

The service uses the following libraries for document generation. Please refer to their documentation for advanced usage and expected data structures.

- **PDF**: [go-pdf/fpdf](https://github.com/go-pdf/fpdf) - A library for generating PDF files.
- **DOCX**: [gingfrederik/docx](https://github.com/gingfrederik/docx) - A library for generating DOCX files.
- **XLSX**: [xuri/excelize](https://github.com/xuri/excelize) - A library for generating XLSX files.

**Expected JSON Structure (DOCX)**

The DOCX generation endpoint accepts a list of actions to construct the document.

```json
{
  "actions": [
    { "type": "AddParagraph" },
    { "type": "AddText", "txt": "My Title", "size": 48, "bold": true, "color": "0000FF" },
    { "type": "AddParagraph" },
    { "type": "AddText", "txt": "Some regular text.", "size": 24 }
  ]
}
```

**Supported DOCX Actions**: `AddParagraph`, `AddText`.
**Supported Text Properties**: `txt`, `size`, `color` (hex), `bold`, `italic`.

**Expected JSON Structure (XLSX)**

The XLSX generation endpoint accepts a list of actions to construct the spreadsheet.

```json
{
  "actions": [
    { "type": "NewSheet", "sheet": "Sheet2" },
    { "type": "SetActiveSheet", "sheet": "Sheet1" },
    { "type": "SetCellValue", "sheet": "Sheet1", "axis": "A1", "value": "Hello Excel" },
    { 
      "type": "NewStyle", 
      "style_ref": "headerStyle", 
      "style": { 
        "font": { "bold": true, "size": 14 }, 
        "fill": { "type": "pattern", "color": ["#CCCCCC"], "pattern": 1 } 
      } 
    },
    { "type": "SetCellStyle", "sheet": "Sheet1", "h_cell": "A1", "v_cell": "A1", "style_ref": "headerStyle" }
  ]
}
```

**Supported XLSX Actions**: `NewSheet`, `SetActiveSheet`, `SetCellValue`, `SetColWidth`, `SetRowHeight`, `MergeCell`, `NewStyle`, `SetCellStyle`.

**Generate PDF**

The PDF generation endpoint accepts a list of commands to construct the PDF dynamically.

```json
{
  "orientation": "P",
  "unit": "mm",
  "size": "A4",
  "actions": [
    {"type": "AddPage"},
    {"type": "SetFont", "family": "Arial", "style": "B", "size": 16},
    {"type": "Cell", "w": 0, "h": 10, "txt": "My Title", "ln": 1, "align": "C"},
    {"type": "SetFont", "family": "Arial", "style": "", "size": 12},
    {"type": "MultiCell", "w": 0, "h": 10, "txt": "Long content goes here...", "align": "L"}
  ]
}
```

**Supported PDF Actions**: `AddPage`, `SetFont`, `Cell`, `MultiCell`, `Ln`, `SetTextColor`, `SetDrawColor`, `SetFillColor`.

**Generate PDF**

```bash
curl -X POST "http://localhost:8080/convert/pdf" \
     -H "Content-Type: application/json" \
     -d @pdf_request.json \
     --output output.pdf
```

**Generate DOCX**

```bash
curl -X POST "http://localhost:8080/convert/docx" \
     -H "Content-Type: application/json" \
     -d @docx_request.json \
     --output output.docx
```

**Generate XLSX**

```bash
curl -X POST "http://localhost:8080/convert/xlsx" \
     -H "Content-Type: application/json" \
     -d @xlsx_request.json \
     --output output.xlsx
```

We can generate really complex documents using this, for example the following JSON generates a really complex and rich pdf document:
```json
{
  "orientation": "P",
  "unit": "mm",
  "size": "A4",
  "actions": [
    { "type": "AddPage" },
    
    { "type": "SetTextColor", "r": 200, "g": 200, "b": 200 },
    { "type": "SetFont", "family": "Arial", "style": "B", "size": 50 },
    { "type": "SetXY", "x": 80, "y": 10 },
    { "type": "Cell", "w": 0, "h": 20, "txt": "DRAFT", "align": "R" },

    { "type": "SetTextColor", "r": 0, "g": 0, "b": 0 },
    { "type": "SetXY", "x": 10, "y": 10 },

    { "type": "SetFont", "family": "Arial", "style": "B", "size": 24 },
    { "type": "Cell", "w": 0, "h": 20, "txt": "Complex PDF Report", "ln": 1, "align": "C" },
    { "type": "Ln", "h": 10 },

    { "type": "SetFont", "family": "Arial", "style": "", "size": 12 },
    { "type": "MultiCell", "w": 0, "h": 6, "txt": "This document demonstrates the capabilities of the PDF generation endpoint. It includes a watermark, a table of data, and mathematical symbols.", "align": "L" },
    { "type": "Ln", "h": 10 },

    { "type": "SetFont", "family": "Arial", "style": "B", "size": 14 },
    { "type": "Cell", "w": 0, "h": 10, "txt": "Data Table", "ln": 1 },
    { "type": "SetFont", "family": "Arial", "style": "B", "size": 12 },
    { "type": "SetFillColor", "r": 240, "g": 240, "b": 240 },
    { "type": "Cell", "w": 40, "h": 10, "txt": "ID", "border": "1", "align": "C" },
    { "type": "Cell", "w": 80, "h": 10, "txt": "Name", "border": "1", "align": "C" },
    { "type": "Cell", "w": 40, "h": 10, "txt": "Value", "border": "1", "ln": 1, "align": "C" },

    { "type": "SetFont", "family": "Arial", "style": "", "size": 12 },
    { "type": "Cell", "w": 40, "h": 10, "txt": "1", "border": "1", "align": "C" },
    { "type": "Cell", "w": 80, "h": 10, "txt": "Item Alpha", "border": "1", "align": "L" },
    { "type": "Cell", "w": 40, "h": 10, "txt": "100.50", "border": "1", "ln": 1, "align": "R" },
    { "type": "Cell", "w": 40, "h": 10, "txt": "2", "border": "1", "align": "C" },
    { "type": "Cell", "w": 80, "h": 10, "txt": "Item Beta", "border": "1", "align": "L" },
    { "type": "Cell", "w": 40, "h": 10, "txt": "250.00", "border": "1", "ln": 1, "align": "R" },
    { "type": "Ln", "h": 10 },

    { "type": "SetFont", "family": "Arial", "style": "B", "size": 14 },
    { "type": "Cell", "w": 0, "h": 10, "txt": "Mathematical Symbols", "ln": 1 },
    { "type": "SetFont", "family": "Arial", "style": "", "size": 12 },
    { "type": "Cell", "w": 0, "h": 10, "txt": "Here are some math symbols using the Symbol font:", "ln": 1 },
    
    { "type": "SetFont", "family": "Symbol", "style": "", "size": 16 },
    { "type": "Cell", "w": 0, "h": 10, "txt": "S x = lim (1 + 1/n)^n", "ln": 1 }
  ]
}
```

### Data Structure Assumptions

- **CSV/TSV**: Requires the input to be a list of objects (or a single object which will be treated as a row). The first row is treated as the header.
- **XML**: Uses a generic map structure. Lists are wrapped in `<List><Item>...</Item></List>` when converting to XML.
- **JSON/TOML/YAML/MsgPack**: Supports arbitrary objects and arrays.
- **Properties**: Flattens nested structures into dot-notation keys (e.g., `parent.child=value`).
- **XLSX**: Reads the first sheet. Assumes the first row is the header. Writes to "Sheet1".

## Running the Service

### Local Development

1. **Install Dependencies**:
   ```bash
   go mod download
   ```

2. **Run**:
   ```bash
   go run cmd/server/main.go
   ```

3. **Run Tests**:
   ```bash
   go test ./... -v
   ```

4. **Test Coverage**:
   ```bash
   go test ./... -cover
   ```

5. **Benchmarks**:
   ```bash
   cd internal/codec
   go test -bench=. -benchmem
   ```

### Docker

1. **Build and Run**:
   ```bash
   docker-compose up --build
   ```

The service will be available at `http://localhost:8080`.

## Integration Guide

This service is designed to be a component in a larger architecture.

1. **Data Pipeline**: Use this service as a transformation step. For example, fetch a CSV report, pipe it to this service to convert to JSON, and then ingest into a NoSQL database.
2. **Microservices**: Other services can offload format conversion tasks to this service to keep their core logic clean.

### Configuration

- **SERVER_ADDRESS**: Environment variable to set the server address (default: `:8080`)
  - Example: `SERVER_ADDRESS=:9000` to run on port 9000
  - Can be configured in [docker-compose.yml](docker-compose.yml)

### Testing

The service includes comprehensive unit tests covering:
- All format codecs (JSON, XML, CSV, TSV, TOML)
- Format conversion combinations
- Complex nested structures
- Edge cases (empty arrays, single objects, type conversions)
- HTTP API endpoints and error handling

**Test Coverage**: ~85% for codec layer, ~82% for API layer

### Performance Notes

- The service uses Go's standard library and optimized third-party libraries for parsing.
- It streams data where possible, but full object decoding is required for format transformation.
- Ensure the host has enough RAM to hold the unmarshaled representation of the largest expected payload.

## License

[MIT](LICENSE)
