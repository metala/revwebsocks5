package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/hashicorp/yamux"
	tls "github.com/refraction-networking/utls"
	"github.com/spf13/cobra"
	"golang.org/x/net/websocket"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a HTTPS server for client agents",
	Long: `The server commands stars a WebSocket HTTPS service that waits for 
client agents to establish a reverse tunnel.`,
	Run: func(cmd *cobra.Command, args []string) {
		if password == "" {
			password = RandString(64)
			log.Println("No password specified. Generated password is " + password)
		}
		srv := server{
			agentpassword: password,
			socksBind:     socksBind,
			socksPort:     socksPort,
		}
		wsSrv := &http.Server{
			Handler:      srv.WsHandler(),
			Addr:         listen,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			ErrorLog:     log.Default(),
			ConnState: func(c net.Conn, cs http.ConnState) {
				if !debug {
					return
				}
				log.Printf("[%s] state: %s", c.RemoteAddr(), cs)
			},
		}
		cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
		if err != nil {
			log.Fatal(err)
		}
		tlsCfg := &tls.Config{
			MinVersion:   tls.VersionTLS12,
			MaxVersion:   tls.VersionTLS13,
			Certificates: []tls.Certificate{cert},
		}

		log.Printf("Listening for agents on %s using TLS", listen)
		ln, err := net.Listen("tcp", listen)
		if err != nil {
			log.Fatal(err)
		}
		tlsLn := tls.NewListener(ln, tlsCfg)
		if err := wsSrv.Serve(tlsLn); err != nil {
			panic("ListenAndServe: " + err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().StringVarP(&listen, "listen", "l", "0.0.0.0:8443", "listen port for receiver address:port")
	serverCmd.Flags().StringVarP(&socksBind, "socks-bind", "", "127.0.0.1", "socks5 bind address")
	serverCmd.Flags().Uint16VarP(&socksPort, "socks-port", "", 1080, "SOCKS5 starting port")
	serverCmd.Flags().StringVarP(&connect, "connect", "c", "", "connect address:port")
	serverCmd.Flags().StringSliceVarP(&proxies, "proxy", "", []string{}, "proxy address:port")
	serverCmd.Flags().StringVarP(&password, "password", "P", "", "Connect password")
	serverCmd.Flags().StringVarP(&userAgent, "user-agent", "", "", "User-Agent")
	serverCmd.Flags().StringVarP(&tlsKey, "tls-key", "", "", "TLS key file")
	serverCmd.Flags().StringVarP(&tlsCert, "tls-cert", "", "", "TLS certificate file")

	serverCmd.MarkFlagsRequiredTogether("tls-key", "tls-cert")
}

type server struct {
	agentpassword string
	socksBind     string
	socksPort     uint16
}

func (s *server) agentHandler(conn *websocket.Conn) {
	agentstr := conn.Request().RemoteAddr
	log.Printf("[%s] Agent connected.", agentstr)
	conn.SetReadDeadline(time.Now().Add(100 * time.Hour))

	session, err := yamux.Client(conn, nil)
	if err != nil {
		log.Printf("[%s] Error creating client in yamux for %s: %v", agentstr, conn.RemoteAddr(), err)
		return
	}
	s.listenForSocks5Clients(agentstr, session)
}

func (s *server) WsHandler() http.HandlerFunc {
	wsAgentHandler := websocket.Handler(s.agentHandler)
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] New agent negotiation.", r.RemoteAddr)
		if r.Header.Get("authorization") != s.agentpassword {
			if debug {
				log.Printf("[%s] Error: Invalid password", r.RemoteAddr)
			}
			w.WriteHeader(http.StatusForbidden)
			return
		}
		wsAgentHandler.ServeHTTP(w, r)
	}
}

func (s *server) listenForSocks5Clients(agentstr string, session *yamux.Session) error {
	var ln net.Listener
	var address string
	var err error

	port := s.socksPort
	for {
		address = fmt.Sprintf("%s:%d", s.socksBind, port)
		log.Printf("[%s] Waiting for SOCKS5 clients on %s for %s", agentstr, address, agentstr)
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
