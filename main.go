/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
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
	Long: `Establishes a reverse tunnel over WebSocket and TLS

Usage:
1) Start on host: revwebsocks5 server -l :8443 -P SuperSecretPassword
2) Start on client: revwebsocks5 client -c clientIP:8443 -P SuperSecretPassword
3) Connect to 127.0.0.1:1080 on the host with any SOCKS5 client.`,
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Be quiet")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "display debug info")
}
