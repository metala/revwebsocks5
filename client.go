package main

import (
	"crypto/x509"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	socks5 "github.com/armon/go-socks5"
	"github.com/hashicorp/yamux"
	tls "github.com/refraction-networking/utls"
	"github.com/spf13/cobra"
	"golang.org/x/net/proxy"
	"golang.org/x/net/websocket"
)

// clientCmd represents the client command
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Client connects to server",
	Long:  `The client connects to the server and acts as an exit node for the tunnel.`,
	Run: func(cmd *cobra.Command, args []string) {
		if quiet {
			log.SetOutput(ioutil.Discard)
		}
		connectUrl, err := url.Parse(connect)
		if err != nil {
			log.Fatal(err)
		}
		if password == "" {
			log.Fatal("missing password")
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
		certPool, err := loadCertPool(tlsCert)
		if err != nil {
			log.Fatal(err)
		}
		for i := 0; i <= reconnectLimit; i++ {
			log.Printf("Connecting to the server. Attempt %d of %d", i, reconnectLimit)
			err := clientConnect(connectUrl, proxyURLs, certPool, tlsSkipVerify)
			if err != nil {
				log.Printf("Failed to connect: %s", err)
			}
			log.Printf("Sleeping for %d sec...", reconnectDelay)
			tsleep := time.Second * time.Duration(reconnectDelay)
			time.Sleep(tsleep)
		}

		log.Fatal("Ending...")
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)

	clientCmd.Flags().StringVarP(&connect, "connect", "c", "", "connect address:port")
	clientCmd.Flags().StringSliceVarP(&proxies, "proxy", "", []string{}, "proxy address:port")
	clientCmd.Flags().StringVarP(&password, "password", "P", "", "Connect password")
	clientCmd.Flags().StringVarP(&userAgent, "user-agent", "", "", "User-Agent")
	clientCmd.Flags().IntVarP(&reconnectLimit, "reconnect-limit", "", 3, "reconnection limit")
	clientCmd.Flags().IntVarP(&reconnectDelay, "reconnect-delay", "", 30, "reconnection delay")
	clientCmd.Flags().StringVarP(&tlsCert, "tls-cert", "", "", "certificate file (defaults to system certificates)")
	clientCmd.Flags().BoolVarP(&tlsSkipVerify, "tls-skip-verify", "", false, "verify TLS server")

	clientCmd.MarkFlagsRequiredTogether("connect", "password")
}

func clientConnect(connect *url.URL, proxyUrls []*url.URL, certPool *x509.CertPool, skipVerify bool) error {
	socksHandler, err := socks5.New(&socks5.Config{})
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

	log.Println("Dialling...")
	var conn net.Conn
	if conn, err = dailer.Dial("tcp", connect.Host); err != nil {
		return err
	}
	if debug && len(proxyUrls) == 0 {
		logger := log.New(os.Stderr, "[conn raw] ", log.LstdFlags)
		conn = newNetConnSpy(conn, logger)
	}

	log.Println("Establishing TLS connection...")
	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		RootCAs:            certPool,
		InsecureSkipVerify: skipVerify,
		ServerName:         connect.Hostname(),
		NextProtos:         []string{"h2", "http/1.1"},
	}
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
			"Authorization": []string{password},
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
			err = socksHandler.ServeConn(stream)
			if err != nil {
				log.Println(err)
			}
		}()
	}
}

func loadCertPool(filename string) (*x509.CertPool, error) {
	if filename == "" {
		return x509.SystemCertPool()
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	certs := x509.NewCertPool()
	ok := certs.AppendCertsFromPEM(data)
	if !ok {
		return nil, errors.New("failed to parse RootCAs")
	}
	return certs, nil
}
