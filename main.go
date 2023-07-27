package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"

	"time"

	tls "github.com/refraction-networking/utls"
	flag "github.com/spf13/pflag"
)

var agentPassword string
var userAgent string

func main() {
	var (
		listen         string
		tlsCert        string
		socksBind      string
		socksPort      uint16
		connect        string
		proxies        []string
		quiet          bool
		reconnectLimit int
		reconnectDelay int
		tlsVerify      bool
	)
	flag.StringVarP(&listen, "listen", "l", "", "listen port for receiver address:port")
	flag.StringVarP(&socksBind, "socks-bind", "", "127.0.0.1", "socks5 bind address")
	flag.Uint16VarP(&socksPort, "socks-port", "", 1080, "socks5 starting port")
	flag.StringVarP(&connect, "connect", "c", "", "connect address:port")
	flag.StringSliceVarP(&proxies, "proxy", "", []string{}, "proxy address:port")
	flag.StringVarP(&agentPassword, "password", "P", "", "Connect password")
	flag.StringVarP(&userAgent, "user-agent", "", "", "User-Agent")
	flag.IntVarP(&reconnectLimit, "reconnect-limit", "", 3, "reconnection limit")
	flag.IntVarP(&reconnectDelay, "reconnect-delay", "", 30, "reconnection delay")
	flag.StringVarP(&tlsCert, "tls-cert", "", "", "certificate file")
	flag.BoolVarP(&tlsVerify, "tls-verify", "", false, "verify TLS server")
	flag.BoolVarP(&quiet, "quiet", "q", false, "Be quiet")
	flag.BoolVarP(&debug, "debug", "d", false, "display debug info")

	flag.Usage = func() {
		fmt.Println("revwebsocks5 - reverse SOCKS5 tunnel over WebSocket")
		fmt.Println("")
		flag.PrintDefaults()
		fmt.Print(`
Usage:
1) Start on host: revwebsocks5 -l :8443 -P SuperSecretPassword
2) Start on client: revwebsocks5 -c clientIP:8443 -P SuperSecretPassword
3) Connect to 127.0.0.1:1080 on the host with any SOCKS5 client.
`)
	}

	flag.Parse()

	if quiet {
		log.SetOutput(ioutil.Discard)
	}

	if listen != "" {
		if agentPassword == "" {
			agentPassword = RandString(64)
			log.Println("No password specified. Generated password is " + agentPassword)
		}
		srv := server{
			agentpassword: agentPassword,
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
		tlsCfg, err := getTlsPairConfig(tlsCert)
		if err != nil {
			log.Fatal(err)
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
	}

	if connect != "" {
		connectUrl, err := url.Parse(connect)
		if err != nil {
			log.Fatal(err)
		}

		if agentPassword == "" {
			agentPassword = "RocksDefaultRequestRocksDefaultRequestRocksDefaultRequestRocks!!"
		}
		if userAgent == "" {
			userAgent = "curl/8.1.2"
		}
		proxyURLs := make([]*url.URL, 0)
		for _, pu := range proxies {
			u, err := url.Parse(pu)
			if err != nil {
				log.Fatalf("Invalid proxy '%s': %s", pu, err)
			}
			proxyURLs = append(proxyURLs, u)
		}

		for i := 0; i <= reconnectLimit; i++ {
			log.Printf("Connecting to the far end. Try %d of %d", i, reconnectLimit)
			err := connectToServer(connectUrl, proxyURLs, tlsCert, tlsVerify)
			if err != nil {
				log.Printf("Failed to connect: %s", err)
			}
			log.Printf("Sleeping for %d sec...", reconnectDelay)
			tsleep := time.Second * time.Duration(reconnectDelay)
			time.Sleep(tsleep)
		}

		log.Fatal("Ending...")
	}

	flag.Usage()
	fmt.Fprintf(os.Stderr, "You must specify a listen port or a connect address\n")
	os.Exit(1)
}
