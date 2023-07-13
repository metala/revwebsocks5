package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	socks5 "github.com/armon/go-socks5"
	"github.com/hashicorp/yamux"
	"golang.org/x/net/proxy"
	"golang.org/x/net/websocket"
)

type server struct {
	agentpassword string
	socksBind     string
	socksPort     uint16
}

func (s *server) agentHandler(conn *websocket.Conn) {
	agentstr := conn.RemoteAddr().String()
	log.Printf("[%s] Got an agent from %v: ", agentstr, conn.RemoteAddr())
	conn.SetReadDeadline(time.Now().Add(100 * time.Hour))

	//Add connection to yamux
	session, err := yamux.Client(conn, nil)
	if err != nil {
		log.Printf("[%s] Error creating client in yamux for %s: %v", agentstr, conn.RemoteAddr(), err)
		return
	}
	listenForClients(agentstr, s.socksBind, s.socksPort, session)
}

func (s *server) WsHandler() http.HandlerFunc {
	wsHandler := websocket.Handler(s.agentHandler)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authorization") != s.agentpassword {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		wsHandler.ServeHTTP(w, r)
	}
}

// Catches local clients and connects to yamux
func listenForClients(agentstr string, bind string, port uint16, session *yamux.Session) error {
	var ln net.Listener
	var address string
	var err error
	for {
		address = fmt.Sprintf("%s:%d", bind, port)
		log.Printf("[%s] Waiting for SOCKS5 clients on %s", agentstr, address)
		ln, err = net.Listen("tcp", address)
		if err != nil {
			log.Printf("[%s] Error listening on %s: %v", agentstr, address, err)
			port = port + 1
		} else {
			break
		}
	}
	go func() {
		<-session.CloseChan()
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("[%s] Error accepting on %s: %v", agentstr, address, err)
			return err
		}
		if session == nil {
			log.Printf("[%s] Session on %s is nil", agentstr, address)
			conn.Close()
			continue
		}
		log.Printf("[%s] Got client. Opening stream for %s", agentstr, conn.RemoteAddr())

		stream, err := session.Open()
		if err != nil {
			log.Printf("[%s] Error opening stream for %s: %v", agentstr, conn.RemoteAddr(), err)
			return err
		}

		// connect both of conn and stream
		log.Printf("[%s] Forwarding connection for %s", agentstr, conn.RemoteAddr())
		go func() {
			io.Copy(conn, stream)
			conn.Close()
			log.Printf("[%s] Done forwarding conn to stream for %s", agentstr, conn.RemoteAddr())
		}()
		go func() {
			io.Copy(stream, conn)
			stream.Close()
			log.Printf("[%s] Done forwarding stream to conn for %s", agentstr, conn.RemoteAddr())
		}()
	}
}

func connectToServer(connect *url.URL, proxyUrls []*url.URL, verify bool) error {
	server, err := socks5.New(&socks5.Config{})
	if err != nil {
		return err
	}

	var dailer proxy.Dialer = proxy.Direct
	if len(proxyUrls) > 0 {
		for _, u := range proxyUrls {
			d, err := proxy.FromURL(u, dailer)
			if err != nil {
				return err
			}
			dailer = d
		}
	}

	log.Println("Connecting...")
	var conn net.Conn
	if conn, err = dailer.Dial("tcp", connect.Host); err != nil {
		return err
	}

	log.Println("Establishing TLS connection...")
	conntls := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: !verify,
	})
	if err := conntls.Handshake(); err != nil {
		log.Printf("Error connect: %v", err)
		return err
	}
	conn = conntls

	log.Println("Starting client...")
	wsConf := websocket.Config{
		Location: connect,
		Origin:   connect,
		Version:  13,
		Header:   http.Header{"Authorization": []string{agentPassword}},
	}
	wsconn, err := websocket.NewClient(&wsConf, conn)
	if err != nil {
		return err
	}
	log.Println("Starting session...")
	session, err := yamux.Server(wsconn, nil)
	if err != nil {
		return err
	}

	log.Println("Accepting connections to SOCKS5 server...")
	for {
		stream, err := session.Accept()
		if err != nil {
			return err
		}
		log.Println("Forwarding connection to SOCKS5 server...")
		go func() {
			err = server.ServeConn(stream)
			if err != nil {
				log.Println(err)
			}
		}()
	}
}
