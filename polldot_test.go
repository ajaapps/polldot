package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/ajaapps/polldot/config"
)

var err error

func TestFetch(t *testing.T) {

	go http.ListenAndServe("127.0.0.1:5050", nil)
	time.Sleep(time.Second) // server needs some time to start

	t.Run("valid", func(t *testing.T) {
		http.Handle("/valid", http.HandlerFunc(valid))
		err = fetch("http://127.0.0.1:5050/valid")
		if err != nil {
			t.Errorf("expecting nil, got %+v", err)
		}
	})

	t.Run("none", func(t *testing.T) {
		err = fetch("http://127.0.0.1:5050/")
		if err == nil {
			t.Error("expecting error, got nil")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		http.Handle("/invalid", http.HandlerFunc(invalid))
		err = fetch("http://127.0.0.1:5050/invalid")
		if err == nil {
			t.Error("expecting error, got nil")
		}

	})

	t.Run("flipping", func(t *testing.T) {
		http.Handle("/flipping", http.HandlerFunc(flipping))
		err = fetch("http://127.0.0.1:5050/flipping")
		if err == nil {
			t.Error("expecting error, got nil")
		}
		err = fetch("http://127.0.0.1:5050/flipping")
		if err != nil {
			t.Errorf("expecting nil, got %+v", err)
		}
	})

}

func TestMail(t *testing.T) {
	// TODO create servers. These tests asume a local mailserver on
	// port 25 and a webserver on port 6060

	var c *config.Config = &config.Config{
		From:    "root@localhost",
		To:      "root@localhost",
		Subject: "go test -run=TestMail",
		Body:    "test run at " + time.Now().String(),
	}

	t.Run("valid", func(t *testing.T) {
		// valid email, using local mail system
		c.Host = "127.0.0.1"
		c.Port = 25
		err = mail(c)
		if err != nil {
			t.Errorf("expecting nil error, got %+v", err)
		}
	})

	t.Run("noserver", func(t *testing.T) {
		// no server on port
		c.Host = "127.0.0.1"
		c.Port = 65432
		err = mail(c)
		if _, ok := err.(*net.OpError); !ok {
			t.Errorf("expecting *net.OpError error, got %T", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		// non-mail server on port (local godoc server)
		if testing.Short() {
			t.Skip("skipping timeout test")
		}
		c.Host = "127.0.0.1"
		c.Port = 6060
		err = mail(c)
		if err == nil {
			t.Errorf("expecting timeout error, got nil")
		}
	})

	t.Run("reject", func(t *testing.T) {
		// mailserver rejects recipient
		c.Host = "127.0.0.1"
		c.Port = 25
		c.To = "bad!recip$ient"
		err = mail(c)
		if err == nil {
			t.Errorf("expecting error from rejecting server, got nil")
		}
		c.To = "root@localhost"
	})

}

func TestInitLog(t *testing.T) {
	defer os.Setenv("HOME", os.Getenv("HOME"))
	logfile := "testdata/polldot.log"
	defer os.Remove(logfile)

	t.Run("nodir", func(t *testing.T) {
		// OpenFile does not succeed -> err non nil
		os.Setenv("HOME", "/tmp/this/should/not/exist/at/all")
		//t.Errorf("TODO: %s", "implement TestInitLog/nodir")
	})

	t.Run("normal", func(t *testing.T) {
		// OpenFile succeeds -> log lines in file with [pid]
		str := "testline from polldot_test.go"

		defer log.SetOutput(io.Writer(os.Stderr))
		defer log.SetPrefix("")

		os.Setenv("HOME", "testdata")
		initLog()
		log.Println(str)

		content, _ := ioutil.ReadFile(logfile)
		prefixLen := len(fmt.Sprintf("[%d] ", os.Getpid())) + 20 // prefix is like "[21308] 2017/01/19 15:25:05 "
		str2 := string(content[prefixLen : prefixLen+len(str)])  // strip prefix and newline

		if str2 != str {
			t.Errorf("\nexpecting: '%v'\n      got: '%v'", str, str2)
		}
	})

}

func TestInitConfig(t *testing.T) {
	defer os.Setenv("HOME", os.Getenv("HOME"))
	configfile := "testdata/.polldot.json"
	defer os.Remove(configfile)

	t.Run("nodir", func(t *testing.T) {
		// config.Load returns error -> log.Fatal expected
		os.Setenv("HOME", "/tmp/this/should/not/exist/at/all")
		//t.Errorf("TODO: %s", "implement TestInitConfig/nodir")
	})

	t.Run("nofile", func(t *testing.T) {
		// no config file -> error
		os.Setenv("HOME", "testdata")
		os.Remove(configfile)

		err := initConfig()
		expected := "edit config file and retry"
		if err != nil {
			if err.Error() != expected {
				t.Errorf("expecting '%s' error, got '%s'", expected, err)
			}
		} else {
			t.Errorf("expecting '%s' error, got nil", expected)
		}
	})

	t.Run("normal", func(t *testing.T) {
		// config.Load successful -> no error, expected content
		os.Setenv("HOME", "testdata")
		os.Remove(configfile)
		_ = initConfig() // creates a default config file

		err := initConfig()
		if err != nil {
			t.Errorf("expecting nil, got %+v", err)
		}

		defaultCfg := &config.Config{"http://www.example.net/path/dotfile", "from@some.host.net", "to@another.host.org", "subject text", "Contents\nof the mail body.\n", "smtp.mailserver.org", 25, 10, "minutes", 600000000000}
		if !reflect.DeepEqual(*cfg, *defaultCfg) {
			t.Errorf("\nExpected: %#v ,\n     got: %#v", *defaultCfg, *cfg)
		}

	})
}

func TestWait(t *testing.T) {
	//t.Errorf("TODO: implement TestWait")
	// -hup: reload config (use testFatal)
	//             a. no file -> using old config
	//             b. good file -> use new settings
	// -int: exit program
}
