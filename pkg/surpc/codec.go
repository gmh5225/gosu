package surpc

import (
	"crypto/cipher"
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/can1357/gosu/pkg/settings"
)

type CipherRWC struct {
	underlying io.ReadWriteCloser
	reader     io.Reader
	writer     io.Writer
	iv         []byte
}

func (c *CipherRWC) Read(p []byte) (n int, err error) {
	return c.reader.Read(p)
}
func (c *CipherRWC) Write(p []byte) (n int, err error) {
	return c.writer.Write(p)
}
func (c *CipherRWC) Close() error {
	return c.underlying.Close()
}
func NewCipherWriter(underlying io.Writer, iv []byte) io.Writer {
	if block, err := settings.Rpc.Get().NewCipher(); err != nil {
		panic(err)
	} else {
		return &cipher.StreamWriter{S: cipher.NewOFB(block, iv), W: underlying}
	}
}
func NewCipherReader(underlying io.Reader, iv []byte) io.Reader {
	if block, err := settings.Rpc.Get().NewCipher(); err != nil {
		panic(err)
	} else {
		return &cipher.StreamReader{S: cipher.NewOFB(block, iv), R: underlying}
	}
}
func NewCipherRWC(underlying io.ReadWriteCloser, iv []byte) *CipherRWC {
	c := &CipherRWC{underlying: underlying, iv: iv}
	if block, err := settings.Rpc.Get().NewCipher(); err != nil {
		panic(err)
	} else {
		c.reader = &cipher.StreamReader{S: cipher.NewOFB(block, c.iv), R: c.underlying}
		c.writer = &cipher.StreamWriter{S: cipher.NewOFB(block, c.iv), W: c.underlying}
	}
	return c
}

type RWC struct {
	Reader io.Reader
	Writer io.Writer
	Closer io.Closer
}

func (c *RWC) Read(p []byte) (n int, err error) {
	if c.Reader == nil {
		return 0, io.EOF
	}
	return c.Reader.Read(p)
}
func (c *RWC) Write(p []byte) (n int, err error) {
	if c.Writer == nil {
		return 0, io.ErrNoProgress
	}
	return c.Writer.Write(p)
}
func (c *RWC) Close() error {
	if c.Closer == nil {
		return nil
	}
	return c.Closer.Close()
}

func NewServerCodec(conn io.ReadWriteCloser, iv []byte) rpc.ServerCodec {
	if iv != nil {
		conn = NewCipherRWC(conn, iv)
	}
	return jsonrpc.NewServerCodec(conn)
}
func NewClientCodec(conn io.ReadWriteCloser, iv []byte) rpc.ClientCodec {
	if iv != nil {
		conn = NewCipherRWC(conn, iv)
	}
	return jsonrpc.NewClientCodec(conn)
}
