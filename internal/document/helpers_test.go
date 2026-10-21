package document

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

var testLimits = Limits{MaxActions: 100, MaxXLSXCells: 10_000}

func mustJSON[T any](t testing.TB, raw string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return v
}

func assertActionError(t *testing.T, err error, index int, typ string) {
	t.Helper()
	var ae *ActionError
	if !errors.As(err, &ae) {
		t.Fatalf("error %v is not an ActionError", err)
	}
	if ae.Index != index || ae.Type != typ {
		t.Errorf("ActionError = %d/%s, want %d/%s (%v)", ae.Index, ae.Type, index, typ, err)
	}
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("ActionError %v does not match ErrInvalid", err)
	}
}

func zipEntry(t *testing.T, archive []byte, name string) []byte {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("output is not a zip archive: %v", err)
	}
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
	}
	t.Fatalf("archive has no %s", name)
	return nil
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file missing (run with -update): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from golden file:\n got: %s\nwant: %s", name, got, want)
	}
}

var pdfStream = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)

// pdfText inflates every content stream so tests can look for rendered text.
func pdfText(t *testing.T, pdf []byte) string {
	t.Helper()
	var out bytes.Buffer
	for _, m := range pdfStream.FindAllSubmatch(pdf, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			out.Write(m[1])
			continue
		}
		_, _ = io.Copy(&out, r)
	}
	return out.String()
}

var pdfPage = regexp.MustCompile(`/Type /Page\b[^s]`)
