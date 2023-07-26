package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"

	"golang.org/x/net/proxy"
)

func init() {
	proxy.RegisterDialerType("http", newHTTPProxy)
	proxy.RegisterDialerType("https", newHTTPProxy)
}

type httpProxy struct {
	host      string
	haveAuth  bool
	username  string
	password  string
	forward   proxy.Dialer
	proxyName string
}

func newHTTPProxy(uri *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	s := new(httpProxy)
	s.host = uri.Host
	s.forward = forward
	if uri.User != nil {
		s.haveAuth = true
		s.username = uri.User.Username()
		s.password, _ = uri.User.Password()
	}
	s.proxyName = uri.Host

	return s, nil
}

func (p *httpProxy) Dial(network, addr string) (net.Conn, error) {
	// Dial and create the https client connection.
	conn, err := p.forward.Dial("tcp", p.host)
	if err != nil {
		return nil, err
	}

	// HACK. http.ReadRequest also does this.
	reqURL, err := url.Parse("http://" + addr)
	if err != nil {
		conn.Close()
		return nil, err
	}
	reqURL.Scheme = ""

	req, err := http.NewRequest("CONNECT", reqURL.String(), nil)
	if err != nil {
		conn.Close()
		return nil, err
	}
	req.Close = false
	if p.haveAuth {
		req.SetBasicAuth(p.username, p.password)
		req.Header.Set("Proxy-Authorization", req.Header.Get("Authorization"))
		req.Header.Del("Authorization")
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Proxy-Connection", "Keep-Alive")

	var logger *log.Logger
	if debug {
		logger = log.New(os.Stderr, fmt.Sprintf("[proxy %s] ", p.proxyName), log.LstdFlags)
		req.Write(logger.Writer())
	}
	err = req.Write(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if debug {
		conn = newNetConnSpy(conn, logger)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		// TODO close resp body ?
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		conn.Close()
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		conn.Close()
		return nil, fmt.Errorf("connect server using proxy error, StatusCode [%d]", resp.StatusCode)
	}

	return conn, nil
}
