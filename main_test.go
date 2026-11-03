package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
)

// fakeDevice serves the CrossPoint endpoints that coverfill uses from an
// in-memory SD card.
type fakeDevice struct {
	files   map[string][]byte
	uploads int
}

func (d *fakeDevice) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	switch r.URL.Path {
	case "/api/status":
		w.Write([]byte(`{"version":"2.0.0","firmwareFamily":"crosspoint","device":"xteink_x4_pro"}`))
	case "/api/files":
		var entries []entry
		seen := map[string]bool{}
		for name := range d.files {
			if !strings.HasPrefix(name, p+"/") {
				continue
			}
			first, rest, _ := strings.Cut(strings.TrimPrefix(name, p+"/"), "/")
			if !seen[first] {
				seen[first] = true
				entries = append(entries, entry{Name: first, IsDirectory: rest != ""})
			}
		}
		json.NewEncoder(w).Encode(entries)
	case "/download":
		data, ok := d.files[p]
		if !ok {
			http.Error(w, "Item not found", http.StatusNotFound)
			return
		}
		w.Write(data)
	case "/upload":
		file, header, err := r.FormFile("file")
		if err != nil || r.URL.Query().Get("overwrite") != "1" {
			http.Error(w, "bad upload", http.StatusBadRequest)
			return
		}
		data, _ := io.ReadAll(file)
		d.files[path.Join(p, header.Filename)] = data
		d.uploads++
		w.Write([]byte("File uploaded successfully: " + header.Filename))
	default:
		http.NotFound(w, r)
	}
}

func coverOf(w, h int) []byte {
	return grid(w, h, true, func(x, y int) byte { return byte(x+y) % 4 }).encode()
}

func TestRun(t *testing.T) {
	short := "/.crosspoint/epub_1/cover_legacy_v2.bmp"
	full := "/.crosspoint/epub_2/cover_original_crop.bmp"
	device := &fakeDevice{files: map[string][]byte{
		short:                               coverOf(480, 720),
		full:                                coverOf(480, 800),
		"/.crosspoint/epub_1/thumb_226.bmp": coverOf(10, 10),
		"/.crosspoint/recent.json":          []byte(`{"books":[{"title":"Wind and Truth","coverBmpPath":"/.crosspoint/epub_1/thumb_[HEIGHT].bmp"}]}`),
		"/.crosspoint/settings.json":        []byte(`{}`),
	}}
	server := httptest.NewServer(device)
	defer server.Close()
	c := &client{base: server.URL, http: server.Client()}
	opt := options{width: 480, height: 800}
	var out strings.Builder

	opt.dryRun = true
	if failed, err := run(c, opt, &out); err != nil || failed != 0 {
		t.Fatalf("dry run: failed=%d err=%v", failed, err)
	}
	if device.uploads != 0 {
		t.Fatal("dry run uploaded a file")
	}
	if !strings.Contains(out.String(), "Wind and Truth (epub_1) cover_legacy_v2.bmp: would pad 480x720 -> 480x800") {
		t.Fatalf("dry run output:\n%s", out.String())
	}

	opt.dryRun = false
	out.Reset()
	if failed, err := run(c, opt, &out); err != nil || failed != 0 {
		t.Fatalf("run: failed=%d err=%v\n%s", failed, err, out.String())
	}
	if device.uploads != 1 {
		t.Fatalf("uploads = %d, want 1 (thumbnails and full covers stay)", device.uploads)
	}
	img, err := parseBMP(device.files[short])
	if err != nil || img.width != 480 || img.height != 800 {
		t.Fatalf("uploaded cover: %v %+v", err, img)
	}
	if !strings.Contains(out.String(), "1 padded, 1 already full screen, 0 skipped, 0 failed") {
		t.Fatalf("run output:\n%s", out.String())
	}

	// A second run finds nothing to do.
	out.Reset()
	run(c, opt, &out)
	if device.uploads != 1 || !strings.Contains(out.String(), "0 padded, 2 already full screen") {
		t.Fatalf("second run uploaded again:\n%s", out.String())
	}
}
