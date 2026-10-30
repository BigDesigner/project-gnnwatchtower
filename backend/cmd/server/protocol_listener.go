package main

import (
	"bufio"
	"crypto/tls"
	"net"
	"sync"
	"time"
)

// One existing port; at most 64 short-lived classification workers. No new listener.
type protocolListener struct {
	net.Listener
	ready chan net.Conn
	done  chan struct{}
	once  sync.Once
}
type bufferedConnection struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConnection) Read(b []byte) (int, error) { return c.reader.Read(b) }
func (l *protocolListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.ready:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}
func (l *protocolListener) Close() error {
	var err error
	l.once.Do(func() {
		close(l.done)
		err = l.Listener.Close()
		for {
			select {
			case c := <-l.ready:
				c.Close()
			default:
				return
			}
		}
	})
	return err
}
func newProtocolListener(base net.Listener, config *tls.Config) *protocolListener {
	l := &protocolListener{Listener: base, ready: make(chan net.Conn), done: make(chan struct{})}
	slots := make(chan struct{}, 64)
	go func() {
		for {
			c, e := base.Accept()
			if e != nil {
				l.Close()
				return
			}
			select {
			case slots <- struct{}{}:
			default:
				c.Close()
				continue
			}
			go func() {
				defer func() { <-slots }()
				c.SetReadDeadline(time.Now().Add(3 * time.Second))
				reader := bufio.NewReader(c)
				first, e := reader.Peek(1)
				if e != nil {
					c.Close()
					return
				}
				c.SetReadDeadline(time.Time{})
				var conn net.Conn = &bufferedConnection{c, reader}
				if first[0] == 0x16 {
					conn = tls.Server(conn, config)
				}
				select {
				case l.ready <- conn:
				case <-l.done:
					conn.Close()
				}
			}()
		}
	}()
	return l
}
