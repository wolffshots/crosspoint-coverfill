// Command coverfill makes CrossPoint sleep covers fill the screen. CrossPoint
// caches each book's sleep cover as a BMP under /.crosspoint/<book cache>/.
// In Fit mode that BMP is narrower or shorter than the screen, so white bars
// show around it. coverfill fetches each cached cover over the File Transfer
// web server, pads it to the screen size with mirrored edges and uploads it in
// place. CrossPoint only checks that the cached file exists, so it draws the
// padded one from then on.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const cacheRoot = "/.crosspoint"

// version is overridden at release build time via -ldflags "-X main.version=...".
var version = "dev"

type client struct {
	base string
	http *http.Client
}

func (c *client) get(p string, q url.Values) ([]byte, error) {
	resp, err := c.http.Get(c.base + p + "?" + q.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s %s: %s: %s", p, q.Get("path"), resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func (c *client) download(file string) ([]byte, error) {
	return c.get("/download", url.Values{"path": {file}})
}

type entry struct {
	Name        string `json:"name"`
	IsDirectory bool   `json:"isDirectory"`
}

func (c *client) list(dir string) ([]entry, error) {
	body, err := c.get("/api/files", url.Values{"path": {dir}})
	if err != nil {
		return nil, err
	}
	var entries []entry
	return entries, json.Unmarshal(body, &entries)
}

// upload replaces file on the device. With overwrite=1 CrossPoint writes a
// .part file and swaps it in only when complete, so a failed upload keeps
// the old cover.
func (c *client) upload(file string, data []byte) error {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", path.Base(file))
	if err != nil {
		return err
	}
	part.Write(data)
	form.Close()

	q := url.Values{"path": {path.Dir(file)}, "overwrite": {"1"}}
	resp, err := c.http.Post(c.base+"/upload?"+q.Encode(), form.FormDataContentType(), &body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload %s: %s: %s", file, resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}

// titles maps book cache folders to book titles from recent.json. Folders of
// books that are not recent keep their folder name.
func (c *client) titles() map[string]string {
	titles := map[string]string{}
	body, err := c.download(cacheRoot + "/recent.json")
	if err != nil {
		return titles
	}
	var recent struct {
		Books []struct {
			Title        string `json:"title"`
			CoverBmpPath string `json:"coverBmpPath"`
		} `json:"books"`
	}
	if json.Unmarshal(body, &recent) == nil {
		for _, b := range recent.Books {
			titles[path.Base(path.Dir(b.CoverBmpPath))] = b.Title
		}
	}
	return titles
}

func isBookCache(name string) bool {
	return strings.HasPrefix(name, "epub_") || strings.HasPrefix(name, "txt_") || strings.HasPrefix(name, "xtc_")
}

func isCover(name string) bool {
	return strings.HasPrefix(name, "cover") && strings.HasSuffix(name, ".bmp")
}

type options struct {
	width, height int
	dryRun        bool
}

// run checks every cached sleep cover and pads the ones that do not fill the
// screen. It returns the number of covers that failed.
func run(c *client, opt options, out io.Writer) (int, error) {
	body, err := c.get("/api/status", nil)
	if err != nil {
		return 0, fmt.Errorf("device not reachable: %w", err)
	}
	var status struct {
		Version        string `json:"version"`
		FirmwareFamily string `json:"firmwareFamily"`
		Device         string `json:"device"`
	}
	if err := json.Unmarshal(body, &status); err != nil || status.FirmwareFamily != "crosspoint" {
		return 0, fmt.Errorf("%s is not a CrossPoint device", c.base)
	}
	fmt.Fprintf(out, "CrossPoint %s on %s, screen %dx%d\n", status.Version, status.Device, opt.width, opt.height)

	titles := c.titles()
	dirs, err := c.list(cacheRoot)
	if err != nil {
		return 0, err
	}
	var padded, full, skipped, failed int
	for _, dir := range dirs {
		if !dir.IsDirectory || !isBookCache(dir.Name) {
			continue
		}
		files, err := c.list(cacheRoot + "/" + dir.Name)
		if err != nil {
			fmt.Fprintf(out, "%s: %v\n", dir.Name, err)
			failed++
			continue
		}
		label := dir.Name
		if t, ok := titles[dir.Name]; ok {
			label = t + " (" + dir.Name + ")"
		}
		for _, f := range files {
			if f.IsDirectory || !isCover(f.Name) {
				continue
			}
			file := cacheRoot + "/" + dir.Name + "/" + f.Name
			result, msg, err := process(c, file, opt)
			switch {
			case err != nil:
				failed++
				msg = "FAILED: " + err.Error()
			case result == resultPadded:
				padded++
			case result == resultSkipped:
				skipped++
			default:
				full++
			}
			fmt.Fprintf(out, "%s %s: %s\n", label, f.Name, msg)
		}
	}
	verb := "padded"
	if opt.dryRun {
		verb = "to pad"
	}
	fmt.Fprintf(out, "%d %s, %d already full screen, %d skipped, %d failed\n", padded, verb, full, skipped, failed)
	return failed, nil
}

type result int

const (
	resultFull result = iota
	resultPadded
	resultSkipped
)

// process pads one cached cover. In a dry run a cover that needs padding
// counts as resultPadded but nothing is uploaded.
func process(c *client, file string, opt options) (result, string, error) {
	data, err := c.download(file)
	if err != nil {
		return 0, "", err
	}
	img, err := parseBMP(data)
	if err != nil {
		return resultSkipped, "skipped, " + err.Error(), nil
	}
	if img.width == opt.width && img.height == opt.height {
		return resultFull, "already full screen", nil
	}
	filled, err := fill(img, opt.width, opt.height)
	if err != nil {
		return resultSkipped, "skipped, " + err.Error(), nil
	}
	change := fmt.Sprintf("%dx%d -> %dx%d", img.width, img.height, opt.width, opt.height)
	if opt.dryRun {
		return resultPadded, "would pad " + change, nil
	}
	padded := filled.encode()
	if err := c.upload(file, padded); err != nil {
		return 0, "", err
	}
	check, err := c.download(file)
	if err != nil {
		return 0, "", fmt.Errorf("uploaded, but the check download failed: %w", err)
	}
	if !bytes.Equal(check, padded) {
		return 0, "", fmt.Errorf("uploaded, but the device returned different bytes")
	}
	return resultPadded, "padded " + change, nil
}

func main() {
	host := flag.String("host", "http://crosspoint.local", "CrossPoint File Transfer address")
	width := flag.Int("width", 480, "screen width in portrait")
	height := flag.Int("height", 800, "screen height in portrait")
	dryRun := flag.Bool("n", false, "dry run: report what would change, upload nothing")
	showVersion := flag.Bool("version", false, "print the version")
	flag.Parse()

	if *showVersion {
		fmt.Println("coverfill", version)
		return
	}

	base := strings.TrimRight(*host, "/")
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}
	c := &client{base: base, http: &http.Client{Timeout: 2 * time.Minute}}
	failed, err := run(c, options{width: *width, height: *height, dryRun: *dryRun}, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "coverfill:", err)
		os.Exit(1)
	}
	if failed > 0 {
		os.Exit(1)
	}
}
