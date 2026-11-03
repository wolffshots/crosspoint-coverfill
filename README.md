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

The exit code is 1 if a cover failed or the device did not answer.

## When to run it again

A book gets a cover BMP only after the device sleeps once with that book open. Run `coverfill` again after you start a new book.

CrossPoint deletes the cache of a book when you clear the cache or upload the book again. The device then makes a new BMP with white bars. Run `coverfill` again to pad it.

A second run changes nothing for covers that are already full screen.

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
