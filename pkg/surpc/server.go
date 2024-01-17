package surpc

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strings"

	"github.com/can1357/gosu/pkg/settings"
	"github.com/gorilla/mux"
	"golang.org/x/net/websocket"
)

type Server struct {
	RpcServer  *rpc.Server
	HttpServer *http.Server
	Router     *mux.Router
	listeners  []net.Listener
}

func (s *Server) Authorize(w http.ResponseWriter, r *http.Request) (secure []byte, ok bool) {
	if secret := r.Header.Get("X-Secret"); secret == "" {
		ip := net.ParseIP(strings.Split(r.RemoteAddr, ":")[0])
		if ip == nil || !ip.IsLoopback() {
			http.Error(w, "Forbidden", 444)
			return
		}
	} else {
		settings := settings.Rpc.Get()
		if settings.Secret != "" {
			if settings.Secret[:8] != secret[:8] {
				http.Error(w, "Forbidden", 444)
				return
			}
			if iv, err := base64.StdEncoding.DecodeString(r.Header.Get("X-IV")); err != nil || len(iv) != 16 {
				http.Error(w, "Forbidden", 444)
				return
			} else {
				secure = iv
			}
		}
	}
	ok = true
	return
}

func (s *Server) RpcHandler(w http.ResponseWriter, r *http.Request) {
	secure, ok := s.Authorize(w, r)
	if !ok {
		return
	}

	if r.Method != "POST" {
		s := websocket.Server{
			Handler: func(c *websocket.Conn) {
				s.RpcServer.ServeCodec(NewServerCodec(c, secure))
			},
			Handshake: func(c *websocket.Config, r *http.Request) error {
				if r.Method != "GET" {
					return fmt.Errorf("websocket: method not GET")
				}
				return nil
			},
		}
		s.ServeHTTP(w, r)
		return
	}

	buffer := bytes.NewBuffer(nil)
	var rwc io.ReadWriteCloser = &RWC{
		Reader: r.Body,
		Writer: buffer,
		Closer: r.Body,
	}
	serverCodec := NewServerCodec(rwc, secure)
	defer serverCodec.Close()

	if secure != nil {
		w.Header().Set("Content-type", "application/json")
	} else {
		w.Header().Set("Content-type", "application/octet-stream")
	}

	err := s.RpcServer.ServeRequest(serverCodec)
	if err != nil {
		log.Printf("Error while serving JSON request: %v", err)
		if buffer.Len() == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buffer.Len()))
	w.WriteHeader(http.StatusOK)
	w.Write(buffer.Bytes())
}

func NewServer() (s *Server) {
	s = &Server{
		RpcServer:  rpc.NewServer(),
		HttpServer: &http.Server{},
		Router:     mux.NewRouter(),
	}
	s.Router.HandleFunc("/rpc", s.RpcHandler)
	s.HttpServer.Handler = s.Router
	return
}
func (s *Server) Register(name string, receiver any) error {
	return s.RpcServer.RegisterName(name, receiver)
}
func (s *Server) Serve(listener net.Listener) error {
	err := s.HttpServer.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
func (s *Server) ListenTo(network string, address string) error {
	listener, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	s.listeners = append(s.listeners, listener)
	go func() error {
		log.Printf("HTTPD (%s,%s) listening", network, address)
		err = s.Serve(listener)
		if err != nil {
			log.Printf("HTTPD (%s,%s) error: %s", network, address, err)
		}
		return err
	}()
	return nil
}

func (s *Server) Listen(address string) error {
	if strings.HasPrefix(address, "http://") {
		return s.ListenTo("tcp", address[7:])
	} else {
		return fmt.Errorf("invalid address: %s", address)
	}
}
func (s *Server) ListenAll() (e error) {
	for _, address := range settings.Rpc.Get().Addresses() {
		e = s.Listen(address)
		if e != nil {
			break
		}
	}
	return
}
func (s *Server) Close() {
	for _, listener := range s.listeners {
		listener.Close()
	}
	if s.HttpServer != nil {
		s.HttpServer.Close()
	}
}
