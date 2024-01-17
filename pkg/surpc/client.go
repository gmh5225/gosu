package surpc

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/rpc"
	"net/url"
	"time"

	"github.com/can1357/gosu/pkg/settings"
	"github.com/can1357/gosu/pkg/util"
	"golang.org/x/net/websocket"
)

type Client struct {
	URL             *url.URL
	Http            *http.Client
	Ws              *rpc.Client
	Secure          bool
	requestTemplate http.Request
}

func NewClient(hostname string) (client *Client) {
	client = &Client{
		Http: &http.Client{},
	}

	local := false
	u, err := url.Parse(hostname)
	if err != nil {
		panic(err)
	}
	u.Path = "/rpc"
	if u.Hostname() == "localhost" || u.Hostname() == "[::1]" || u.Hostname() == "127.0.0.1" {
		local = true
	}

	tmp := http.Request{
		Method:     "POST",
		Host:       u.Host,
		URL:        u,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"Content-Type":    []string{"application/json"},
			"Accept-Encoding": []string{"identity"},
		},
	}

	if !local {
		settings := settings.Rpc.Get()
		if secret := settings.Secret; secret != "" {
			client.Secure = true
			tmp.Header = http.Header{
				"Content-Type":    []string{"application/octet-stream"},
				"X-Secret":        []string{secret[:8]},
				"Accept-Encoding": []string{"identity"},
			}
		}
	}
	client.requestTemplate = tmp
	client.URL = u
	return
}
func (c *Client) next() (h http.Header, iv []byte) {
	if c.Secure {
		h = c.requestTemplate.Header.Clone()
		iv = util.RandomBytes(16)
		h.Set("X-IV", base64.StdEncoding.EncodeToString(iv))
	} else {
		h = c.requestTemplate.Header
	}
	return
}
func (c *Client) Open(force bool) (*rpc.Client, error) {
	if c.Ws != nil {
		if force {
			c.Ws.Close()
		} else {
			return c.Ws, nil
		}
	}

	alternative := *c.URL
	alternative.Scheme = "ws"

	h, iv := c.next()
	ws, err := websocket.DialConfig(&websocket.Config{
		Location: &alternative,
		Origin:   c.URL,
		Header:   h,
		Version:  websocket.ProtocolVersionHybi13,
	})
	if err != nil {
		return nil, fmt.Errorf("websocket failed with: %v", err)
	} else {
		c.Ws = rpc.NewClientWithCodec(NewClientCodec(ws, iv))
		return c.Ws, nil
	}
}
func (c *Client) Close() {
	if c.Ws != nil {
		c.Ws.Close()
	}
}

func (c *Client) Call(serviceMethod string, reply any, args any) error {
	ws, err := c.Open(false)
	if err != nil && ws != nil {
		err = ws.Call(serviceMethod, args, reply)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			ws, err = c.Open(true)
			if err == nil {
				return ws.Call(serviceMethod, args, reply)
			}
		}
		return err
	}

	var iv []byte
	req := c.requestTemplate.WithContext(context.Background())
	req.Header, iv = c.next()

	// Build request body.
	reqBuffer := bytes.NewBuffer(nil)
	rwc := &RWC{
		Writer: reqBuffer,
		Reader: nil,
		Closer: nil,
	}
	codec := NewClientCodec(rwc, iv)
	defer codec.Close()
	err = codec.WriteRequest(&rpc.Request{
		ServiceMethod: serviceMethod,
		Seq:           1,
	}, args)
	if err != nil {
		return err
	}
	req.Body = io.NopCloser(reqBuffer)
	req.ContentLength = int64(reqBuffer.Len())
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(reqBuffer.Bytes())), nil
	}

	response, err := c.Http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	rwc.Reader, rwc.Closer = response.Body, response.Body
	if response.StatusCode != 200 {
		return fmt.Errorf("HTTP Error %d: %s", response.StatusCode, response.Status)
	}

	var resp rpc.Response
	err = codec.ReadResponseHeader(&resp)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	} else {
		return codec.ReadResponseBody(reply)
	}
}
