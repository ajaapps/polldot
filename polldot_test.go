package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ajaapps/polldot/config"
)

func init() {

	if os.Getenv("TESTMAIN") != "" {
		// see TestMain(): this is a child process; servers are
		// already up
		return
	}

	// fake mail server for testing
	go fakeSMTP("127.0.0.1:2525")

	err = waitFor("127.0.0.1:2525")
	if err != nil {
		log.Fatalf("init() can not dial fake mailserver.", err)
	}

	// web server for testing
	go http.ListenAndServe("127.0.0.1:8080", nil)
	http.Handle("/valid", http.HandlerFunc(valid))
	http.Handle("/notadot", http.HandlerFunc(notadot))
	http.Handle("/empty", http.HandlerFunc(empty))
	http.Handle("/flipping", http.HandlerFunc(flipping))

	err = waitFor("127.0.0.1:8080")
	if err != nil {
		log.Fatalf("init() can not dial test web server.", err)
	}

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
			t.SkipNow()
		}
		c = testCfg()
		c.Port = 8080

		err = mail(c)
		if err == nil || !strings.Contains(err.Error(), "mail timeout: ") {
			t.Errorf("expecting 'mail timeout: ', got '%+v'", err)
		}
	})

}

/*
 */
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
		flog.Println(str)
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

// TestInitConfig implicitely also tests parts of the config package
func TestInitConfig(t *testing.T) {

	// homedir does not exist -> error
	t.Run("nosuchdir", func(t *testing.T) {
		initTest()
		os.Setenv("HOME", "/tmp/this/should/not/exist/at/all")

		err := initConfig()
		if err == nil {
			t.Errorf("expecting error, got %+v", err)
		}
	})

	// no config file -> error
	t.Run("nofile", func(t *testing.T) {
		initTest()
		os.Remove(os.Getenv("HOME") + "/.polldot.json")

		err := initConfig()

		if _, ok := err.(config.ErrVanilla); !ok {
			t.Errorf("expecting error type 'ErrVanilla', got %T", err)
		}

	})

	// config.Load successful -> no error, expected content
	t.Run("normal", func(t *testing.T) {
		initTest()
		cfg = new(config.Config)

		err := initConfig()
		if err != nil {
			t.Errorf("expecting nil, got %+v", err)
		}

		defaultCfg := testCfg()

		if !reflect.DeepEqual(*cfg, *defaultCfg) {
			t.Errorf("\nExpected: %#v ,\n     got: %#v", *defaultCfg, *cfg)
		}

	})
}

func TestPollLoop(t *testing.T) {

	t.Run("quit", func(t *testing.T) {
		initTest()
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
			initTest()
			cfg.URL = "SOME STRANGE STRING"
			go func() { reload <- 1 }()
			go pollLoop()
			time.Sleep(time.Millisecond * 50) // TODO i don't like these sleeps
			if cfg.URL == "SOME STRANGE STRING" {
				t.Errorf("Expected valid URL, got %s", cfg.URL)
			}
		})

		t.Run("nok", func(t *testing.T) {
			initTest()
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

	// succesful fetch -> mail sent
	t.Run("after", func(t *testing.T) {
		initTest()
		config.Sleep = time.Millisecond * 100
		ch := make(chan string, 1)
		str := ""

		go func() { ch <- pollLoop() }()

		select {
		case str = <-ch:
		case <-time.After(mailTimeout + time.Second):
			str = "pollLoop timed out"
		}

		if str != "mail sent." {
			t.Errorf("Expected 'mail sent.', got '%s'", str)
		}

	})
}

/*
 */

func TestMain(t *testing.T) { //TODO answer why it does not give extra coverage, while none zero percentages are given.
	// note: see use of cmd.Process.Kill() in net/http/serve_test.go:
	// this way we can use / test wait also.
	if os.Getenv("TESTMAIN") == "1" {
		initTest()
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMain")
	cmd.Env = append(os.Environ(), "TESTMAIN=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		t.Errorf("process ran with err %v, want <nil>", err)
	}

	//t.Run("normal", func(t *testing.T) {
	//})
}

/*
 */
