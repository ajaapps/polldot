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
	err = os.Setenv("HOME", "testdata")
	if err != nil {
		log.Fatal(err)
	}
}

func TestFetch(t *testing.T) {
	go http.ListenAndServe("127.0.0.1:8080", nil)
	time.Sleep(time.Millisecond * 50)

	t.Run("valid", func(t *testing.T) {
		http.Handle("/valid", http.HandlerFunc(valid))
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
		http.Handle("/notadot", http.HandlerFunc(notadot))
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
		http.Handle("/empty", http.HandlerFunc(empty))
		err = fetch("http://127.0.0.1:8080/empty")
		if err == nil {
			t.Error("expecting error, got nil")
		}

	})

	t.Run("flipping", func(t *testing.T) {
		http.Handle("/flipping", http.HandlerFunc(flipping))
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
	go http.ListenAndServe("127.0.0.1:8081", nil)
	go mailServer("127.0.0.1:2525", t)
	time.Sleep(time.Millisecond * 50)

	var c *config.Config = &config.Config{
		From:    "root@localhost",
		To:      "root@localhost",
		Subject: "mail from polldot go test",
		Body:    "test run at " + time.Now().String(),
	}

	t.Run("valid", func(t *testing.T) {
		// valid email
		c.Host = "127.0.0.1"
		c.Port = 2525
		err = mail(c)
		if err != nil {
			t.Errorf("expecting <nil>, got %+v", err)
		}
	})

	t.Run("recip", func(t *testing.T) {
		// invalid recipient
		c.Host = "127.0.0.1"
		c.Port = 2525
		c.To = "bad!re$ip ient"
		err = mail(c)
		if err == nil || !strings.Contains(err.Error(), "invalid address") {
			t.Errorf("expecting 'invalid address', got '%+v'", err)
		}
		c.To = "root@localhost"
	})

	t.Run("noconn", func(t *testing.T) {
		// no server on port
		c.Host = "127.0.0.1"
		c.Port = 65432 // asuming no server is running on this port
		err = mail(c)
		if _, ok := err.(*net.OpError); !ok {
			t.Errorf("expecting *net.OpError error, got %T", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		// non-mail server on port
		if testing.Short() {
			t.Skip("skipping timeout test")
		}
		c.Host = "127.0.0.1"
		c.Port = 8081
		err = mail(c)
		if err == nil || !strings.Contains(err.Error(), "mail timeout: ") {
			t.Errorf("expecting 'mail timeout: ', got '%+v'", err)
		}
	})

}

func TestInitLog(t *testing.T) {
	logfile := os.Getenv("HOME") + "/polldot.log"

	t.Run("normal", func(t *testing.T) {
		// OpenFile succeeds -> log lines in file with [pid]
		os.Remove(logfile)
		err := initLog()
		if err != nil {
			t.Log(err)
			t.FailNow()
		}
		defer os.Remove(logfile)
		defer log.SetPrefix("")

		str := "testline from polldot_test.go"
		log.Println(str)

		content, _ := ioutil.ReadFile(logfile)
		prefixLen := len(fmt.Sprintf("[%d] ", os.Getpid())) + 20 // prefix is like "[21308] 2017/01/19 15:2525:05 "
		str2 := string(content[prefixLen : prefixLen+len(str)])  // strip prefix and newline

		if str2 != str {
			t.Errorf("\nexpecting: '%v'\n      got: '%v'", str, str2)
		}
	})

	t.Run("nosuchdir", func(t *testing.T) {
		// OpenFile does not succeed -> err non nil
		defer os.Setenv("HOME", os.Getenv("HOME"))
		os.Setenv("HOME", "/tmp/this/should/not/exist/at/all")

		err := initLog()
		if err == nil {
			t.Errorf("expecting error, got %+v", err)
		}
		os.Remove(logfile)
		log.SetPrefix("")

	})

}

// TestInitConfig implicitely also tests the config package
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
		initConfig() // just to create a config file
		defer os.Remove(configfile)

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
	//  TODO: provide cfg, webserver and mailserver
	configfile := os.Getenv("HOME") + "/.polldot.json"
	os.Remove(configfile)
	initConfig() // just to create vanilla cfg and config file
	defer os.Remove(configfile)

	logfile := os.Getenv("HOME") + "/polldot.log"
	initLog() // to not have too much output in tests
	defer os.Remove(logfile)

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
			cfg.URL = ""
			go func() { reload <- 1 }()
			go pollLoop()
			time.Sleep(time.Millisecond * 50) // TODO i don't like these sleeps
			if cfg.URL == "" {
				t.Errorf("Expected valid URL, got %s", cfg.URL)
			}
		})

		t.Run("nok", func(t *testing.T) {
			cfg.URL = "SOME STRANGE STRING"
			os.Remove(configfile)
			go func() { reload <- 1 }()
			go pollLoop()
			time.Sleep(time.Millisecond * 50) // TODO i don't like these sleeps
			if cfg.URL != "SOME STRANGE STRING" {
				t.Errorf("Expected '', got '%s'", cfg.URL)
			}
		})

	})
	// TODO:
	//  test fetch -> mail -> exit
}

func TestMain(t *testing.T) { //TODO
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
