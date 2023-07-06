package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"time"

	"strconv"
	"strings"

	flag "github.com/spf13/pflag"
)

var agentpassword string
var socksdebug bool

func main() {
	var (
		listen         string
		tlsCert        string
		socksListen    string
		connect        string
		proxyAddress   string
		dnsListen      string
		dnsDelay       string
		dnsDomain      string
		proxyTimeout   string
		proxyAuth      string
		quiet          bool
		reconnectLimit int
		reconnectDelay int
		debug          bool
		tlsVerifyPeer  bool
		version        bool
	)
	flag.StringVarP(&listen, "listen", "", "", "listen port for receiver address:port")
	flag.StringVarP(&socksListen, "socks", "", "127.0.0.1:1080", "socks address:port")
	flag.StringVarP(&connect, "connect", "", "", "connect address:port")
	flag.StringVarP(&proxyAddress, "proxy", "", "", "proxy address:port")
	flag.StringVarP(&dnsListen, "dns-listen", "", "", "Where should DNS server listen")
	flag.StringVarP(&dnsDelay, "dns-delay", "", "", "Delay/sleep time between requests (200ms by default)")
	flag.StringVarP(&dnsDomain, "dns-connect", "", "", "DNS domain to use for DNS tunneling")
	flag.StringVarP(&proxyTimeout, "proxy-timeout", "", "", "proxy response timeout (ms)")
	flag.StringVarP(&proxyAuth, "proxy-auth", "", "", "proxy auth Domain/user:Password ")
	flag.StringVarP(&agentpassword, "password", "P", "", "Connect password")
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
		fmt.Println("")
		fmt.Println("Usage (standard tcp):")
		fmt.Println("1) Start on the client: revsocks -listen :8080 -socks 127.0.0.1:1080 -pass test")
		fmt.Println("2) Start on the server: revsocks -connect client:8080 -pass test")
		fmt.Println("3) Connect to 127.0.0.1:1080 on the client with any socks5 client.")
		fmt.Println("Usage (dns):")
		fmt.Println("1) Start on the DNS server: revsocks -dns example.com -dnslisten :53 -socks 127.0.0.1:1080")
		fmt.Println("2) Start on the target: revsocks -dns example.com -pass <paste-generated-key>")
		fmt.Println("3) Connect to 127.0.0.1:1080 on the DNS server with any socks5 client.")
	}

	flag.Parse()

	if quiet {
		log.SetOutput(ioutil.Discard)
	}

	if debug {
		socksdebug = true
	}
	if version {
		fmt.Printf("revsocks - reverse socks5 server/client %s (%s)", Version, CommitID)
		os.Exit(0)
	}

	if listen != "" {
		log.Println("Starting to listen for clients")
		if proxyTimeout != "" {
			opttimeout, _ := strconv.Atoi(proxyTimeout)
			proxytout = time.Millisecond * time.Duration(opttimeout)
		} else {
			proxytout = time.Millisecond * 1000
		}

		if agentpassword == "" {
			agentpassword = RandString(64)
			log.Println("No password specified. Generated password is " + agentpassword)
		}

		//listenForSocks(*listen, *certificate)
		log.Fatal(listenForAgents(true, listen, socksListen, tlsCert))
	}

	if connect != "" {
		if proxyTimeout != "" {
			opttimeout, _ := strconv.Atoi(proxyTimeout)
			proxytimeout = time.Millisecond * time.Duration(opttimeout)
		} else {
			proxytimeout = time.Millisecond * 1000
		}

		if proxyAuth != "" {
			if strings.Contains(proxyAuth, "/") {
				domain = strings.Split(proxyAuth, "/")[0]
				username = strings.Split(strings.Split(proxyAuth, "/")[1], ":")[0]
				password = strings.Split(strings.Split(proxyAuth, "/")[1], ":")[1]
			} else {
				username = strings.Split(proxyAuth, ":")[0]
				password = strings.Split(proxyAuth, ":")[1]
			}
			log.Printf("Using domain %s with %s:%s", domain, username, password)
		} else {
			domain = ""
			username = ""
			password = ""
		}

		if agentpassword != "" {
			agentpassword = "RocksDefaultRequestRocksDefaultRequestRocksDefaultRequestRocks!!"
		}

		if userAgent == "" {
			userAgent = "Mozilla/5.0 (Windows NT 6.1; Trident/7.0; rv:11.0) like Gecko"
		}
		//log.Fatal(connectForSocks(*connect,*proxy))
		if reconnectLimit > 0 {
			for i := 1; i <= reconnectLimit; i++ {
				log.Printf("Connecting to the far end. Try %d of %d", i, reconnectLimit)
				error1 := connectForSocks(true, tlsVerifyPeer, connect, proxyAddress)
				log.Print(error1)
				log.Printf("Sleeping for %d sec...", reconnectDelay)
				tsleep := time.Second * time.Duration(reconnectDelay)
				time.Sleep(tsleep)
			}

		} else {
			for {
				log.Printf("Reconnecting to the far end... ")
				error1 := connectForSocks(true, tlsVerifyPeer, connect, proxyAddress)
				log.Print(error1)
				log.Printf("Sleeping for %d sec...", reconnectDelay)
				tsleep := time.Second * time.Duration(reconnectDelay)
				time.Sleep(tsleep)
			}
		}

		log.Fatal("Ending...")
	}

	if dnsDomain != "" {
		dnskey := agentpassword
		if dnskey == "" {
			dnskey = GenerateKey()
			log.Printf("No password specified, generated following (recheck if same on both sides): %s", dnskey)
		}
		if len(dnskey) != 64 {
			fmt.Fprintf(os.Stderr, "Specified key of incorrect size for DNS (should be 64 in hex)\n")
			os.Exit(1)
		}
		if dnsListen != "" {
			ServeDNS(dnsListen, dnsDomain, socksListen, dnskey, dnsDelay)
		} else {
			DnsConnectSocks(dnsDomain, dnskey, dnsDelay)
		}
		log.Fatal("Ending...")
	}

	flag.Usage()
	fmt.Fprintf(os.Stderr, "You must specify a listen port or a connect address\n")
	os.Exit(1)
}
