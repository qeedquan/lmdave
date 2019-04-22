package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
)

type Level struct {
	Path  [256]uint8
	Tiles [1000]uint8
	Pad   [24]uint8
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("dump_dave: ")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 2 {
		usage()
	}
	err := dump(flag.Arg(0), flag.Arg(1))
	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: [options] infile outdir")
	flag.PrintDefaults()
	os.Exit(2)
}

func writepng(name string, img *image.RGBA) error {
	fmt.Printf("writing image: %s\n", name)
	w, err := os.Create(name)
	if err != nil {
		return err
	}
	err = png.Encode(w, img)
	xerr := w.Close()
	if err == nil {
		err = xerr
	}
	return err
}

func dump(infile, outdir string) error {
	data, err := ioutil.ReadFile(infile)
	if err != nil {
		return err
	}

	os.MkdirAll(outdir, 0755)

	var ulz ulz
	data, err = ulz.decode(data)
	if err != nil {
		return err
	}

	name := filepath.Join(outdir, "daveu.exe")
	fmt.Printf("writing uncompressed exe: %s\n", name)
	err = ioutil.WriteFile(name, data, 0644)
	if err != nil {
		return err
	}

	var lvls []Level
	for i := 0; i < 10; i++ {
		lvl, err := dumplevel(data, outdir, i)
		if err != nil {
			return fmt.Errorf("level%d: %v", i, err)
		}
		lvls = append(lvls, lvl)
	}

	imgs, err := dumptiles(data, outdir)
	if err != nil {
		return err
	}

	err = dumpmap(lvls, imgs, outdir)
	if err != nil {
		return err
	}

	return nil
}

func dumpmap(lvls []Level, imgs []*image.RGBA, outdir string) error {
	m := image.NewRGBA(image.Rect(0, 0, 1600, 1600))
	for k := 0; k < 10; k++ {
		for j := 0; j < 10; j++ {
			for i := 0; i < 100; i++ {
				id := lvls[k].Tiles[j*100+i]
				pt := image.Pt(i*16, k*160+j*16)
				dp := image.Rect(pt.X, pt.Y, pt.X+16, pt.Y+16)
				draw.Draw(m, dp, imgs[id], image.ZP, draw.Over)
			}
		}
	}

	err := writepng(filepath.Join(outdir, "map.png"), m)
	if err != nil {
		return err
	}

	return nil
}

func dumplevel(data []byte, outdir string, level int) (Level, error) {
	const leveloff = 0x26e0a
	const levelsize = 1280

	start := leveloff + levelsize*int64(level)
	end := start + levelsize
	if end >= int64(len(data)) {
		return Level{}, fmt.Errorf("level does not exist in file")
	}
	r := bytes.NewBuffer(data[start:end])

	var l Level
	binary.Read(r, binary.LittleEndian, &l)

	name := fmt.Sprintf("%s/level%d.dat", outdir, level)
	fmt.Printf("writing level: %s\n", name)
	w, err := os.Create(name)
	if err != nil {
		return Level{}, err
	}

	err = binary.Write(w, binary.LittleEndian, &l)
	xerr := w.Close()
	if err == nil {
		err = xerr
	}

	return l, err
}

func dumptiles(data []byte, outdir string) ([]*image.RGBA, error) {
	const vgaoff = 0x120f0
	const paloff = 0x26b0a

	r := bytes.NewReader(data)
	rv := io.NewSectionReader(r, vgaoff, math.MaxInt32)
	rp := io.NewSectionReader(r, paloff, math.MaxInt32)

	var out []byte
	var flen uint32
	binary.Read(rv, binary.LittleEndian, &flen)
	for clen := uint32(0); clen < flen; {
		var b, nb uint8
		binary.Read(rv, binary.LittleEndian, &b)
		if b&0x80 != 0 {
			b &= 0x7f
			for b++; b != 0; b-- {
				binary.Read(rv, binary.LittleEndian, &nb)
				out = append(out, nb)
				clen++
			}
		} else {
			binary.Read(rv, binary.LittleEndian, &nb)
			for b += 3; b != 0; b-- {
				out = append(out, nb)
				clen++
			}
		}
	}

	var pal [768]uint8
	for i := 0; i < 256; i++ {
		for j := 0; j < 3; j++ {
			binary.Read(rp, binary.LittleEndian, &pal[i*3+j])
			pal[i*3+j] <<= 2
		}
	}

	if len(out) < 4 {
		return nil, fmt.Errorf("decompressed tile index buffer of %d bytes is too small", len(out))
	}
	tn := binary.LittleEndian.Uint32(out[0:])

	if int64(len(out)) < int64(tn)*4+4 {
		return nil, fmt.Errorf("decompressed tile index buffer of %d bytes is too small for tile length of %d bytes", len(out), tn)
	}

	var ti []uint32
	for i := uint32(0); i < tn; i++ {
		t := binary.LittleEndian.Uint32(out[i*4+4:])
		ti = append(ti, t)
	}
	ti = append(ti, flen)

	var imgs []*image.RGBA
	for tc := uint32(0); tc < tn; tc++ {
		if int64(tc) >= int64(len(ti)) {
			return nil, fmt.Errorf("invalid tile index %d", tc)
		}
		cb := ti[tc]
		tw, th := 16, 16
		if cb > 65280 {
			cb++
		}

		if out[cb+1] == 0 && out[cb+3] == 0 {
			if out[cb] > 0 && out[cb] < 0xbf && out[cb+2] > 0 && out[cb+2] < 0x64 {
				tw, th = int(out[cb]), int(out[cb+2])
				cb += 4
			}
		}

		m := image.NewRGBA(image.Rect(0, 0, tw, th))
		x, y := 0, 0
		for ; cb < ti[tc+1]; cb++ {
			if int64(cb) >= int64(len(out)) {
				return nil, fmt.Errorf("invalid current byte index %d", cb)
			}
			sb := int(out[cb])
			if sb*3+2 >= len(pal) {
				return nil, fmt.Errorf("invalid palette index %d", sb)
			}
			col := color.RGBA{pal[sb*3], pal[sb*3+1], pal[sb*3+2], 255}
			m.Set(x, y, col)
			if x++; x >= tw {
				x, y = 0, y+1
			}
		}
		imgs = append(imgs, m)

		name := fmt.Sprintf("%s/tile%d.png", outdir, tc)
		err := writepng(name, m)
		if err != nil {
			return nil, fmt.Errorf("tile%d: %v", tc, err)
		}
	}

	return imgs, nil
}

type ulz struct {
	in       []byte
	out      []byte
	ihead    [0x10]uint16
	ohead    [0x10]uint16
	inf      [8]uint16
	ver      int
	loadsize uint32
}

func (p *ulz) decode(in []byte) ([]byte, error) {
	p.in = in
	p.ver = p.rdhead()
	if p.ver == 0 {
		return p.in, nil
	}

	err := p.mkreltbl()
	if err != nil {
		return nil, err
	}

	fpos := len(p.out)
	i := (0x200 - fpos) & 0x1ff
	p.ohead[4] = uint16((fpos + i) >> 4)
	for ; i > 0; i-- {
		p.out = append(p.out, 0)
	}

	err = p.unpack()
	if err != nil {
		return nil, err
	}
	p.wrhead()

	return p.out, nil
}

func (p *ulz) rdhead() int {
	r := bytes.NewReader(p.in)
	err := binary.Read(r, binary.LittleEndian, &p.ihead)
	if err != nil {
		return 0
	}
	copy(p.ohead[:], p.ihead[:])

	if p.ihead[0x0] != 0x5a4d || p.ihead[0xd] != 0 || p.ihead[0xc] != 0x1c {
		return 0
	}

	entry := (int(p.ihead[0x4])+int(p.ihead[0xb]))<<4 + int(p.ihead[0xa])
	if entry >= len(p.in)+len(sig90) {
		return 0
	}
	status := bytes.Compare(p.in[entry:entry+len(sig90)], sig90)
	if status == 0 {
		return 90
	}
	status = bytes.Compare(p.in[entry:entry+len(sig91)], sig91)
	if status == 0 {
		return 91
	}

	return 0
}

func (p *ulz) mkreltbl() error {
	fpos := (uint32(p.ihead[0xb]) + uint32(p.ihead[0x4])) << 4
	if fpos >= uint32(len(p.in)) {
		return fmt.Errorf("compressed relocation table not found")
	}
	r := bytes.NewBuffer(p.in[fpos:])
	err := binary.Read(r, binary.LittleEndian, &p.inf)
	if err != nil {
		return err
	}
	p.ohead[0xa] = p.inf[0]
	p.ohead[0xb] = p.inf[1]
	p.ohead[0x8] = p.inf[2]
	p.ohead[0x7] = p.inf[3]
	p.ohead[0xc] = 0x1c

	p.out = make([]byte, 0x1c)
	switch p.ver {
	case 90:
		err = p.reloc90(fpos)
	case 91:
		err = p.reloc91(fpos)
	default:
		panic("unreachable")
	}

	return err
}

func (p *ulz) reloc90(fpos uint32) error {
	fpos += 0x19d
	if fpos >= uint32(len(p.in)) {
		return fmt.Errorf("compressed relocation table not found")
	}
	r := bytes.NewReader(p.in[fpos:])

	var relcount, reloff, relseg uint16
	for {
		var c uint16
		err := binary.Read(r, binary.LittleEndian, &c)
		if err != nil {
			return err
		}
		for ; c > 0; c-- {
			err = binary.Read(r, binary.LittleEndian, &reloff)
			if err != nil {
				return err
			}

			p.putword(reloff)
			p.putword(relseg)
			relcount++
		}
		relseg += 0x1000
		if int32(relseg) >= 0xf000+0x1000 {
			break
		}
	}
	p.ohead[3] = relcount
	return nil
}

func (p *ulz) reloc91(fpos uint32) error {
	fpos += 0x158
	if fpos >= uint32(len(p.in)) {
		return fmt.Errorf("compressed relocation table not found")
	}
	r := bytes.NewReader(p.in[fpos:])

	var relcount, reloff, relseg, span uint16
	for {
		temp, err := r.ReadByte()
		if err != nil {
			return err
		}

		span = uint16(temp)
		if span == 0 {
			err = binary.Read(r, binary.LittleEndian, &span)
			if err != nil {
				return err
			}
			if span == 0 {
				relseg += 0xfff
			} else if span == 1 {
				break
			}
		}
		reloff += span
		relseg += (reloff &^ 0x0f) >> 4
		reloff &= 0xf
		p.putword(reloff)
		p.putword(relseg)
		relcount++
	}
	p.ohead[3] = relcount
	return nil
}

func (p *ulz) putword(v uint16) {
	p.out = append(p.out, uint8(v))
	p.out = append(p.out, uint8(v>>8))
}

func (p *ulz) unpack() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not unpack corrupted compressed stream")
		}
	}()

	var data [0x4500]byte
	var bits bitstream
	var ln int
	var span int16

	dp := 0
	ip := (int(p.ihead[0xb]) - int(p.inf[0x4]) + int(p.ihead[0x4])) << 4
	op := int(p.ohead[4]) << 4
	iop := op

	bits.init(p.in, &ip)

	for {
		if dp > 0x4000 {
			if len(p.out) < 0x2000+op {
				p.resize(0x2000 + op)
			}
			copy(p.out[op:], data[:0x2000])
			op += 0x2000
			dp -= 0x2000
			copy(data[:dp], data[0x2000:])
		}
		if bits.getbit(&ip) != 0 {
			data[dp], dp = p.in[ip], dp+1
			ip++
			continue
		}
		if bits.getbit(&ip) == 0 {
			ln = bits.getbit(&ip) << 1
			ln |= bits.getbit(&ip)
			ln += 2
			span = int16(uint16(p.in[ip]) | 0xff00)
			ip++
		} else {
			span, ip = int16(p.in[ip]), ip+1
			ln, ip = int(p.in[ip]), ip+1
			span |= int16(((ln &^ 0x07) << 5) | 0xe000)
			ln = (ln & 0x07) + 2
			if ln == 2 {
				ln, ip = int(p.in[ip]), ip+1
				if ln == 0 {
					break
				}
				if ln == 1 {
					continue
				} else {
					ln++
				}
			}
		}

		for ; ln > 0; ln, dp = ln-1, dp+1 {
			data[dp] = data[dp+int(span)]
		}
	}
	if dp != len(data) {
		size := dp
		vecsize := len(p.out)
		if vecsize < size+op {
			p.resize(size + op)
		}
		copy(p.out[op:], data[:size])
		op += size
	}
	p.loadsize = uint32(op - iop)
	return
}

func (p *ulz) resize(n int) {
	if len(p.out) >= n {
		return
	}
	p.out = append(p.out, make([]byte, n-len(p.out))...)
}

func (p *ulz) wrhead() {
	if p.ihead[6] != 0 {
		p.ohead[5] -= p.inf[5] + ((p.inf[6] + 16 - 1) >> 4) + 9
		if p.ihead[6] != 0xffff {
			p.ohead[6] -= p.ihead[5] - p.ohead[5]
		}
	}
	p.ohead[1] = uint16(p.loadsize+(uint32(p.ohead[4])<<4)) & 0x1ff
	p.ohead[2] = uint16((p.loadsize + ((uint32(p.ohead[4]) << 4) + 0x1ff)) >> 9)
	for i := 0; i < 0xe; i++ {
		binary.LittleEndian.PutUint16(p.out[i*2:], p.ohead[i])
	}
}

type bitstream struct {
	data  []byte
	buf   uint16
	count uint8
}

func (p *bitstream) init(data []byte, pos *int) {
	p.count = 0x10
	p.data = data
	p.buf = binary.LittleEndian.Uint16(p.data[*pos:])
	*pos += 2
}

func (p *bitstream) getbit(pos *int) int {
	b := int(p.buf & 0x1)
	if p.count--; p.count == 0 {
		p.buf = binary.LittleEndian.Uint16(p.data[*pos:])
		*pos += 2
		p.count = 0x10
	} else {
		p.buf >>= 1
	}
	return b
}

var sig90 = []byte{
	0x06, 0x0E, 0x1F, 0x8B, 0x0E, 0x0C, 0x00, 0x8B,
	0xF1, 0x4E, 0x89, 0xF7, 0x8C, 0xDB, 0x03, 0x1E,
	0x0A, 0x00, 0x8E, 0xC3, 0xB4, 0x00, 0x31, 0xED,
	0xFD, 0xAC, 0x01, 0xC5, 0xAA, 0xE2, 0xFA, 0x8B,
	0x16, 0x0E, 0x00, 0x8A, 0xC2, 0x29, 0xC5, 0x8A,
	0xC6, 0x29, 0xC5, 0x39, 0xD5, 0x74, 0x0C, 0xBA,
	0x91, 0x01, 0xB4, 0x09, 0xCD, 0x21, 0xB8, 0xFF,
	0x4C, 0xCD, 0x21, 0x53, 0xB8, 0x53, 0x00, 0x50,
	0xCB, 0x2E, 0x8B, 0x2E, 0x08, 0x00, 0x8C, 0xDA,
	0x89, 0xE8, 0x3D, 0x00, 0x10, 0x76, 0x03, 0xB8,
	0x00, 0x10, 0x29, 0xC5, 0x29, 0xC2, 0x29, 0xC3,
	0x8E, 0xDA, 0x8E, 0xC3, 0xB1, 0x03, 0xD3, 0xE0,
	0x89, 0xC1, 0xD1, 0xE0, 0x48, 0x48, 0x8B, 0xF0,
	0x8B, 0xF8, 0xF3, 0xA5, 0x09, 0xED, 0x75, 0xD8,
	0xFC, 0x8E, 0xC2, 0x8E, 0xDB, 0x31, 0xF6, 0x31,
	0xFF, 0xBA, 0x10, 0x00, 0xAD, 0x89, 0xC5, 0xD1,
	0xED, 0x4A, 0x75, 0x05, 0xAD, 0x89, 0xC5, 0xB2,
	0x10, 0x73, 0x03, 0xA4, 0xEB, 0xF1, 0x31, 0xC9,
	0xD1, 0xED, 0x4A, 0x75, 0x05, 0xAD, 0x89, 0xC5,
	0xB2, 0x10, 0x72, 0x22, 0xD1, 0xED, 0x4A, 0x75,
	0x05, 0xAD, 0x89, 0xC5, 0xB2, 0x10, 0xD1, 0xD1,
	0xD1, 0xED, 0x4A, 0x75, 0x05, 0xAD, 0x89, 0xC5,
	0xB2, 0x10, 0xD1, 0xD1, 0x41, 0x41, 0xAC, 0xB7,
	0xFF, 0x8A, 0xD8, 0xE9, 0x13, 0x00, 0xAD, 0x8B,
	0xD8, 0xB1, 0x03, 0xD2, 0xEF, 0x80, 0xCF, 0xE0,
	0x80, 0xE4, 0x07, 0x74, 0x0C, 0x88, 0xE1, 0x41,
	0x41, 0x26, 0x8A, 0x01, 0xAA, 0xE2, 0xFA, 0xEB,
	0xA6, 0xAC, 0x08, 0xC0, 0x74, 0x40, 0x3C, 0x01,
	0x74, 0x05, 0x88, 0xC1, 0x41, 0xEB, 0xEA, 0x89,
}

var sig91 = []byte{
	0x06, 0x0E, 0x1F, 0x8B, 0x0E, 0x0C, 0x00, 0x8B,
	0xF1, 0x4E, 0x89, 0xF7, 0x8C, 0xDB, 0x03, 0x1E,
	0x0A, 0x00, 0x8E, 0xC3, 0xFD, 0xF3, 0xA4, 0x53,
	0xB8, 0x2B, 0x00, 0x50, 0xCB, 0x2E, 0x8B, 0x2E,
	0x08, 0x00, 0x8C, 0xDA, 0x89, 0xE8, 0x3D, 0x00,
	0x10, 0x76, 0x03, 0xB8, 0x00, 0x10, 0x29, 0xC5,
	0x29, 0xC2, 0x29, 0xC3, 0x8E, 0xDA, 0x8E, 0xC3,
	0xB1, 0x03, 0xD3, 0xE0, 0x89, 0xC1, 0xD1, 0xE0,
	0x48, 0x48, 0x8B, 0xF0, 0x8B, 0xF8, 0xF3, 0xA5,
	0x09, 0xED, 0x75, 0xD8, 0xFC, 0x8E, 0xC2, 0x8E,
	0xDB, 0x31, 0xF6, 0x31, 0xFF, 0xBA, 0x10, 0x00,
	0xAD, 0x89, 0xC5, 0xD1, 0xED, 0x4A, 0x75, 0x05,
	0xAD, 0x89, 0xC5, 0xB2, 0x10, 0x73, 0x03, 0xA4,
	0xEB, 0xF1, 0x31, 0xC9, 0xD1, 0xED, 0x4A, 0x75,
	0x05, 0xAD, 0x89, 0xC5, 0xB2, 0x10, 0x72, 0x22,
	0xD1, 0xED, 0x4A, 0x75, 0x05, 0xAD, 0x89, 0xC5,
	0xB2, 0x10, 0xD1, 0xD1, 0xD1, 0xED, 0x4A, 0x75,
	0x05, 0xAD, 0x89, 0xC5, 0xB2, 0x10, 0xD1, 0xD1,
	0x41, 0x41, 0xAC, 0xB7, 0xFF, 0x8A, 0xD8, 0xE9,
	0x13, 0x00, 0xAD, 0x8B, 0xD8, 0xB1, 0x03, 0xD2,
	0xEF, 0x80, 0xCF, 0xE0, 0x80, 0xE4, 0x07, 0x74,
	0x0C, 0x88, 0xE1, 0x41, 0x41, 0x26, 0x8A, 0x01,
	0xAA, 0xE2, 0xFA, 0xEB, 0xA6, 0xAC, 0x08, 0xC0,
	0x74, 0x34, 0x3C, 0x01, 0x74, 0x05, 0x88, 0xC1,
	0x41, 0xEB, 0xEA, 0x89, 0xFB, 0x83, 0xE7, 0x0F,
	0x81, 0xC7, 0x00, 0x20, 0xB1, 0x04, 0xD3, 0xEB,
	0x8C, 0xC0, 0x01, 0xD8, 0x2D, 0x00, 0x02, 0x8E,
	0xC0, 0x89, 0xF3, 0x83, 0xE6, 0x0F, 0xD3, 0xEB,
	0x8C, 0xD8, 0x01, 0xD8, 0x8E, 0xD8, 0xE9, 0x72,
}
