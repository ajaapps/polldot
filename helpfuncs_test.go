package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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

// waitFor waits for a tcp server to be connectable. If it takes
// longer then a full second to get a connection, a timeout error is
// returned.
func waitFor(addr string) error {

	tempDelay := 1 * time.Nanosecond
	timeout := 1 * time.Second

	for { // loop until a connection is accepted

		c, err := net.Dial("tcp", addr)
		if err == nil {
			c.Close()
			return nil
		}

		if tempDelay > timeout/2 {
			return fmt.Errorf("timeout exceeded (%+v).", timeout)
		}

		tempDelay *= 2
		time.Sleep(tempDelay)

	}
}

// initTest makes a clean test environment by:
// (0) creating an empty directory 'testdata'
// (1) setting the HOME environment variable to 'testdata'
// (2) setting cfg to a sane test value where the testservers are
// being used
// (3) create a corresponding configuration file
// (4) create an empty log file
// (5) makes flog a *log.Logger to that file
// The test servers are started from init() in polldot_test.go.
func initTest() {

	// (0)  (see func documentation)
	err = os.RemoveAll("testdata")
	if err != nil {
		log.Fatal(err)
	}
	err = os.Mkdir("testdata", os.ModeDir|0755)
	if err != nil {
		log.Fatal(err)
	}

	// (1)
	err = os.Setenv("HOME", "testdata")
	if err != nil {
		log.Fatal(err)
	}

	// (2)
	cfg = testCfg()
	sleep = time.Second * 10

	// (3)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile("testdata/.polldot.json", data, 0644)
	if err != nil {
		log.Fatal(err)
	}

	// (4)
	_, err = os.Create("testdata/polldot.log")
	if err != nil {
		log.Fatal(err)
	}

	// (5)
	fd, err := os.OpenFile("testdata/polldot.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	flog = log.New(io.Writer(fd), "TEST ", log.Lshortfile)
}

// testCfg returns a complete cfg variable with testing values
func testCfg() *config.Config {
	c := new(config.Config)
	c.URL = "http://127.0.0.1:8080/valid"
	c.From = "root@localhost"
	c.To = "root@localhost"
	c.Subject = "mail from polldot go test"
	c.Body = "test run"
	c.Host = "127.0.0.1"
	c.Port = 2525
	return c
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
func testNonreturner(envvar string, f func(), regexp string, t *testing.T) {
	if os.Getenv(envvar) == "1" {
		initTest()
		f()
	}
	cmd := exec.Command(os.Args[0], "-test.run="+regexp)
	cmd.Env = append(os.Environ(), envvar+"=1")
	err = cmd.Run()
	/*TODO parm for exit > 0
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("process ran with err %v, want exit status 1", err)
	*/
	if err != nil {
		t.Errorf("process ran with err %v, want <nil>", err)
	}

}

// fakeSMTP is a stub smtp server for testing.  Honoustly stolen from
// smtp_test.go
func fakeSMTP(addr string) error {

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		err = handleOne(conn)
		if err != nil {
			// i don't care
		}

		conn.Close()
	}
}

func handleOne(c net.Conn) error {

	tc := textproto.NewConn(c)

	for i := 0; i < len(serverChat); i++ {

		tc.PrintfLine(serverChat[i])
		if serverChat[i] == "221 Goodbye" {
			return nil
		}

		var msg string

		if serverChat[i] == "354 Go ahead" {
			// read body
			for {
				msg, err = tc.ReadLine()
				if err != nil {
					return err
				}
				if msg == "." {
					i = 6
					tc.PrintfLine(serverChat[i])
					break
				}
			}
		}

		msg, err = tc.ReadLine()
		if err != nil {
			return err
		}

		if strings.Contains(msg, "QUIT") {
			i = 6
		}
	}

	return nil
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
