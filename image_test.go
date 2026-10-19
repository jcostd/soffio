// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
	"io"
	"testing"
)

// webp makes the first bytes of a WebP: its RIFF header, then a chunk
// of kind with its payload
func webp(kind string, payload ...byte) []byte {
	b := append([]byte("RIFF\x00\x00\x00\x00WEBP"+kind+"\x00\x00\x00\x00"), payload...)
	return append(b, make([]byte, 30)...)
}

type noSeek struct{ io.Reader }

func (noSeek) Seek(int64, int) (int64, error) { return 0, errors.New("no seek") }

// a WebP's size is read from its header, as cwebp writes the three
// kinds; the rest is the standard library's
func TestImageConfig(t *testing.T) {
	le := binary.LittleEndian
	var pngFile bytes.Buffer
	if err := png.Encode(&pngFile, image.NewGray(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	lossless := le.AppendUint32([]byte{0x2f}, 299|500<<14)
	for _, tt := range []struct {
		name string
		file []byte
		w, h int
	}{
		{"lossy", webp("VP8 ", le.AppendUint16(le.AppendUint16([]byte{0, 0, 0, 0x9d, 0x01, 0x2a}, 641|1<<14), 479)...), 641, 479},
		{"lossless", webp("VP8L", lossless...), 300, 501},
		{"extended", webp("VP8X", 0x10, 0, 0, 0, 0xfe, 0x03, 0, 0xfe, 0x02, 0), 1023, 767},
		{"lossy without its start code", webp("VP8 ", 0, 0, 0, 1, 2, 3), 0, 0},
		{"lossless without its signature", webp("VP8L", 0x2e), 0, 0},
		{"another chunk", webp("ALPH"), 0, 0},
		{"png", pngFile.Bytes(), 3, 2},
		{"too short for any", []byte("RIFF"), 0, 0},
	} {
		c, err := imageConfig(bytes.NewReader(tt.file))
		if tt.w == 0 && err == nil || tt.w != 0 && (err != nil || c.Width != tt.w || c.Height != tt.h) {
			t.Errorf("%s: %dx%d, %v; want %dx%d", tt.name, c.Width, c.Height, err, tt.w, tt.h)
		}
	}
	if _, err := imageConfig(noSeek{bytes.NewReader(pngFile.Bytes())}); err == nil {
		t.Error("a file that can't seek back: no error")
	}
}
