package main

import (
	"io"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// handler functions for http.Handle()

func valid(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, ".")
}
func notadot(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "this does not start with '.'")
}
func empty(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "")
}

var hasFlipped bool = false

func flipping(w http.ResponseWriter, req *http.Request) {
	if hasFlipped {
		valid(w, req)
		return
	}
	empty(w, req)
	hasFlipped = true
}

// testFatal is a helper function. It runs a subtest by starting go
// test again, this time using -test.run flag and setting an
// environment variable.
// This way of testing functions ending in exit(1) is found here:
// http://stackoverflow.com/questions/30688554/how-to-test-go-function-containing-log-fatal
func testFatal(envvar string, f func(), regexp string, t *testing.T) {
	if os.Getenv(envvar) == "1" {
		f()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run="+regexp)
	cmd.Env = append(os.Environ(), envvar+"=1")
	err = cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("process ran with err %v, want exit status 1", err)
}

var sendMailServer = `220 hello world
502 EH?
250 mx.google.com at your service
250 Sender ok
250 Receiver ok
354 Go ahead
250 Data ok
221 Goodbye
`
var serverChat = strings.Split(sendMailServer, "\n")

// mailServer is a stub smtp server for testing.
// Honoustly stolen from smtp_test.go
func mailServer(addr string, t *testing.T) {

	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Unable to to create listener: %v", err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			t.Errorf("Accept error: %v", err)
			return
		}
		handleOne(conn, t)
		conn.Close()
	}
}

func handleOne(c net.Conn, t *testing.T) {

	tc := textproto.NewConn(c)

	for i := 0; i < len(serverChat); i++ {
		// serverChat is the slice of server replies
		tc.PrintfLine(serverChat[i])

		if serverChat[i] == "221 Goodbye" {
			return
		}

		reading := false
	body:
		for !reading || serverChat[i] == "354 Go ahead" {
			msg, err := tc.ReadLine()
			reading = true
			if err != nil {
				t.Errorf("Read error: %v", err)
				return
			}
			if serverChat[i] == "354 Go ahead" && msg == "." {
				break body
			}
		}
	}
}
