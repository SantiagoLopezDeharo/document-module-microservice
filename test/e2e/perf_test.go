//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Performance scenarios run against an optimized (non-race) build and write a
// report with latency, throughput, memory amplification and CPU/alloc
// hotspots, plus the optimization opportunities they point to. Scale the
// payloads with E2E_PERF_SCALE.

func perfScale() float64 {
	if v, err := strconv.ParseFloat(os.Getenv("E2E_PERF_SCALE"), 64); err == nil && v > 0 {
		return v
	}
	return 1
}

func scaled(n int) int {
	return max(1, int(math.Round(float64(n)*perfScale())))
}

func TestPerformance(t *testing.T) {
	if testing.Short() || image != "" {
		t.Skip("performance suite needs a local binary and is skipped in short mode")
	}
	s := startBinary(t, perfBinary)
	r := newReport(t, s)
	defer r.write()

	r.scenarioIdle()
	r.scenarioSmallRequests(32, 5*time.Second)
	r.scenarioLargeConversions(scaled(5 << 20))
	r.scenarioDocuments(16, 4*time.Second)
}

func (r *report) scenarioIdle() {
	sc := r.scenario("idle", "freshly started server")
	time.Sleep(300 * time.Millisecond)
	sc.metric("rss", float64(r.s.rssKB(r.t))/1024, "MiB")
}

func (r *report) scenarioSmallRequests(clients int, duration time.Duration) {
	name := "small-conversions"
	record := []byte(`[{"id":1,"name":"Ada Lovelace","email":"ada@example.com","active":true,"tags":["math","engines"],"balance":1234.56}]`)
	paths := []string{"/convert?input=json&output=xml", "/convert?input=json&output=csv", "/convert?input=json&output=yaml", "/convert?input=json&output=toml"}

	r.allocBaseline(name)
	stop := r.profile(name, duration)
	latencies, elapsed := hammer(r.t, clients, duration, func(c, i int) (string, []byte) {
		return paths[(c+i)%len(paths)], record
	}, r.s)
	stop()
	r.allocProfile(name)

	sc := r.scenario(name, fmt.Sprintf("%d clients sending ~120 byte JSON documents to XML, CSV, YAML and TOML for %s", clients, duration))
	sc.metric("requests per second", float64(len(latencies))/elapsed.Seconds(), "req/s")
	sc.latency(latencies)
}

func (r *report) scenarioLargeConversions(size int) {
	name := "large-conversions"
	var csv bytes.Buffer
	csv.WriteString("id,name,email,amount,country,active\n")
	for i := 0; csv.Len() < size; i++ {
		fmt.Fprintf(&csv, "%d,User %d,user%d@example.com,%d.%02d,UY,%t\n", i, i, i, i*3, i%100, i%2 == 0)
	}
	jsonBody := r.s.mustConvert(r.t, "csv", "json", csv.Bytes())

	sc := r.scenario(name, fmt.Sprintf("one %d MiB CSV dataset converted sequentially, input and output in every table format", csv.Len()>>20))
	stop := r.profile(name, 6*time.Second)
	r.allocBaseline(name)
	for _, out := range []string{"json", "xml", "csv", "yaml", "toml", "msgpack", "xlsx"} {
		before := r.totalAlloc()
		res := r.s.convert(r.t, "csv", out, csv.Bytes())
		if res.status != http.StatusOK {
			r.t.Fatalf("csv->%s: %d", out, res.status)
		}
		sc.metric("csv->"+out+" throughput", float64(csv.Len())/res.latency.Seconds()/(1<<20), "MiB/s")
		if before > 0 {
			sc.metric("csv->"+out+" allocated per input byte", float64(r.totalAlloc()-before)/float64(csv.Len()), "B")
		}
		if out == "csv" {
			continue
		}
		before = r.totalAlloc()
		back := r.s.convert(r.t, out, "json", res.body)
		if back.status != http.StatusOK {
			r.t.Fatalf("%s->json: %d", out, back.status)
		}
		sc.metric(out+"->json throughput", float64(len(res.body))/back.latency.Seconds()/(1<<20), "MiB/s")
		if before > 0 {
			sc.metric(out+"->json allocated per input byte", float64(r.totalAlloc()-before)/float64(len(res.body)), "B")
		}
	}
	_ = jsonBody
	stop()
	r.allocProfile(name)
	sc.metric("peak rss", float64(r.s.peakRSSKB(r.t))/1024, "MiB")
}

func (r *report) scenarioDocuments(clients int, duration time.Duration) {
	var pdf, docx, xlsx []string
	for p := 1; p <= 5; p++ {
		pdf = append(pdf, `{"type":"AddPage"}`, `{"type":"SetFont","family":"Arial","style":"B","size":16}`,
			fmt.Sprintf(`{"type":"Cell","w":0,"h":10,"txt":"Report page %d","ln":1}`, p),
			`{"type":"SetFont","family":"Times","size":11}`,
			`{"type":"MultiCell","w":0,"h":6,"txt":"`+strings.Repeat("Quarterly figures and commentary. ", 40)+`"}`)
	}
	for p := 1; p <= 200; p++ {
		docx = append(docx, `{"type":"AddParagraph"}`, fmt.Sprintf(`{"type":"AddText","txt":"Paragraph %d with some body text ","bold":%t,"size":11}`, p, p%5 == 0))
	}
	xlsx = append(xlsx, `{"type":"NewStyle","style_ref":"h","style":{"font":{"bold":true}}}`)
	for row := 1; row <= 200; row++ {
		for col := 'A'; col <= 'E'; col++ {
			xlsx = append(xlsx, fmt.Sprintf(`{"type":"SetCellValue","sheet":"Sheet1","axis":"%c%d","value":%d}`, col, row, row*int(col)))
		}
	}
	xlsx = append(xlsx, `{"type":"SetCellStyle","sheet":"Sheet1","h_cell":"A1","v_cell":"E1","style_ref":"h"}`)

	for kind, actions := range map[string][]string{"pdf": pdf, "docx": docx, "xlsx": xlsx} {
		name := "generate-" + kind
		body := []byte(`{"actions":[` + strings.Join(actions, ",") + `]}`)
		r.allocBaseline(name)
		stop := r.profile(name, duration)
		latencies, elapsed := hammer(r.t, clients, duration, func(int, int) (string, []byte) {
			return "/convert/" + kind, body
		}, r.s)
		stop()
		r.allocProfile(name)
		sc := r.scenario(name, fmt.Sprintf("%d clients generating a %d-action %s for %s", clients, len(actions), strings.ToUpper(kind), duration))
		sc.metric("documents per second", float64(len(latencies))/elapsed.Seconds(), "doc/s")
		sc.latency(latencies)
	}
}

func hammer(t *testing.T, clients int, duration time.Duration, next func(c, i int) (string, []byte), s *server) ([]time.Duration, time.Duration) {
	var mu sync.Mutex
	var all []time.Duration
	var wg sync.WaitGroup
	start := time.Now()
	deadline := start.Add(duration)
	for c := range clients {
		wg.Go(func() {
			var local []time.Duration
			for i := 0; time.Now().Before(deadline); i++ {
				path, body := next(c, i)
				res, err := s.try(http.MethodPost, path, nil, body)
				if err != nil || res.status != http.StatusOK {
					t.Errorf("%s: %v status %d", path, err, res.status)
					return
				}
				local = append(local, res.latency)
			}
			mu.Lock()
			all = append(all, local...)
			mu.Unlock()
		})
	}
	wg.Wait()
	return all, time.Since(start)
}

func (r *report) totalAlloc() uint64 {
	if r.s.pprofURL == "" {
		return 0
	}
	return r.s.memStats(r.t)["TotalAlloc"]
}

// peakRSSKB reads the process high-water mark where the platform exposes it.
func (s *server) peakRSSKB(t testing.TB) int64 {
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", s.cmd.Process.Pid)); err == nil {
		for line := range strings.SplitSeq(string(data), "\n") {
			if v, ok := strings.CutPrefix(line, "VmHWM:"); ok {
				kb, _ := strconv.ParseInt(strings.Fields(v)[0], 10, 64)
				return kb
			}
		}
	}
	if stats := s.memStatsOrNil(t); stats != nil {
		return int64(stats["Sys"] >> 10)
	}
	return s.rssKB(t)
}

func (s *server) memStatsOrNil(t testing.TB) map[string]uint64 {
	if s.pprofURL == "" {
		return nil
	}
	return s.memStats(t)
}
