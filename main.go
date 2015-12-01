package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

func main() {
	// Flags
	var (
		source, dest string
		delay        time.Duration
	)
	flag.StringVar(&source, "source", ":2000", "the source address to listen on")
	flag.StringVar(&dest, "dest", ":3000", "the destination address to send to")
	flag.DurationVar(&delay, "delay", time.Millisecond*100, "the delay to inject")
	flag.Parse()

	// Source listener
	ln, err := net.Listen("tcp", source)
	if err != nil {
		log.Fatalf("unable to listen on %s, %s", source, err)
	}
	log.Printf("listening on %s", source)
	defer func() {
		if err := ln.Close(); err != nil {
			log.Fatalf("unable to close listener, %s", err)
		}
	}()

	// Accept connections
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("unable to accept connection, %s", err)
		}
		go func(conn net.Conn) {
			if err := handleConn(conn, dest, delay); err != nil {
				log.Printf("unable to handle connection, %s", err)
			}
		}(conn)
	}
}

type delayBuf struct {
	buf []byte
	n   int
	at  time.Time
	err error
}

func newDelayBuf() delayBuf {
	return delayBuf{
		buf: make([]byte, 1024),
	}
}

// handleConn proxies data from an accepted connection to the dest with a delay
func handleConn(conn net.Conn, dest string, delay time.Duration) error {
	destConn, err := net.Dial("tcp", dest)
	if err != nil {
		return fmt.Errorf("unable to dial %s, %s", dest, err)
	}

	dataChan := make(chan delayBuf, 1024)
	errChan := make(chan error)

	go func() {
		// Read from source
		for {
			db := newDelayBuf()
			db.n, db.err = conn.Read(db.buf)
			db.at = time.Now()
			dataChan <- db
			if db.err != nil {
				// Stop reading
				return
			}
		}
	}()

	go func() {
		// Write to dest
		for {
			db := <-dataChan
			if db.n > 0 {
				diff := time.Now().Sub(db.at)
				if diff < delay {
					// Wait until delay
					time.Sleep(delay - diff)
				}
				if _, err := destConn.Write(db.buf[:db.n]); err != nil {
				}
			}
			if db.err != nil {
				if db.err != io.EOF {
					errChan <- db.err
					destConn.Close()
				} else {
					errChan <- destConn.Close()
				}
				return
			}
		}
	}()

	return <-errChan
}
