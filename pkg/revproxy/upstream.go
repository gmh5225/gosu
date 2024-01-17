package revproxy

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"sync/atomic"

	"github.com/can1357/gosu/pkg/clog"
	"github.com/can1357/gosu/pkg/ipc"
)

type Dialer = func(ctx context.Context) (net.Conn, error)
type Upstream struct {
	Name           string
	DialContext    Dialer
	proxy          *httputil.ReverseProxy
	numConnections atomic.Int32
}

func NewUpstream(name string, dialer Dialer) (u *Upstream) {
	u = &Upstream{
		Name:        name,
		DialContext: dialer,
	}
	u.proxy = &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = "http"
			r.URL.Host = r.Host
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return u.DialContext(ctx)
			},
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			c := r.Context()
			if c.Err() != nil || err == context.Canceled {
				return
			}
			clog.FromContext(c).Printf("upstream[%s] error: %s", u.Name, err)
			if rc := c.Value(requestContextKey{}); rc != nil {
				ctx := rc.(*requestContext)
				select {
				case <-c.Done():
					break
				case <-ctx.Lb.RetryBackoff.After():
					ctx.RetryCount++
					ctx.Lb.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "", http.StatusBadGateway)
		},
	}
	return
}
func NewIpcUpstream(name string, address string) (u *Upstream) {
	return NewUpstream(name, func(ctx context.Context) (net.Conn, error) {
		return ipc.DialContext(ctx, address)
	})
}

func (p *Upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.numConnections.Add(1)
	defer p.numConnections.Add(-1)
	p.proxy.ServeHTTP(w, r)
}
