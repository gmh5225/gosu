package revproxy

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/can1357/gosu/pkg/clog"
	"github.com/can1357/gosu/pkg/util"
	"github.com/samber/lo"
)

type LbMethod string

const (
	LbConn   LbMethod = "conn"
	LbRandom LbMethod = "random"
	LbHash   LbMethod = "hash"
)

type Options struct {
	Host         string                `json:"host"`        // The host header to set.
	Listen       string                `json:"listen"`      // The listen address.
	Sticky       bool                  `json:"sticky"`      // If true, the same upstream is chosen for the same client if possible.
	Method       LbMethod              `json:"method"`      // The load balancing method.
	RetryMax     int                   `json:"retry_max"`   // The maximum number of retries.
	RetryBackoff util.ParsableDuration `json:"retry_delay"` // The delay between retries.
}

type requestContextKey struct{}
type requestContext struct {
	Ip         string
	Lb         *LoadBalancer
	RetryCount int
	Previous   *Upstream
}

type ClientSession struct {
	upstream atomic.Pointer[Upstream]
}

type LoadBalancer struct {
	Options
	Upstreams []*Upstream
	mu        sync.RWMutex
	server    *http.Server
	sessions  sync.Map //map[string]*ClientSession
}

func NewLoadBalancer(opt Options) (lb *LoadBalancer) {
	lb = &LoadBalancer{Options: opt}
	lb.server = &http.Server{Addr: opt.Listen, Handler: lb}
	return
}

func (lb *LoadBalancer) AddUpstream(u *Upstream) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.Upstreams = append(lb.Upstreams, u)
}
func (lb *LoadBalancer) RemoveUpstream(u *Upstream) {
	lb.mu.Lock()
	lb.Upstreams = lo.Without(lb.Upstreams, u)
	lb.mu.Unlock()
	lb.sessions.Range(func(key, value any) bool {
		if value == u {
			lb.sessions.Delete(key)
		}
		return true
	})
}

func (lb *LoadBalancer) Next(ip string, retry *Upstream) (us *Upstream) {
	// If sticky, get the session.
	//
	var session *ClientSession
	if lb.Sticky {
		if s, ok := lb.sessions.Load(ip); ok {
			session = s.(*ClientSession)
		} else {
			session = &ClientSession{}
			if actual, loaded := lb.sessions.LoadOrStore(ip, session); loaded {
				session = actual.(*ClientSession)
			}
		}

		// Also defer the session update.
		defer func() {
			if us != nil {
				for {
					v := session.upstream.Load()
					if v != nil && v != retry {
						us = v
						return
					} else if !session.upstream.CompareAndSwap(v, us) {
						continue
					} else {
						break
					}
				}
				fmt.Printf("sticky session[%s]=%v\n", ip, us)
			}
		}()
	}

	// If sticky, try the sticky session first.
	//
	if session != nil {
		if u := session.upstream.Load(); u != nil {
			if u == retry {
				session.upstream.CompareAndSwap(retry, nil)
			} else {
				return u
			}
		}

	}

	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// If there are no upstreams, return nil, if there is one upstream, return it.
	if len(lb.Upstreams) == 0 {
		return nil
	} else if len(lb.Upstreams) == 1 {
		return lb.Upstreams[0]
	}

	var up *Upstream
	if lb.Method == LbConn {
		up = lo.MinBy(lb.Upstreams, func(a, b *Upstream) bool {
			if a == retry {
				return false
			} else if b == retry {
				return true
			}
			return a.numConnections.Load() < b.numConnections.Load()
		})
		if up == retry {
			return nil
		}
	} else {
		var n int
		if lb.Method == LbHash {
			n = fastHash(ip) % len(lb.Upstreams)
		} else {
			n = rand.Intn(len(lb.Upstreams))
		}
		up = lb.Upstreams[n]
		if up == retry {
			for i := 1; i < len(lb.Upstreams); i++ {
				up = lb.Upstreams[(n+i)%len(lb.Upstreams)]
				if up != retry {
					break
				}
			}
		}
	}
	return up
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var ctx *requestContext
	if v := r.Context().Value(requestContextKey{}); v != nil {
		ctx = v.(*requestContext)
		if ctx.RetryCount >= lb.RetryMax {
			clog.FromContext(r.Context()).Printf("retry count exceeded")
			http.Error(w, "", http.StatusBadGateway)
			return
		}
	} else {
		ctx = &requestContext{Ip: getClientIp(r), Lb: lb, Previous: nil}
		r = r.WithContext(context.WithValue(r.Context(), requestContextKey{}, ctx))
	}

	r.Header.Set("CF-Connecting-IP", ctx.Ip)
	r.Header.Set("X-Forwarded-For", ctx.Ip)
	if lb.Host != "" {
		r.Host = lb.Host
	}

	us := lb.Next(ctx.Ip, ctx.Previous)
	if us != nil {
		ctx.Previous = us
		us.ServeHTTP(w, r)
	} else {
		http.Error(w, "", http.StatusBadGateway)
	}
}
func (lb *LoadBalancer) Listen() error {
	return lb.server.ListenAndServe()
}
func (lb *LoadBalancer) Close() {
	lb.server.Close()
}
