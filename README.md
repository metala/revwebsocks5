# revwebsocks5

Reverse SOCKS5 tunnel over WebSocket with TLS and proxy support  
Forked from <https://github.com/kost/revsocks>

## Use Case

When behind a (L7 inspecting) firewall and/or HTTP proxy and you are unable to set up a VPN (e.g. Wireguard) tunnel.

# Usage
    Establishes a reverse tunnel over WebSocket and TLS

    Usage:
      revwebsocks5 [command]

    Examples:
    0) Generate key and certificate:
        revwebsocks5 keygen --key-out ./tls/server.key --cert-out ./tls/server.crt --dns-name localhost --ip-addr 127.0.0.1
    1) Start on host:
        revwebsocks5 server -l :8443 -P SuperSecretPassword --tls-key ./tls/server.key --tls-cert ./tls/server.crt
    2) Start on client:
        revwebsocks5 client -c https://localhost:8443 -P SuperSecretPassword --tls-cert ./tls/server.crt
    3) Connect to 127.0.0.1:1080 on the host with any SOCKS5 client.
    4) Enjoy.

    Available Commands:
      client      Client connects to server
      completion  Generate the autocompletion script for the specified shell
      help        Help about any command
      keygen      generates a key and certificate
      server      Start a HTTPS server for client agents

    Flags:
      -d, --debug   display debug info
      -h, --help    help for revwebsocks5
      -q, --quiet   Be quiet

    Use "revwebsocks5 [command] --help" for more information about a command.


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
* `github.com/spf13/cobra` - commands and POSIX cli options
* `golang.org/x/net` - proxy and websocket support

# Disclaimer

This software is intended for educational and research purposes only, and should not be used to target or exploit systems and networks without explicit permission from the owner.