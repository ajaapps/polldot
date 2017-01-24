/*
polldot regularly polls for the existence and contents of file.
The file is expected to be offered online by a webserver.

If the file exists and contains '.' (without the quotes), a mail is
sent.After sending the mail, the program exits. Never more than
one mail is sent.

If the file cannot be retreived, or if it contains something else
than '.', the program undertakes no action and waits for a while
cycle to try again.

The URL of the file, the mail settings (To, From, mailserver, etc) and
the frequency of polling for the file are configurable via a
configuration file ~/.polldot.json. Program execution is logged to
~/polldot.log.

The configuration will be loaded on startup and when sending the
program a SIGHUP signal. The program exits when any one of these
things has happened:
- the file is retreived and starts with a dot '.'
- the program receives SIGINT, SIGTERM or SIGUSR1
*/
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ajaapps/polldot/config"
	"github.com/go-gomail/gomail"
)

var (
	err         error
	cfg         *config.Config
	mailerr     chan error    = make(chan error, 1)
	quit        chan int      = make(chan int, 1)
	reload      chan int      = make(chan int, 1)
	mailTimeout time.Duration = time.Second * 5
)

// fetch gets the file and checks its contents
// A non-nil error is returned if:
//   - the file is empty, or
//   - the file could not be retreived, or
//   - the file starts with something else then '.' (Note that most
//     webservers will respond with some html content if the file is
//     not found. In that case the first rune is typically '<'.
func fetch(url string) error {

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	content := make([]byte, 1)
	n, err := resp.Body.Read(content)
	if n == 0 {
		return fmt.Errorf("no content; %+v", err)
	}

	if str := string(content[0]); str != "." {
		return fmt.Errorf("got '%s'", str)
	}

	return nil
}

// mail sends the mail
func mail(cfg *config.Config) error {
	m := gomail.NewMessage()
	m.SetHeader("From", cfg.From)
	m.SetHeader("To", cfg.To)
	m.SetHeader("Subject", cfg.Subject)
	m.SetBody("text/plain", cfg.Body)
	d := gomail.Dialer{Host: cfg.Host, Port: cfg.Port}
	go func() {
		// run in separate goroutine, so we can enforce a timeout
		mailerr <- d.DialAndSend(m)
	}()
	return mailWait(mailTimeout)
}

// mailWait waits for either mail result or timeout,
// which ever comes first.
func mailWait(timeout time.Duration) error {
	select {
	case e := <-mailerr:
		return e
	case <-time.After(timeout):
		return fmt.Errorf("mail timeout: %+v", timeout)
	}
}

// initLog configures log to use a logfile and a prefix
func initLog() error {
	filename := os.Getenv("HOME") + "/polldot.log"
	fd, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	log.SetOutput(io.Writer(fd))
	log.SetPrefix(fmt.Sprintf("[%d] ", os.Getpid()))

	return nil
}

// initConfig fills the cfg variable
func initConfig() error {
	cfg, err = config.Load()
	return err
}

// catchSignals catches signals SIGHUP, SIGINT, SIGTERM and SIGUSR1.
// SIGHUP triggers a reload of the configuration.
// SIGINT, SIGTERM and SIGUSR1 will make the program exit.
func catchSignals() {

	signals := make(chan os.Signal, 1)
	signal.Notify(
		signals,
		syscall.SIGHUP,
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1,
	)

	log.Println("waiting for signals ...")

	for sig := range signals {
		log.Printf("received signal: %+v\n", sig)
		switch sig {
		case syscall.SIGHUP:
			reload <- 1
		case syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1:
			quit <- 1
		}
	}
}

// pollLoop is the main fetch/main loop of the program.
func pollLoop() string {
	for {
		select {

		case <-quit:
			return "exit."

		case <-reload:

			cfgOld := cfg
			err = initConfig()
			if err != nil {
				log.Printf("not using new config; %+v\n", err)
				cfg = cfgOld
			}
			log.Printf("configuration: %+v\n", cfg)

		case <-time.After(cfg.Sleep):

			err = fetch(cfg.URL)
			if err != nil {
				log.Println(err)
				continue // retry again later
			}
			err = mail(cfg)
			if err != nil {
				return err.Error()
			}
			return "mail sent"
		}
	}
}

func main() {

	// initialize logging
	err = initLog()
	if err != nil {
		log.Println(err)
		log.SetOutput(os.Stderr)
		log.Fatal(err)
	}
	log.Println("")
	log.Println("polldot started")

	// initialize configuration
	err = initConfig()
	if err != nil {
		log.Println(err)
		log.SetOutput(os.Stderr)
		log.Fatal(err)
	}
	log.Printf("configuration: %+v\n", cfg)

	// start signal handler
	go catchSignals()

	// start the main fetch/mail loop
	returnStr := pollLoop()

	log.Println(returnStr)
	log.SetOutput(os.Stderr)
	log.Println(returnStr)
}
