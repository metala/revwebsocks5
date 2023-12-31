package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
	debug bool
	quiet bool

	listen         string
	tlsKey         string
	tlsCert        string
	socksBind      string
	socksPort      uint16
	connect        string
	proxies        []string
	reconnectLimit int
	reconnectDelay int
	tlsSkipVerify  bool
	password       string
	userAgent      string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "revwebsocks5",
	Short: "reverse SOCKS5 tunnel over WebSocket",
	Long:  "Establishes a reverse tunnel over WebSocket and TLS",
	Example: `0) Generate key and certificate:
    revwebsocks5 keygen --key-out ./tls/server.key --cert-out ./tls/server.crt --dns-name localhost --ip-addr 127.0.0.1
1) Start on host:
    revwebsocks5 server -l :8443 -P SuperSecretPassword --tls-key ./tls/server.key --tls-cert ./tls/server.crt
2) Start on client:
    revwebsocks5 client -c https://localhost:8443 -P SuperSecretPassword --tls-cert ./tls/server.crt
3) Connect to 127.0.0.1:1080 on the host with any SOCKS5 client.
4) Enjoy.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if quiet {
			log.SetOutput(ioutil.Discard)
		}
	},
}

var mooCmd = &cobra.Command{
	Use:    "moo",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("If you connect into the interwebz, the interwebz connects back.")
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Be quiet")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "display debug info")
	rootCmd.AddCommand(mooCmd)
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
