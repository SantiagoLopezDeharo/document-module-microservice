# Document Module Microservice

A high-performance Go web service for transforming data payloads between various formats (JSON, XML, CSV, TSV, TOML, XLSX). Designed for heavy data processing pipelines.

Also has built in Documents (pdf, docx, excel) generation functionality that ensures a very high performance and reliability.

## 🛠️ Tech Stack

| Golang | Docker |
| ------ | ------ |
| <img height="60" src="https://raw.githubusercontent.com/marwin1991/profile-technology-icons/refs/heads/main/icons/go.png"> | <img height="60" src="https://raw.githubusercontent.com/marwin1991/profile-technology-icons/refs/heads/main/icons/docker.png"> |



## Features

- **Supported Formats**: JSON, XML, CSV, TSV, TOML, YAML, MsgPack, XLSX, Properties.
- **Concurrency**: Built on Go's HTTP server; a configurable number of conversions run in parallel and the rest queue, so memory stays bounded no matter how many clients connect.
- **Hardened input handling**: body size, nesting depth, document actions and worksheet size are all bounded; malformed or hostile documents get a 4xx instead of crashing the process.
- **Faithful conversions**: XML output is escaped, JSON integers beyond 2^53 keep every digit, `${...}` in `.properties` files is kept literally.
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
**Supported Text Properties**: `txt`, `size` (1–1638 pt), `color` (6-digit hex, optionally prefixed with `#`, or `auto`), `bold`, `italic`.

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

**Supported PDF Actions**: `AddPage`, `SetFont`, `Cell`, `MultiCell`, `Ln`, `SetXY`, `SetTextColor`, `SetDrawColor`, `SetFillColor`. Fonts are the PDF core fonts (Arial/Helvetica, Times, Courier, ZapfDingbats; fpdf treats `Symbol` as an alias of ZapfDingbats).

Unknown action types, invalid parameters (unknown fonts or page sizes, malformed cell references, unknown style references) and actions that fail are rejected with `400` and a message naming the offending action, e.g. `action 3 (SetCellValue): ...`, instead of producing a partial document.

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
- **XML**: Uses a generic map structure under a `<doc>` root. Top-level lists become repeated `<List>` elements, keys prefixed with `-` are attributes and `#text` is element text. Keys must be valid XML names.
- **JSON/TOML/YAML/MsgPack**: Supports arbitrary objects and arrays.
- **Properties**: Flattens nested structures into dot-notation keys (e.g., `parent.child=value`).
- **XLSX**: Reads the first sheet. Assumes the first row is the header. Writes to "Sheet1".

### Responses and errors

| Status | Meaning |
|--------|---------|
| `200` | Converted or generated document |
| `400` | Malformed input, unsupported format, invalid document action |
| `405` | Wrong HTTP method |
| `413` | Body, nesting depth, action count or worksheet size over the configured limits |
| `422` | Valid input that the output format cannot represent (e.g. a list of numbers as CSV, a key that is not a valid XML name, an integer above `int64` as TOML) |
| `500` | Unexpected failure; details are logged, not returned |

## Running the Service

### Local Development

Requires Go 1.27.3 or newer.

```bash
go run ./cmd/server
```

### Docker

A ~18 MB multi-arch (`linux/amd64`, `linux/arm64`) image built from `scratch` and running as a non-root user is published on Docker Hub:

```bash
docker run --rm -p 8080:8080 slincsilver/document-module-microservice:1.0.1
```

Or build locally with `docker-compose up --build`. The service shuts down gracefully on `SIGTERM`, letting in-flight conversions finish.

### Image security and supply chain

- **Minimal image**: `scratch` base containing only the static binary and a writable `/tmp` for streamed XLSX files, running as UID `65532`. No shell, package manager or libc to exploit.
- **Pinned build**: the Go builder image is pinned by digest and module checksums are verified against `go.sum`; the build context is an allowlist of sources only.
- **Reproducible**: layer timestamps are set to the last commit time (`SOURCE_DATE_EPOCH`), so rebuilding a commit yields a bit-identical image digest.
- **Scanned before publishing**: `make docker-push` runs `make scan` first and refuses to publish if `govulncheck` finds a vulnerability reachable from the code or Docker Scout finds a fixable critical/high CVE in the image.
- **Attested**: every release carries an SBOM and SLSA provenance (max mode):
  ```shell
  docker buildx imagetools inspect slincsilver/document-module-microservice:1.0.1 --format '{{json .SBOM}}'
  docker buildx imagetools inspect slincsilver/document-module-microservice:1.0.1 --format '{{json .Provenance}}'
  ```
- **Signed**: releases are signed keyless with [Sigstore cosign](https://docs.sigstore.dev/); verify before deploying:
  ```shell
  cosign verify slincsilver/document-module-microservice:1.0.1 \
    --certificate-identity=santiagolhgpro@gmail.com \
    --certificate-oidc-issuer=https://accounts.google.com
  ```
- **Run hardened**: `docker-compose.yml` runs the service with a read-only root filesystem, all Linux capabilities dropped, `no-new-privileges` and memory/CPU/PID limits. The equivalent `docker run` flags are `--read-only --cap-drop=ALL --security-opt=no-new-privileges --tmpfs /tmp`; `make e2e-image` runs the test suites against the container with these restrictions.

## Integration Guide

This service is designed to be a component in a larger architecture.

1. **Data Pipeline**: Use this service as a transformation step. For example, fetch a CSV report, pipe it to this service to convert to JSON, and then ingest into a NoSQL database.
2. **Microservices**: Other services can offload format conversion tasks to this service to keep their core logic clean.

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_ADDRESS` | `:8080` | Listen address |
| `MAX_BODY_BYTES` | `33554432` (32 MiB) | Largest accepted request body |
| `MAX_ACTIONS` | `10000` | Most actions per PDF/DOCX/XLSX document |
| `MAX_XLSX_CELLS` | `200000` | Worksheet budget for XLSX generation (rows plus cells up to the furthest column used in each row) |
| `MAX_CONCURRENT_REQUESTS` | `2 × GOMAXPROCS` | Conversions processed at once; further requests wait. Bodies are received before a slot is taken, so slow uploads cannot hold slots |
| `PPROF_ADDR` | – (off) | Address for an optional `net/http/pprof` listener, e.g. `127.0.0.1:6060`. Never expose it publicly |

Nesting is limited to 10 000 levels in every format, and uploaded workbooks may inflate to at most 256 MiB. Invalid configuration stops the service at startup with every problem reported at once.

## 🧪 Testing

```bash
make test        # unit tests with the race detector
make e2e         # end-to-end suites against a race-detector build
make e2e-image   # functional and security suites against the Docker image
make perf        # performance scenarios and optimization report
make bench       # micro benchmarks
```

- **Unit tests** cover every package: all codecs and their nesting guards, JSON integer precision, fuzz targets for every decoder and document renderer, differential fuzzing of the YAML emitter, JSON encoder and XML depth scanner against their reference implementations, PDF text extraction, a golden DOCX `document.xml`, XLSX workbooks reopened with excelize, the worksheet budget, and every API status code.
- **End-to-end tests** (`test/e2e`, build tag `e2e`) build the real binary with the race detector and coverage, boot it with a throwaway configuration and drive it over HTTP:
  - *functional*: every format pair round-tripping a dataset of tricky strings, nested documents, 64-bit integers, header negotiation, 20 MiB payloads, generated PDF/DOCX/XLSX, graceful shutdown with in-flight requests, startup validation;
  - *security*: nesting, MessagePack length, YAML alias, XML entity and XLSX zip bombs, XXE, XML output injection, worksheet amplification, document injection, body/header limits, slow uploads and slowloris, leak-free error bodies, connection floods and memory that stays bounded as clients grow, each asserting the process survives;
  - *concurrency*: two dozen clients on every endpoint with per-request markers to detect cross-talk, parallel identical renders, queueing at the concurrency limit, shutdown under load. Any data race, panic or unclean exit fails the run.
- **Performance suite** profiles the live server through pprof and writes `test/e2e/reports/perf-<label>.md` with latency percentiles, throughput, bytes allocated per input byte, peak memory, CPU and allocation hotspots and the optimization opportunities they point to. `E2E_BINARY` profiles another build and `E2E_PERF_BASELINE` adds a comparison column.

## ⚡ Performance

Release builds use profile-guided optimization: `cmd/server/default.pgo` merges the CPU profiles the performance suite captures from the live server, and `make pgo` refreshes it.

Driven by the performance suite's findings; median of three runs on an Apple M3 Pro with the load generator on the same host, against the previous release of the service:

| Scenario | Before | After |
|----------|-------:|------:|
| Small conversions, 32 clients | 61.2k req/s, p99 2.42 ms | 84.8k req/s, p99 1.68 ms |
| CSV → YAML (5 MiB) | 8.0 MiB/s | 20.7 MiB/s |
| CSV → XLSX (5 MiB) | 4.6 MiB/s | 16.5 MiB/s |
| CSV → MessagePack (5 MiB) | 45.3 MiB/s | 96.8 MiB/s |
| TOML → JSON | 55.0 MiB/s | 88.2 MiB/s |
| CSV → XML (5 MiB) | 40.1 MiB/s | 33.2 MiB/s |
| Peak memory, large-conversion scenario | 1353 MiB | 368 MiB |
| DOCX generation, 16 clients | 1.7k doc/s | 7.3k doc/s |
| PDF generation, 16 clients | 2.3k doc/s | 2.5k doc/s |

Main changes: a streaming YAML emitter (yaml.v3 keeps every event of a document queued, about 400 bytes per input byte), excelize's stream writer for XLSX output, JSON indented in a single pass, an allocation-free XML depth check, request bodies read once and pooled output buffers. CSV → XML is slower than before because output is now escaped and validated (the previous version produced malformed XML for `&` and allowed markup injection) and responses are buffered so failures return a proper status instead of a truncated `200`. The remaining hotspots are inside dependencies: fpdf creates a new zlib writer per PDF stream and XLSX decoding is bound by excelize's XML parsing.

## License

This project is licensed under the [MIT License](LICENSE). The vendored `internal/docx` package keeps its original MIT license.
