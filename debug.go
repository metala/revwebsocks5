package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

type rwcSpy struct {
	io.ReadWriteCloser

	logger *log.Logger
}

func newRwcSpy(rwc io.ReadWriteCloser, l *log.Logger) rwcSpy {
	return rwcSpy{rwc, l}
}

func (s *rwcSpy) Read(p []byte) (n int, err error) {
	n, err = s.ReadWriteCloser.Read(p)
	s.logger.Printf("Read(): err=%s, n=%d, p <=", err, n)
	xxd(bytes.NewReader(p[:n]), s.logger.Writer())
	return n, err
}

func (s *rwcSpy) Write(p []byte) (n int, err error) {
	n, err = s.ReadWriteCloser.Write(p)
	s.logger.Printf("Write(): err=%s, n=%d, p =>", err, n)
	buf := bytes.NewBuffer(nil)
	xxd(bytes.NewReader(p), buf)
	s.logger.Writer().Write(buf.Bytes())
	return n, err
}

func (s *rwcSpy) Close() error {
	err := s.ReadWriteCloser.Close()
	s.logger.Printf("Close(): err=%s", err)
	return err
}

type netConnSpy struct {
	rwcSpy

	conn net.Conn
}

func newNetConnSpy(conn net.Conn, l *log.Logger) netConnSpy {
	return netConnSpy{newRwcSpy(conn, l), conn}
}

func (s netConnSpy) Read(p []byte) (n int, err error) {
	return s.rwcSpy.Read(p)
}

func (s netConnSpy) Write(p []byte) (n int, err error) {
	return s.rwcSpy.Write(p)
}

func (s netConnSpy) Close() error {
	return s.rwcSpy.Close()
}

func (s netConnSpy) LocalAddr() net.Addr {
	result := s.conn.LocalAddr()
	s.rwcSpy.logger.Printf("LocalAddr(): result=%s", result)
	return result
}

func (s netConnSpy) RemoteAddr() net.Addr {
	result := s.conn.RemoteAddr()
	s.rwcSpy.logger.Printf("RemoteAddr(): result=%s", result)
	return result
}

func (s netConnSpy) SetDeadline(t time.Time) error {
	err := s.conn.SetDeadline(t)
	s.rwcSpy.logger.Printf("SetDeadline(): t=%s, err=%s", t, err)
	return err
}

func (s netConnSpy) SetReadDeadline(t time.Time) error {
	err := s.conn.SetReadDeadline(t)
	s.rwcSpy.logger.Printf("SetReadDeadline(): t=%s, err=%s", t, err)
	return err
}

func (s netConnSpy) SetWriteDeadline(t time.Time) error {
	err := s.conn.SetWriteDeadline(t)
	s.rwcSpy.logger.Printf("SetWriteDeadline(): t=%s, err=%s", t, err)
	return err
}

func xxd(r io.Reader, w io.Writer) error {
	base := 0
	for {
		xb := strings.Builder{}
		xb.WriteString(fmt.Sprintf("%08x: ", base))

		var b [16]byte
		n, err := io.ReadFull(r, b[:])
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return err
		}

		for i := 0; i < n; i++ {
			xb.WriteString(fmt.Sprintf("%02x ", b[i]))
			if b[i] < 32 || b[i] >= 127 {
				b[i] = '.'
			}
		}
		for i := n; i < 16; i++ {
			xb.WriteString("   ")
		}

		xb.WriteByte(' ')
		xb.Write(b[:n])
		xb.WriteByte('\n')
		w.Write([]byte(xb.String()))
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		base += 16
	}
	return nil
}
