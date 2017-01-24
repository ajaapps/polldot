package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ajaapps/polldot/config"
)

func init() {
	// fake mail server for testing
	go mailServer("127.0.0.1:2525")

	// web server for testing
	go http.ListenAndServe("127.0.0.1:8080", nil)
	http.Handle("/valid", http.HandlerFunc(valid))
	http.Handle("/notadot", http.HandlerFunc(notadot))
	http.Handle("/empty", http.HandlerFunc(empty))
	http.Handle("/flipping", http.HandlerFunc(flipping))

}

func TestFetch(t *testing.T) {

	t.Run("valid", func(t *testing.T) {
		err = fetch("http://127.0.0.1:8080/valid")
		if err != nil {
			t.Errorf("expecting nil, got %+v", err)
		}
	})

	t.Run("protocol", func(t *testing.T) {
		err = fetch("this: is an unsupported protocol scheme")
		if err == nil {
			t.Error("expecting error, got nil")
		}
	})

	t.Run("notadot", func(t *testing.T) {
		err = fetch("http://127.0.0.1:8080/notadot")
		if err == nil {
			t.Error("expecting error, got nil")
		}
	})

	t.Run("none", func(t *testing.T) {
		err = fetch("http://127.0.0.1:8080/")
		if err == nil {
			t.Error("expecting error, got nil")
		}
	})

	t.Run("empty", func(t *testing.T) {
		err = fetch("http://127.0.0.1:8080/empty")
		if err == nil {
			t.Error("expecting error, got nil")
		}

	})

	t.Run("flipping", func(t *testing.T) {
		err = fetch("http://127.0.0.1:8080/flipping")
		if err == nil {
			t.Error("expecting error, got nil")
		}
		err = fetch("http://127.0.0.1:8080/flipping")
		if err != nil {
			t.Errorf("expecting nil, got %+v", err)
		}
	})

}

func TestMail(t *testing.T) {

	var c *config.Config

	// valid email
	t.Run("valid", func(t *testing.T) {
		c = testCfg()
		err = mail(c)
		if err != nil {
			t.Errorf("expecting <nil>, got %+v", err)
		}
	})

	// invalid recipient
	t.Run("recip", func(t *testing.T) {
		c = testCfg()
		c.To = "bad!re$ip ient"
		err = mail(c)
		if err == nil || !strings.Contains(err.Error(), "invalid address") {
			t.Errorf("expecting 'invalid address', got '%+v'", err)
		}
	})

	// no server on port
	t.Run("noconn", func(t *testing.T) {
		c = testCfg()
		c.Port = 65432 // asuming no server is running on this port
		err = mail(c)
		if _, ok := err.(*net.OpError); !ok {
			t.Errorf("expecting *net.OpError error, got %T", err)
		}
	})

	// non-mail server on port
	t.Run("timeout", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping timeout test")
		}
		c = testCfg()
		c.Port = 8080
		err = mail(c)
		if err == nil || !strings.Contains(err.Error(), "mail timeout: ") {
			t.Errorf("expecting 'mail timeout: ', got '%+v'", err)
		}
	})

}

func TestInitLog(t *testing.T) {

	// OpenFile succeeds -> log lines in file with [pid]
	t.Run("normal", func(t *testing.T) {
		initTest()
		logfile := os.Getenv("HOME") + "/polldot.log"
		os.Remove(logfile)
		err := initLog()
		if err != nil {
			t.Log(err)
			t.FailNow()
		}

		str := "testline from polldot_test.go"
		log.Println(str)
		content, _ := ioutil.ReadFile(logfile)
		prefixLen := len(fmt.Sprintf("[%d] ", os.Getpid())) + 20 // prefix is like "[21308] 2017/01/19 15:23:05 "
		str2 := string(content[prefixLen : prefixLen+len(str)])  // strip prefix and newline
		if str2 != str {
			t.Errorf("\nexpecting: '%v'\n      got: '%v'", str, str2)
		}
	})

	// OpenFile does not succeed -> err non nil
	t.Run("nosuchdir", func(t *testing.T) {
		initTest()
		os.Setenv("HOME", "/tmp/this/should/not/exist/at/all")

		err := initLog()
		if err == nil {
			t.Errorf("expecting error, got %+v", err)
		}
	})

}

// TestInitConfig implicitely also tests the config package
// TODO use initTest()
func TestInitConfig(t *testing.T) {
	configfile := os.Getenv("HOME") + "/.polldot.json"

	t.Run("nosuchdir", func(t *testing.T) {
		// config.Load returns non-nil error
		defer os.Setenv("HOME", os.Getenv("HOME"))
		os.Setenv("HOME", "/tmp/this/should/not/exist/at/all")

		err := initConfig()
		if err == nil {
			t.Errorf("expecting error, got %+v", err)
		}
	})

	t.Run("nofile", func(t *testing.T) {
		// no config file -> error
		os.Remove(configfile)

		err := initConfig()
		defer os.Remove(configfile)

		expected := "edit config file and retry"
		if err != nil {
			if err.Error() != expected {
				t.Errorf("expecting '%s' error, got '%s'", expected, err)
			}
		} else { // err == nil
			t.Errorf("expecting '%s' error, got nil", expected)
		}

	})

	t.Run("normal", func(t *testing.T) {
		// config.Load successful -> no error, expected content
		os.Remove(configfile)
		initConfig() // just to create a vanilla config file

		err := initConfig()
		if err != nil {
			t.Errorf("expecting nil, got %+v", err)
		}

		defaultCfg := &config.Config{URL: "http://www.example.net/path/dotfile", From: "from@some.host.net", To: "to@another.host.org", Subject: "subject text", Body: "Contents\nof the mail body.\n", Host: "smtp.mailserver.org", Port: 25, CycleLen: 10, CycleUnit: "minutes", Sleep: 600000000000}

		if !reflect.DeepEqual(*cfg, *defaultCfg) {
			t.Errorf("\nExpected: %#v ,\n     got: %#v", *defaultCfg, *cfg)
		}

	})
}

func TestPollLoop(t *testing.T) {
	// TODO use initTest()

	initLog()    // to not have too much output in tests
	initConfig() // to have an existing config file
	cfg = testCfg()

	t.Run("quit", func(t *testing.T) {
		go func() {
			quit <- 1
		}()
		str := pollLoop()
		if str != "exit." {
			t.Errorf("Expected 'exit.', got '%s'", str)
		}
	})

	t.Run("reload", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			cfg.URL = "SOME STRANGE STRING"
			go func() { reload <- 1 }()
			go pollLoop()
			time.Sleep(time.Millisecond * 50) // TODO i don't like these sleeps
			if cfg.URL == "SOME STRANGE STRING" {
				t.Errorf("Expected valid URL, got %s", cfg.URL)
			}
		})

		t.Run("nok", func(t *testing.T) {
			os.Remove(os.Getenv("HOME") + "/.polldot.json")
			cfg.URL = "SOME STRANGE STRING"
			go func() { reload <- 1 }()
			go pollLoop()
			time.Sleep(time.Millisecond * 50) // TODO i don't like these sleeps
			if cfg.URL != "SOME STRANGE STRING" {
				t.Errorf("Expected 'SOME STRANGE STRING', got '%s'", cfg.URL)
			}
		})

	})

	t.Run("after", func(t *testing.T) {
		// succesful fetch -> mail sent
		cfg = testCfg()
		cfg.Sleep = time.Millisecond * 100
		/*
			var ret string
			wait := cfg.Sleep * 10
			go func() {
				ret = pollLoop()
			}()
			time.Sleep(wait) // TODO
		*/
		var ret string = ""
		returned := make(chan string, 1)
		go func() { returned <- pollLoop() }()
		select {
		case ret = <-returned:
		case <-time.After(mailTimeout + time.Second):
			ret = "pollLoop timed out"
		}
		if ret != "mail sent" {
			t.Errorf("Expected 'mail sent', got '%s'", ret)
		}

	})
}

func TestMain(t *testing.T) { //TODO
	// TODO use initTest()
	// start een mail- en webserver en verzorg config file
	//  start het programma met cmd.Run(...)
	// beeindig het door de file te serveren
	// check contents of logfile
	// check de mail

	// maybe move TestFetch/flipping here
	// maybe move most of TestPollLoop here

	configfile := os.Getenv("HOME") + "/.polldot.json"
	os.Remove(configfile)
	defer os.Remove(configfile)
	logfile := os.Getenv("HOME") + "/polldot.log"
	os.Remove(logfile)
	// main()

}
