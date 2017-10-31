package log

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"time"
)

type ioLogger struct {
	c   chan Event
	out io.Writer
	buf []byte
}

var (
	sub *Subscription
)

func init() {
	l := &ioLogger{c: make(chan Event, 100), out: os.Stderr}
	sub = Subscribe(func(evt Event) {
		l.c <- evt
	})
	go l.listenEvent()
}

func (l *ioLogger) listenEvent() {
	for true {
		e := <-l.c
		l.writeEvent(e)
	}
}

// Cheap integer to fixed-width decimal ASCII.  Give a negative width to avoid zero-padding.
func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

func (l *ioLogger) formatHeader(buf *[]byte, prefix string, t time.Time) {
	t = t.UTC()
	// Y/M/D
	year, month, day := t.Date()
	itoa(buf, year, 4)
	*buf = append(*buf, '/')
	itoa(buf, int(month), 2)
	*buf = append(*buf, '/')
	itoa(buf, day, 2)
	*buf = append(*buf, ' ')

	// H/M/S
	hour, min, sec := t.Clock()
	itoa(buf, hour, 2)
	*buf = append(*buf, ':')
	itoa(buf, min, 2)
	*buf = append(*buf, ':')
	itoa(buf, sec, 2)

	// no microseconds
	//*buf = append(*buf, '.')
	//itoa(buf, t.Nanosecond()/1e3, 6)

	*buf = append(*buf, ' ')
	if len(prefix) > 0 {
		*buf = append(*buf, prefix...)
		*buf = append(*buf, ' ')
	}
}

func (l *ioLogger) writeEvent(e Event) {
	l.buf = l.buf[:0]
	l.formatHeader(&l.buf, e.Prefix, e.Time)
	l.out.Write(l.buf)
	if len(e.Message) > 0 {
		l.out.Write([]byte(e.Message))
		l.out.Write([]byte{' '})
	}

	wr := ioEncoder{l.out}
	for _, f := range e.Context {
		f.Encode(wr)
		l.out.Write([]byte{' '})
	}
	for _, f := range e.Fields {
		f.Encode(wr)
		l.out.Write([]byte{' '})
	}
	wr.Write([]byte{'\n'})
}

type ioEncoder struct {
	io.Writer
}

func (e ioEncoder) EncodeBool(key string, val bool) {
	fmt.Fprintf(e, "%s=%t", key, val)
}

func (e ioEncoder) EncodeFloat64(key string, val float64) {
	fmt.Fprintf(e, "%s=%f", key, val)
}

func (e ioEncoder) EncodeInt(key string, val int) {
	fmt.Fprintf(e, "%s=%d", key, val)
}

func (e ioEncoder) EncodeInt64(key string, val int64) {
	fmt.Fprintf(e, "%s=%d", key, val)
}

func (e ioEncoder) EncodeDuration(key string, val time.Duration) {
	fmt.Fprintf(e, "%s=%s", key, val)
}

func (e ioEncoder) EncodeUint(key string, val uint) {
	fmt.Fprintf(e, "%s=%d", key, val)
}

func (e ioEncoder) EncodeUint64(key string, val uint64) {
	fmt.Fprintf(e, "%s=%d", key, val)
}

func (e ioEncoder) EncodeString(key string, val string) {
	fmt.Fprintf(e, "%s=%q", key, val)
}

func (e ioEncoder) EncodeObject(key string, val interface{}) {
	fmt.Fprintf(e, "%s=%q", key, val)
}

func (e ioEncoder) EncodeType(key string, val reflect.Type) {
	fmt.Fprintf(e, "%s=%v", key, val)
}
