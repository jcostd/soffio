// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"encoding/binary"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
)

// imageConfig reads the width and height of the image in r: a WebP's
// from its first 30 bytes, as the standard library has no WebP and
// soffio wants no more of it; a jpeg's, png's or gif's as the standard
// library reads them.
func imageConfig(r io.ReadSeeker) (image.Config, error) {
	var h [30]byte
	if _, err := io.ReadFull(r, h[:]); err == nil && string(h[:4]) == "RIFF" && string(h[8:12]) == "WEBP" {
		return webpConfig(h)
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return image.Config{}, err
	}
	c, _, err := image.DecodeConfig(r)
	return c, err
}

// webpConfig reads the size in a WebP's first chunk, after its 12
// bytes of RIFF header and 8 of chunk header: a lossy VP8 frame's,
// 14 bits each after its start code; a lossless VP8L's, 14 bits each,
// less one, after its signature; an extended VP8X's canvas, 24 bits
// each, less one, after its flags.
func webpConfig(h [30]byte) (image.Config, error) {
	le := binary.LittleEndian
	u24 := func(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }
	switch string(h[12:16]) {
	case "VP8 ":
		if string(h[23:26]) == "\x9d\x01\x2a" {
			return image.Config{Width: int(le.Uint16(h[26:]) & 0x3fff), Height: int(le.Uint16(h[28:]) & 0x3fff)}, nil
		}
	case "VP8L":
		if h[20] == 0x2f {
			b := le.Uint32(h[21:])
			return image.Config{Width: int(b&0x3fff) + 1, Height: int(b>>14&0x3fff) + 1}, nil
		}
	case "VP8X":
		return image.Config{Width: u24(h[24:]) + 1, Height: u24(h[27:]) + 1}, nil
	}
	return image.Config{}, errors.New("not a WebP soffio reads")
}
