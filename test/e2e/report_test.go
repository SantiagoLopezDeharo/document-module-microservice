//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

type report struct {
	t         *testing.T
	s         *server
	dir       string
	label     string
	Scenarios []*scenarioResult `json:"scenarios"`
	hotspots  map[string][]hotspot
	allocs    map[string][]hotspot
	heap      map[string][]hotspot
}

type scenarioResult struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Metrics     []metric `json:"metrics"`
}

type metric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type hotspot struct {
	Flat     float64
	Cum      float64
	Function string
}

type sample struct {
	rssKB      int64
	heapInuse  uint64
	goroutines int
}

func newReport(t *testing.T, s *server) *report {
	dir := os.Getenv("E2E_REPORT_DIR")
	if dir == "" {
		dir = filepath.Join(moduleRoot(), "test", "e2e", "reports")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	label := os.Getenv("E2E_PERF_LABEL")
	if label == "" {
		label = "current"
	}
	return &report{t: t, s: s, dir: dir, label: label, hotspots: map[string][]hotspot{}, allocs: map[string][]hotspot{}, heap: map[string][]hotspot{}}
}

func (r *report) sample() sample {
	smp := sample{rssKB: r.s.rssKB(r.t)}
	if r.s.pprofURL != "" {
		smp.heapInuse = r.s.memStats(r.t)["HeapInuse"]
		smp.goroutines = r.s.goroutines(r.t)
	}
	return smp
}

func (r *report) scenario(name, description string) *scenarioResult {
	sc := &scenarioResult{Name: name, Description: description}
	r.Scenarios = append(r.Scenarios, sc)
	return sc
}

func (sc *scenarioResult) metric(name string, value float64, unit string) {
	sc.Metrics = append(sc.Metrics, metric{name, value, unit})
}

func (sc *scenarioResult) distribution(name string, values []time.Duration) {
	if len(values) == 0 {
		return
	}
	slices.Sort(values)
	pick := func(p float64) float64 {
		idx := min(len(values)-1, int(p*float64(len(values))))
		return float64(values[idx].Microseconds()) / 1000
	}
	sc.metric(name+" p50", pick(0.50), "ms")
	sc.metric(name+" p95", pick(0.95), "ms")
	sc.metric(name+" p99", pick(0.99), "ms")
	sc.metric(name+" max", float64(values[len(values)-1].Microseconds())/1000, "ms")
}

func (sc *scenarioResult) latency(values []time.Duration) {
	sc.distribution("latency", values)
}

func (sc *scenarioResult) value(name string) (float64, bool) {
	for _, m := range sc.Metrics {
		if m.Name == name {
			return m.Value, true
		}
	}
	return 0, false
}

// profile captures a CPU profile of the server while a scenario runs. The
// returned function blocks until the profile is written and summarized.
func (r *report) profile(name string, duration time.Duration) func() {
	if r.s.pprofURL == "" {
		return func() {}
	}
	seconds := max(1, int(duration.Round(time.Second).Seconds()))
	done := make(chan struct{})
	go func() {
		defer close(done)
		path := filepath.Join(r.dir, fmt.Sprintf("%s-cpu-%s.pb.gz", r.label, name))
		if err := r.download(fmt.Sprintf("/debug/pprof/profile?seconds=%d", seconds), path); err == nil {
			r.hotspots[name] = topFunctions(path, "")
		}
	}()
	return func() { <-done }
}

func (r *report) allocBaseline(name string) {
	if r.s.pprofURL != "" {
		_ = r.download("/debug/pprof/allocs", filepath.Join(r.dir, fmt.Sprintf("%s-allocs-%s-before.pb.gz", r.label, name)))
	}
}

func (r *report) allocProfile(name string) {
	if r.s.pprofURL == "" {
		return
	}
	before := filepath.Join(r.dir, fmt.Sprintf("%s-allocs-%s-before.pb.gz", r.label, name))
	after := filepath.Join(r.dir, fmt.Sprintf("%s-allocs-%s.pb.gz", r.label, name))
	if r.download("/debug/pprof/allocs", after) == nil {
		r.allocs[name] = topFunctions(after, before, "-sample_index=alloc_space")
	}
}

func (r *report) heapProfile(name string) {
	if r.s.pprofURL == "" {
		return
	}
	path := filepath.Join(r.dir, fmt.Sprintf("%s-heap-%s.pb.gz", r.label, name))
	if r.download("/debug/pprof/heap?gc=1", path) == nil {
		r.heap[name] = topFunctions(path, "", "-sample_index=inuse_space")
	}
}

func (r *report) download(path, dest string) error {
	resp, err := httpClient.Get(r.s.pprofURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

var topLine = regexp.MustCompile(`^\s*[\d.]+\w*\s+([\d.]+)%\s+[\d.]+%\s+[\d.]+\w*\s+([\d.]+)%\s+(.+)$`)

func topFunctions(profile, base string, extra ...string) []hotspot {
	args := []string{"tool", "pprof", "-top", "-nodecount=25"}
	args = append(args, extra...)
	if base != "" {
		args = append(args, "-base="+base)
	}
	args = append(args, profile)
	out, err := exec.Command("go", args...).Output()
	if err != nil {
		return nil
	}
	var spots []hotspot
	for line := range strings.SplitSeq(string(out), "\n") {
		m := topLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		flat, _ := strconv.ParseFloat(m[1], 64)
		cum, _ := strconv.ParseFloat(m[2], 64)
		spots = append(spots, hotspot{Flat: flat, Cum: cum, Function: strings.TrimSpace(m[3])})
	}
	return spots
}

type opportunity struct {
	Area     string
	Evidence string
	Advice   string
}

type hotspotRule struct {
	pattern *regexp.Regexp
	area    string
	advice  string
}

var cpuRules = []hotspotRule{
	{regexp.MustCompile(`gopkg\.in/yaml\.v3`), "YAML codec",
		"yaml.v3 builds a node tree and emits through reflection for every value. Encoding plain maps and slices with a hand-rolled emitter, or a faster YAML library, would cut it."},
	{regexp.MustCompile(`excelize.*\(\*File\)\.(SetCellValue|SetCellStr|SetCellDefault|prepareSheetXML|setCellStr)|excelize.*\(\*xlsxWorksheet\)`), "XLSX cell writes",
		"Cell-by-cell writes into excelize's in-memory worksheet dominate. excelize.StreamWriter writes rows sequentially with far less memory and CPU."},
	{regexp.MustCompile(`compress/flate`), "compression",
		"DEFLATE dominates. PDF streams and XLSX/DOCX parts are compressed per object; a faster compression level or reusing compressors would help."},
	{regexp.MustCompile(`runtime\.(mallocgc|gcBgMarkWorker|scanobject|gcDrain|memclrNoHeapPointers|scanObjectsSmall|memmove)`), "allocation and GC",
		"Allocation, copying and GC work are significant. Avoid copying request bodies more than once, pre-size output buffers and reduce per-value allocations."},
	{regexp.MustCompile(`clbanning/mxj`), "XML codec",
		"mxj converts through generic maps with reflection and string building per element."},
	{regexp.MustCompile(`encoding/json`), "JSON codec",
		"JSON encoding/decoding is significant; indentation and number normalization add passes over the data."},
	{regexp.MustCompile(`fmt\.(Sprintf|Sprint|Fprintf)|fmt\.\(\*pp\)`), "fmt formatting",
		"fmt formatting on the per-cell path (header flattening, value stringification) is measurable; strconv and type switches are much cheaper."},
	{regexp.MustCompile(`sort\.|slices\.Sort`), "sorting",
		"Sorting keys per row or per document shows up; sort once and reuse."},
	{regexp.MustCompile(`syscall\.(syscall|write|rawsyscalln?)|internal/poll`), "syscalls",
		"Kernel IO is significant; responses could be written with fewer, larger writes."},
	{regexp.MustCompile(`regexp\.`), "regular expressions",
		"Regular expression matching on the request path is measurable; simple byte checks would do."},
}

func (r *report) opportunities() []opportunity {
	var out []opportunity
	seen := map[string]bool{}
	for _, name := range r.scenarioNames() {
		for _, spot := range r.hotspots[name] {
			for _, rule := range cpuRules {
				key := rule.area + name
				if spot.Flat >= 3 && rule.pattern.MatchString(spot.Function) && !seen[key] {
					seen[key] = true
					out = append(out, opportunity{rule.area, fmt.Sprintf("%s: `%s` %.1f%% flat CPU", name, spot.Function, spot.Flat), rule.advice})
				}
			}
		}
		for _, spot := range r.allocs[name][:min(3, len(r.allocs[name]))] {
			if spot.Flat >= 15 && !seen["alloc"+spot.Function] {
				seen["alloc"+spot.Function] = true
				out = append(out, opportunity{"allocations", fmt.Sprintf("%s: `%s` allocates %.1f%% of bytes", name, spot.Function, spot.Flat),
					"A single call site dominates allocated bytes. Pool or reuse its buffers."})
			}
		}
	}

	for _, sc := range r.Scenarios {
		for _, m := range sc.Metrics {
			if strings.HasSuffix(m.Name, "allocated per input byte") && m.Value > 50 {
				out = append(out, opportunity{"memory amplification", fmt.Sprintf("%s: %s %.0f bytes", sc.Name, m.Name, m.Value),
					"The conversion allocates far more than its input. Streaming the encoder or avoiding intermediate representations would cut GC pressure and peak memory."})
			}
		}
		p50, ok50 := sc.value("latency p50")
		p99, ok99 := sc.value("latency p99")
		if ok50 && ok99 && p50 > 0 && p99/p50 > 10 {
			out = append(out, opportunity{"tail latency", fmt.Sprintf("%s: p99 %.2f ms is %.0f× p50 %.2f ms", sc.Name, p99, p99/p50, p50),
				"Tail latency is far from the median. Look for GC pauses or requests queueing behind large conversions."})
		}
	}
	return out
}

func (r *report) scenarioNames() []string {
	names := make([]string, len(r.Scenarios))
	for i, sc := range r.Scenarios {
		names[i] = sc.Name
	}
	return names
}

func (r *report) baseline() map[string]map[string]float64 {
	path := os.Getenv("E2E_PERF_BASELINE")
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		r.t.Logf("baseline %s: %v", path, err)
		return nil
	}
	var base report
	if err := json.Unmarshal(data, &base); err != nil {
		r.t.Logf("baseline %s: %v", path, err)
		return nil
	}
	values := map[string]map[string]float64{}
	for _, sc := range base.Scenarios {
		values[sc.Name] = map[string]float64{}
		for _, m := range sc.Metrics {
			values[sc.Name][m.Name] = m.Value
		}
	}
	return values
}

func (r *report) write() {
	if len(r.Scenarios) == 0 {
		return
	}
	base := r.baseline()
	var md bytes.Buffer
	fmt.Fprintf(&md, "# Performance report: %s\n\n", r.label)
	fmt.Fprintf(&md, "Generated %s on %s/%s with %d CPUs (%s). Load generator and server share the host, so absolute numbers are a lower bound.\n\n",
		time.Now().Format(time.RFC3339), runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version())

	for _, sc := range r.Scenarios {
		fmt.Fprintf(&md, "## %s\n\n%s\n\n", sc.Name, sc.Description)
		if base != nil {
			md.WriteString("| metric | value | baseline | change |\n|---|---:|---:|---:|\n")
		} else {
			md.WriteString("| metric | value |\n|---|---:|\n")
		}
		for _, m := range sc.Metrics {
			fmt.Fprintf(&md, "| %s | %s %s |", m.Name, formatValue(m.Value), m.Unit)
			if base != nil {
				if b, ok := base[sc.Name][m.Name]; ok && b != 0 {
					fmt.Fprintf(&md, " %s %s | %+.1f%% |", formatValue(b), m.Unit, (m.Value-b)/b*100)
				} else {
					md.WriteString(" – | – |")
				}
			}
			md.WriteString("\n")
		}
		writeHotspots(&md, "CPU hotspots (flat %)", r.hotspots[sc.Name])
		writeHotspots(&md, "Allocation hotspots (bytes, flat %)", r.allocs[sc.Name])
		writeHotspots(&md, "Heap in use (bytes, flat %)", r.heap[sc.Name])
		md.WriteString("\n")
	}

	md.WriteString("## Optimization opportunities\n\n")
	opps := r.opportunities()
	if len(opps) == 0 {
		md.WriteString("No hotspot crossed the reporting thresholds.\n")
	}
	for _, o := range opps {
		fmt.Fprintf(&md, "- **%s** — %s. %s\n", o.Area, o.Evidence, o.Advice)
	}

	mdPath := filepath.Join(r.dir, "perf-"+r.label+".md")
	jsonPath := filepath.Join(r.dir, "perf-"+r.label+".json")
	data, _ := json.MarshalIndent(r, "", "  ")
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		r.t.Errorf("write %s: %v", jsonPath, err)
	}
	if err := os.WriteFile(mdPath, md.Bytes(), 0o644); err != nil {
		r.t.Errorf("write %s: %v", mdPath, err)
	}
	r.t.Logf("performance report written to %s\n%s", mdPath, md.String())
}

func writeHotspots(md *bytes.Buffer, title string, spots []hotspot) {
	if len(spots) == 0 {
		return
	}
	fmt.Fprintf(md, "\n%s:\n\n", title)
	for _, s := range spots[:min(8, len(spots))] {
		fmt.Fprintf(md, "- %.1f%% (cum %.1f%%) `%s`\n", s.Flat, s.Cum, s.Function)
	}
}

func formatValue(v float64) string {
	switch {
	case v >= 100:
		return strconv.FormatFloat(v, 'f', 0, 64)
	case v >= 1:
		return strconv.FormatFloat(v, 'f', 2, 64)
	default:
		return strconv.FormatFloat(v, 'f', 3, 64)
	}
}
