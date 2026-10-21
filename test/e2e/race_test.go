//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type job struct {
	name   string
	method string
	path   string
	body   []byte
	check  func(response) error
}

// jobs returns a request for client c, iteration i whose response must carry
// a marker unique to that request, so any cross-talk between concurrent
// requests is detected.
func jobs(c, i int) []job {
	marker := fmt.Sprintf("client%dreq%d", c, i)
	contains := func(needle string) func(response) error {
		return func(r response) error {
			if r.status != http.StatusOK {
				return fmt.Errorf("status %d: %.200s", r.status, r.body)
			}
			if !bytes.Contains(r.body, []byte(needle)) {
				return fmt.Errorf("response lacks its marker %s: %.200s", needle, r.body)
			}
			return nil
		}
	}
	ok := func(magic string) func(response) error {
		return func(r response) error {
			if r.status != http.StatusOK || !bytes.HasPrefix(r.body, []byte(magic)) {
				return fmt.Errorf("status %d body %.40q", r.status, r.body)
			}
			return nil
		}
	}
	record := fmt.Sprintf(`[{"marker":"%s","n":%d},{"marker":"%s","n":%d}]`, marker, i, marker, i+1)
	return []job{
		{"json->xml", http.MethodPost, "/convert?input=json&output=xml", []byte(record), contains("<marker>" + marker + "</marker>")},
		{"json->csv", http.MethodPost, "/convert?input=json&output=csv", []byte(record), contains(marker + "," + fmt.Sprint(i))},
		{"json->yaml", http.MethodPost, "/convert?input=json&output=yaml", []byte(record), contains("marker: " + marker)},
		{"json->toml", http.MethodPost, "/convert?input=json&output=toml", []byte(record), contains(`marker = '` + marker + `'`)},
		{"csv->json", http.MethodPost, "/convert?input=csv&output=json", []byte("marker,n\n" + marker + ",1\n"), contains(`"marker": "` + marker + `"`)},
		{"xml->json", http.MethodPost, "/convert?input=xml&output=json", []byte("<doc><marker>" + marker + "</marker></doc>"), contains(marker)},
		{"properties->json", http.MethodPost, "/convert?input=properties&output=json", []byte("a.marker=" + marker), contains(marker)},
		{"json->properties", http.MethodPost, "/convert?input=json&output=properties", []byte(`{"m":"` + marker + `"}`), contains("m=" + marker)},
		{"json->xlsx", http.MethodPost, "/convert?input=json&output=xlsx", []byte(record), ok("PK")},
		{"pdf", http.MethodPost, "/convert/pdf", []byte(`{"actions":[{"type":"AddPage"},{"type":"SetFont","family":"Arial","size":12},{"type":"Cell","w":40,"h":10,"txt":"` + marker + `"}]}`), ok("%PDF")},
		{"docx", http.MethodPost, "/convert/docx", []byte(`{"actions":[{"type":"AddText","txt":"` + marker + `"}]}`), ok("PK")},
		{"xlsx", http.MethodPost, "/convert/xlsx", []byte(`{"actions":[{"type":"SetCellValue","sheet":"Sheet1","axis":"A1","value":"` + marker + `"}]}`), ok("PK")},
	}
}

func TestConcurrentClientsGetTheirOwnResults(t *testing.T) {
	s := startServer(t)
	const clients, rounds = 24, 4
	var wg sync.WaitGroup
	var failures atomic.Int64
	for c := range clients {
		wg.Go(func() {
			for i := range rounds {
				for _, j := range jobs(c, i) {
					res, err := s.try(j.method, j.path, nil, j.body)
					if err == nil {
						err = j.check(res)
					}
					if err != nil {
						failures.Add(1)
						t.Errorf("client %d %s: %v", c, j.name, err)
					}
				}
			}
		})
	}
	wg.Wait()
	t.Logf("%d requests across %d concurrent clients, %d failures", clients*rounds*len(jobs(0, 0)), clients, failures.Load())
}

func TestIdenticalDocumentsRenderIdenticallyInParallel(t *testing.T) {
	s := startServer(t)
	var actions []string
	for r := 1; r <= 200; r++ {
		actions = append(actions, fmt.Sprintf(`{"type":"AddParagraph"},{"type":"AddText","txt":"line %d","bold":%t}`, r, r%2 == 0))
	}
	body := `{"actions":[` + strings.Join(actions, ",") + `]}`
	reference := s.generate(t, "docx", body)
	want, err := readZipEntry(reference.body, "word/document.xml")
	if err != nil {
		t.Fatalf("status %d: %v", reference.status, err)
	}
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			res, err := s.try(http.MethodPost, "/convert/docx", nil, []byte(body))
			if err != nil {
				t.Error(err)
				return
			}
			if got, err := readZipEntry(res.body, "word/document.xml"); err != nil || !bytes.Equal(got, want) {
				t.Errorf("concurrent render differs: %v", err)
			}
		})
	}
	wg.Wait()
}

func TestConcurrencyLimitQueuesRequests(t *testing.T) {
	s := startServer(t, withEnv("MAX_CONCURRENT_REQUESTS", "1"))
	var wg sync.WaitGroup
	var failures atomic.Int64
	for i := range 40 {
		wg.Go(func() {
			j := jobs(i, 0)[i%len(jobs(0, 0))]
			res, err := s.try(j.method, j.path, nil, j.body)
			if err == nil {
				err = j.check(res)
			}
			if err != nil {
				failures.Add(1)
				t.Errorf("%s: %v", j.name, err)
			}
		})
	}
	wg.Wait()
}

func TestShutdownUnderLoad(t *testing.T) {
	s := startServer(t)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	var ok, refused, broken atomic.Int64
	for c := range 16 {
		wg.Go(func() {
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				j := jobs(c, i)[i%len(jobs(0, 0))]
				res, err := s.try(j.method, j.path, nil, j.body)
				switch {
				case err != nil:
					refused.Add(1)
					time.Sleep(10 * time.Millisecond)
				case j.check(res) == nil:
					ok.Add(1)
				default:
					broken.Add(1)
					t.Errorf("%s during shutdown: %v", j.name, j.check(res))
				}
			}
		})
	}
	time.Sleep(time.Second)
	if err := s.stop(60 * time.Second); err != nil {
		t.Errorf("exit under load: %v", err)
	}
	close(stop)
	wg.Wait()
	t.Logf("completed %d, refused after shutdown %d, broken %d", ok.Load(), refused.Load(), broken.Load())
	if ok.Load() == 0 {
		t.Error("no request completed before shutdown")
	}
}
