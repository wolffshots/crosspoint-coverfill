# crosspoint-coverfill

`coverfill` makes the book cover fill the whole sleep screen on a CrossPoint e-reader, such as an Xteink X4 or X4 Pro. It pads each cached sleep cover with mirrored edges, so the white bars of Fit mode go away and Crop mode cuts nothing.

It works over the File Transfer web server of CrossPoint. It changes no firmware, no book and no setting.

## How it works

CrossPoint makes a BMP of the cover the first time the device sleeps with a book open. It keeps the BMP at `/.crosspoint/epub_<id>/cover_*.bmp` and uses that file on each later sleep. CrossPoint only checks that the file exists, so it draws a replaced file as it is.

In Fit mode the BMP has the screen width or the screen height, but not both. For example, a cover can be 480x720 on a 480x800 screen. CrossPoint draws a BMP of the screen size at full screen with no scaling.

`coverfill` does these steps for each cached cover:

1. Download the BMP.
2. If the BMP is already the screen size, go to the next cover.
3. Pad the BMP to the screen size. The padding repeats the edge of the cover, flipped like a mirror. The cover stays at the same position on the screen.
4. Upload the BMP in place of the old file.
5. Download the BMP again and compare it with the upload.

The padded BMP keeps the 2-bit gray palette and the dithering of the firmware. The device draws it with no new conversion.

## Install

### Homebrew (macOS / Linux)

```sh
brew install wolffshots/tap/coverfill
```

The formula builds from source through [wolffshots/homebrew-tap](https://github.com/wolffshots/homebrew-tap).

### Scoop (Windows)

```powershell
scoop bucket add wolffshots https://github.com/wolffshots/scoop-bucket
scoop install wolffshots/coverfill
```

Scoop installs the Windows x86-64 binary from the latest release through [wolffshots/scoop-bucket](https://github.com/wolffshots/scoop-bucket).

### go install

```sh
go install github.com/wolffshots/crosspoint-coverfill@latest
```

This command installs the binary as `crosspoint-coverfill`. A binary from `go install` prints `coverfill dev` for `--version`.

### Prebuilt binaries

Download a binary from the [latest release](https://github.com/wolffshots/crosspoint-coverfill/releases/latest). The release has binaries for Linux x86-64, Windows x86-64 and macOS arm64, and a `checksums.txt` file.

## Use

1. On the device, open File Transfer and join your Wi-Fi network.
2. Note the address that the device shows.
3. Run a dry run. It shows the covers to pad and uploads nothing.

```bash
coverfill -host 192.168.88.112 -n
```

4. Pad the covers.

```bash
coverfill -host 192.168.88.112
```

| Flag | Default | Meaning |
|---|---|---|
| `-host` | `http://crosspoint.local` | The address of the File Transfer web server |
| `-n` | off | Dry run. Report the changes and upload nothing. |
| `-width` | `480` | The screen width in portrait |
| `-height` | `800` | The screen height in portrait |
| `-version` | off | Print the version and stop. |

The exit code is 1 if a cover failed or the device did not answer.

## When to run it again

A book gets a cover BMP only after the device sleeps once with that book open. Run `coverfill` again after you start a new book.

CrossPoint deletes the cache of a book when you clear the cache or upload the book again. The device then makes a new BMP with white bars. Run `coverfill` again to pad it.

A second run changes nothing for covers that are already full screen.

## Run on a schedule with Docker

The Docker image runs `coverfill` on a cron schedule for a list of devices. Each run calls `coverfill -host <address>` once for each device.

The image is `ghcr.io/wolffshots/crosspoint-coverfill` for `linux/amd64` and `linux/arm64`. The tag `latest` is the latest release. A tag such as `0.2.0` or `0.2` pins a release.

1. Give each device a fixed IP address in your router. The container cannot resolve `crosspoint.local`.
2. Copy [compose.yaml](compose.yaml) to your server.
3. Set the addresses and the schedule in `compose.yaml`.
4. Start the container.

```bash
docker compose up -d
```

To update to a new release:

```bash
docker compose pull && docker compose up -d
```

| Variable | Default | Meaning |
|---|---|---|
| `COVERFILL_HOSTS` | none, required | The device addresses, separated by spaces or commas |
| `COVERFILL_SCHEDULE` | `*/5 * * * *` | A 5-field cron schedule |
| `COVERFILL_ARGS` | empty | Extra flags for each run, for example `-n` or `-width 480 -height 800` |
| `TZ` | `UTC` | The time zone of the schedule, for example `Europe/London` |

The device answers only while File Transfer is open on it. At other times the log shows `device not reachable` for that device, and the next run tries again. A short schedule finds the device sooner.

A run that starts while the previous run is busy does nothing.

Read the log:

```bash
docker compose logs -f coverfill
```

Start a run now:

```bash
docker compose exec coverfill coverfill-all
```

## Undo

Delete the padded `cover_*.bmp` file in the File Transfer page, or send a request to the delete endpoint:

```bash
curl -X POST -d "path=/.crosspoint/epub_<id>/cover_legacy_v2.bmp" http://192.168.88.112/delete
```

The device makes the cover BMP again the next time it sleeps with that book open.

## Limits

- The tool needs the Cover, Cover + Custom or Cover + Transparent sleep screen with Cover Mode set to Fit.
- Text near the edge of a cover shows flipped in the padding.
- A cover that is smaller than the screen in both directions is skipped. Fit mode does not make such a file for a normal cover.
- The tool was tested against CrossPoint 2.0.0 on an X4 Pro. Another firmware version can change the cache layout.

## Build

```bash
go build -o coverfill .
go test ./...
```

The tool uses the Go standard library only.

## Release

Push a tag that starts with `v`, for example `v0.1.0`. The release workflow runs the tests, builds the binaries, writes `checksums.txt` and makes the GitHub release. The docker workflow builds the image and pushes it to `ghcr.io`.

To publish an image of `main` with the tag `main`, run the docker workflow by hand:

```bash
gh workflow run docker.yml --ref main
```

Build the image locally:

```bash
docker build -t coverfill .
```

## License

MIT. See [LICENSE](LICENSE).
