package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"

	"time"

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
		debug          bool
		tlsVerifyPeer  bool
		version        bool
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
	flag.StringVarP(&tlsCert, "cert", "", "", "certificate file")
	flag.BoolVarP(&tlsVerifyPeer, "verify-peer", "", false, "verify TLS peer")
	flag.BoolVarP(&quiet, "quiet", "q", false, "Be quiet")
	flag.BoolVarP(&debug, "debug", "", false, "display debug info")
	flag.BoolVarP(&version, "version", "", false, "show program version")

	flag.Usage = func() {
		fmt.Printf("revsocks - reverse socks5 server/client %s (%s)", Version, CommitID)
		fmt.Println("")
		flag.PrintDefaults()
		fmt.Print(`
Usage (standard tcp):
1) Start on the client: revsocks -listen :8080 -socks 127.0.0.1:1080 -pass test
2) Start on the server: revsocks -connect client:8080 -pass test
3) Connect to 127.0.0.1:1080 on the client with any socks5 client.
Usage (dns):
1) Start on the DNS server: revsocks -dns example.com -dnslisten :53 -socks 127.0.0.1:1080
2) Start on the target: revsocks -dns example.com -pass <paste-generated-key>
3) Connect to 127.0.0.1:1080 on the DNS server with any socks5 client.
`)
	}

	flag.Parse()

	if quiet {
		log.SetOutput(ioutil.Discard)
	}

	if version {
		fmt.Printf("%s - reverse SOCKS5 server/client over ws %s (%s)", os.Args[0], Version, CommitID)
		os.Exit(0)
	}

	if listen != "" {
		log.Println("Starting to listen for clients")

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
		}
		tlsCfg, err := getTlsConfig(tlsCert)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Listening for agents on %s using TLS", listen)
		ln, err := net.Listen("tcp", listen)
		if err != nil {
			log.Fatal(err)
		}
		tlsLn := tls.NewListener(ln, tlsCfg)
		if err = wsSrv.Serve(tlsLn); err != nil {
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
			userAgent = "Mozilla/5.0 (Windows NT 6.1; Trident/7.0; rv:11.0) like Gecko"
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
			err := connectToServer(connectUrl, proxyURLs, tlsVerifyPeer)
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
