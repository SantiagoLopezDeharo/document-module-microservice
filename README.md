# Document Module Microservice

A high-performance Go web service for transforming data payloads between various formats (JSON, XML, CSV, TSV, TOML, XLSX). Designed for heavy data processing pipelines.


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

### Endpoint

`POST /convert`

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
