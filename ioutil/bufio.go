package ioutil

// Anyone can use a bufio.Writer or bytes.Buffer to do buffering.
// For reading though, bytes.Buffer incurs copying which we can avoid.
// This is optimized for a zero-copy bufio implementation.
// As expected, structures here are not safe for concurrent use.

import (
	"io"
)

const defBufferSize = 1024

// BufReader is not safe for concurrent use.
type BufReader struct {
	rd   io.Reader
	buf  []byte
	r, w int
	err  error
}

// BufWriter is not safe for concurrent use.
type BufWriter struct {
	W   io.Writer
	buf []byte
	n   int
}

func NewBufReader(r io.Reader, b []byte) (br *BufReader) {
	if b == nil {
		if r == nil {
			return
		}
		b = make([]byte, defBufferSize)
	}
	br = &BufReader{rd: r, buf: b}
	return
}

func (b *BufReader) fill() {
	//This is only called if the buffer is empty (ie w = 0)
	if b.rd == nil || b.err != nil {
		return
	}
	var n int
	n, b.err = b.rd.Read(b.buf)
	b.w += n
}

func (b *BufReader) ReadN(n int) (bs []byte, err error) {
	if n == 0 {
		return
	}
	if b.r == b.w {
		b.r, b.w = 0, 0
	}
	if b.rd != nil && b.w == 0 {
		if b.err != nil {
			err, b.err = b.err, nil
			return
		}
		b.fill()
	}
	if b.w == 0 {
		if b.err != nil {
			err, b.err = b.err, nil
		}
		return
	}
	if n > b.w-b.r {
		n = b.w - b.r
	}
	n2 := b.r + n
	bs = b.buf[b.r:n2]
	b.r = n2
	return
}

func (b *BufReader) Read(bs []byte) (r int, err error) {
	n := len(bs)
	var bs2 []byte
	for r < n {
		bs2, err = b.ReadN(n - r)
		if n2 := len(bs2); n2 > 0 {
			copy(bs[r:], bs2[:n2])
			r += n2
		}
		if err != nil {
			return
		}
	}
	return
}

func NewBufWriter(w io.Writer, b []byte) (bw *BufWriter) {
	if b == nil {
		b = make([]byte, defBufferSize)
	}
	bw = &BufWriter{W: w, buf: b}
	return
}

func (b *BufWriter) Flush() (err error) {
	var i, w int = 0, 0
	for w != b.n {
		i, err = b.W.Write(b.buf[w:b.n])
		w += i
		if err != nil {
			if w != 0 {
				//slide (writing starts from b.n, and available from 0)
				copy(b.buf, b.buf[w:b.n])
				b.n -= w
			}
			return
		}
	}
	b.n = 0
	return
}

func (b *BufWriter) Write(bs []byte) (w int, err error) {
	n := len(bs)
	if b.n+n > len(b.buf) {
		if err = b.Flush(); err != nil {
			return
		}
		if n > len(b.buf) {
			w, err = b.W.Write(bs)
			return
		}
	}
	copy(b.buf[b.n:], bs)
	b.n += n
	w = n
	return
}
