package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ajaapps/polldot/config"
)

// initTest makes a clean test environment by:
// (1) setting the HOME environment variable to 'testdata'
// (2) setting cfg to a sane test value where the testservers are
// being used
// (3) create a corresponding configuration file
// (4) create an empty log file
// Test servers are started from init() in polldot_test.go.
func initTest() {
	err = os.Setenv("HOME", "testdata")
	if err != nil {
		log.Fatal(err)
	}

	cfg = testCfg()

	// TODO create .polldot.json from cfg

	logfile := os.Getenv("HOME") + "/polldot.log"
	os.Remove(logfile)
	os.Truncate(logfile, 0)
	if err != nil {
		log.Fatal(err)
	}

}

// testCfg returns a complete cfg variable with testing values
func testCfg() *config.Config {
	return &config.Config{
		URL:       "http://127.0.0.1:8080/valid",
		From:      "root@localhost",
		To:        "root@localhost",
		Subject:   "mail from polldot go test",
		Body:      "test run at " + time.Now().String(),
		Host:      "127.0.0.1",
		Port:      2525,
		CycleLen:  10,
		CycleUnit: "seconds",
		Sleep:     time.Second * 10,
	}
}

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

// mailServer is a stub smtp server for testing.
// Honoustly stolen from smtp_test.go
func mailServer(addr string) {

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("Unable to to create listener: %v", err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		//log.Println(" *****     new conn *****") //TODO remove
		if err != nil {
			log.Fatal("Accept error: %v", err)
		}
		handleOne(conn)
		//log.Println(" ***** closing conn *****") //TODO remove
		conn.Close()
	}
}

func handleOne(c net.Conn) {

	tc := textproto.NewConn(c)

	for i := 0; i < len(serverChat); i++ {

		tc.PrintfLine(serverChat[i])
		//log.Println("S", i, serverChat[i])

		if serverChat[i] == "221 Goodbye" {
			return
		}

		var msg string

		if serverChat[i] == "354 Go ahead" {
			// read body
			for {
				msg, _ = tc.ReadLine() // TODO err handling
				//		log.Println("C  ", msg)
				if msg == "." {
					i = 6
					tc.PrintfLine(serverChat[i])
					//log.Println("S", i, serverChat[i])
					break
				}
			}
		}

		msg, _ = tc.ReadLine() // TODO err handling
		//log.Println("C  ", msg)

		if strings.Contains(msg, "QUIT") {
			i = 6
		}
	}
}

var serverChat = [8]string{
	"220 hello world",
	"502 EH?",
	"250 mx.google.com at your service",
	"250 Sender ok",
	"250 Receiver ok",
	"354 Go ahead",
	"250 Data ok",
	"221 Goodbye",
}
