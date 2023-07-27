# revwebsocks5

Reverse SOCKS5 tunneler over WebSocket with SSL/TLS and proxy support  
Forked from <https://github.com/kost/revsocks>

## Use Case

When behind a (L7 inspecting) firewall and/or HTTP proxy and you are unable to set up a VPN (e.g. Wireguard) tunnel.

# Usage
## Example
    1) Start on host: revwebsocks5 -l :8443 -P SuperSecretPassword
    2) Start on client: revwebsocks5 -c clientIP:8443 -P SuperSecretPassword
    3) Connect to 127.0.0.1:1080 on the host with any SOCKS5 client.

## Command-line options
    revwebsocks5 - reverse SOCKS5 tunnel over WebSocket
    
      -c, --connect string        connect address:port
      -d, --debug                 display debug info
      -l, --listen string         listen port for receiver address:port
      -P, --password string       Connect password
          --proxy strings         proxy address:port
      -q, --quiet                 Be quiet
          --reconnect-delay int   reconnection delay (default 30)
          --reconnect-limit int   reconnection limit (default 3)
          --socks-bind string     socks5 bind address (default "127.0.0.1")
          --socks-port uint16     socks5 starting port (default 1080)
          --tls-cert string       certificate file
          --tls-verify            verify TLS server
          --user-agent string     User-Agent

# Design
## Overview
The client established a connection multiplexing (yamux) over WebSocket over HTTP+TLS (HTTPS) and starts a SOCKS5 server for every multiplexed connection. The server forwards all SOCKS5 connections through the connection multiplexer.

## Step-by-Step Details
1. The server starts a HTTP server w/ TLS on the specified bind address:port and waits for a WebSocket connection with the correct authentication password.
2. The client connects through a chain of proxies, if any, using the `CONNECT` method, which is required for the TLS.
3. When the client reaches the server, it starts a TLS handshake (with or without TLS peer verification). Once the secure connection is established, the HTTP connection is upgraded to WebSocket connection and followed up by yamux connection multiplexer.
4. After the successful **yamux** over **WebSocket** over **HTTPS** is established, the server starts to listen on SOCKS5 port (likely 1080) specified. If  the port is not available the server finds the first available in the range above the specified port.
5. Every SOCKS5 connection is forwarded over a new yamux session, which creates a corresponding SOCKS5 server on the client's end serving the yamux channel/session.

## Package Dependencies

* `github.com/armon/go-socks5` - SOCKS5 server handling the connections
* `github.com/hashicorp/yamux` - connection multiplexer
* `github.com/refraction-networking/utls` - custom `ClientHello` and prevents TLS fingerprinting
* `github.com/spf13/pflag` - POSIX cli options
* `golang.org/x/net` - proxy and websocket support

# Disclaimer

This software is intended for educational and research purposes only, and should not be used to target or exploit systems and networks without explicit permission from the owner.