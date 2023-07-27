package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	socks5 "github.com/armon/go-socks5"
	"github.com/hashicorp/yamux"
	tls "github.com/refraction-networking/utls"
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
		log.Printf("[%s] New agent connection.", r.RemoteAddr)
		if r.Header.Get("authorization") != s.agentpassword {
			log.Printf("[%s] Error: Invalid password", r.RemoteAddr)
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

func connectToServer(connect *url.URL, proxyUrls []*url.URL, tlsCert string, verify bool) error {
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
	if debug && len(proxyUrls) == 0 {
		logger := log.New(os.Stderr, "[conn raw] ", log.LstdFlags)
		conn = newNetConnSpy(conn, logger)
	}

	log.Println("Establishing TLS connection...")
	tlsCfg, err := getTlsCertConfig(tlsCert)
	if err != nil {
		return err
	}
	tlsCfg.InsecureSkipVerify = !verify
	tlsCfg.MinVersion = tls.VersionTLS12
	tlsCfg.MaxVersion = tls.VersionTLS13
	tlsCfg.ServerName = connect.Hostname()
	tlsCfg.NextProtos = []string{"h2", "http/1.1"}
	conntls := tls.UClient(conn, tlsCfg, tls.HelloCustom)
	conntls.ApplyPreset(&tls.ClientHelloSpec{
		TLSVersMin: tls.VersionTLS12,
		TLSVersMax: tls.VersionTLS13,
		CipherSuites: []uint16{
			0x1302, 0x1303, 0x1301, 0xc02c, 0xc030, 0x009f, 0xcca9, 0xcca8,
			0xccaa, 0xc02b, 0xc02f, 0x009e, 0xc024, 0xc028, 0x006b, 0xc023,
			0xc027, 0x0067, 0xc014, 0x0039, 0xc009, 0xc013, 0x0033, 0x009d,
			0x009d, 0x009c, 0x003d, 0x003c, 0x0035, 0x002f, 0x00ff,
		},
		CompressionMethods: []uint8{0},
		Extensions: []tls.TLSExtension{
			&tls.SNIExtension{ServerName: connect.Hostname()},
			&tls.SupportedPointsExtension{SupportedPoints: []uint8{0, 1, 2}},
			&tls.SupportedCurvesExtension{Curves: []tls.CurveID{
				/* x25519 */ 0x001d /* secp256r1 */, 0x0017 /* x448 */, 0x001e /*secp521r1*/, 0x0019,
				0x0018, 0x0100, 0x0101, 0x0102, 0x0103, 0x0104,
			}},
			&tls.ALPNExtension{AlpnProtocols: []string{"h2", "http/1.1"}},
			&tls.GenericExtension{Id: 22},
			&tls.UtlsExtendedMasterSecretExtension{},
			&tls.GenericExtension{Id: 49},
			&tls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []tls.SignatureScheme{
				0x0403, 0x0503, 0x0603, 0x0807, 0x0808, 0x0809, 0x080a, 0x080b,
				0x0804, 0x0805, 0x0806, 0x0401, 0x0501, 0x0601, 0x0303, 0x0301,
				0x0302, 0x0402, 0x0502, 0x0602,
			}},
			&tls.SupportedVersionsExtension{Versions: []uint16{
				tls.VersionTLS13, tls.VersionTLS12,
			}},
			&tls.PSKKeyExchangeModesExtension{Modes: []uint8{
				tls.PskModeDHE,
			}},
			&tls.KeyShareExtension{
				KeyShares: []tls.KeyShare{
					{Group: tls.X25519},
				},
			},
			&tls.UtlsPaddingExtension{PaddingLen: 174, WillPad: true},
		},
	})
	if err := conntls.Handshake(); err != nil {
		log.Printf("Error connect: %v", err)
		return err
	}
	conn = conntls
	if debug {
		logger := log.New(os.Stderr, "[conn] ", log.LstdFlags)
		conn = newNetConnSpy(conn, logger)
	}
	rwc := io.ReadWriteCloser(conn)

	log.Println("Starting tunnel client...")
	wsConf := websocket.Config{
		Location: connect,
		Origin:   connect,
		Version:  13,
		Header: http.Header{
			"User-Agent":    []string{userAgent},
			"Authorization": []string{agentPassword},
			"Connection":    []string{"Upgrade"},
		},
	}
	wsconn, err := websocket.NewClient(&wsConf, rwc)
	if err != nil {
		return err
	}
	log.Println("Starting tunnel session...")
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
		log.Println("Serving new SOCKS5 connection...")
		go func() {
			err = server.ServeConn(stream)
			if err != nil {
				log.Println(err)
			}
		}()
	}
}
