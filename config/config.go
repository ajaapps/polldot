// Package config loads the configuration for polldot.
// If a configuration file is not found, a new one is created from
// default values.
package config

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"
)

// ErrVanilla is returned when no configurationfile is found and a new
// file is written. The content of the file corresponds to the value
// of the cfg variable just after defaults().
type ErrVanilla struct {
	path string
	error
}

func (e ErrVanilla) Error() string {
	return "new Vanilla configuration file created. Please edit this file: " + e.path
}

var ErrHomeless = errors.New("HOME must be set")
var ErrUnit = errors.New("CycleUnit must be 'seconds' or 'minutes'")
var ErrMinsleep = errors.New("config.Sleep must not be less then " + minSleep.String())

// Config contains all the fields from the configuration file.
type Config struct {
	URL string `json:"url"` // the file to retreive and check for '.'
	mailCfg
	CycleLen  int    `json:"cycle.length"`
	CycleUnit string `json:"cycle.unit"` // "minutes" or "seconds"
}

// mailCfg contains configuration data for the mail to be sent
type mailCfg struct {
	From    string `json:"mail.from"`
	To      string `json:"mail.to"`
	Subject string `json:"mail.subject"`
	Body    string `json:"mail.body"`
	Host    string `json:"mail.host"` // mailserver hostname
	Port    int    `json:"mail.port"` // mailserver port no
}

var (
	cfg      *Config
	Sleep    time.Duration // duration between fetch cycles; calculated value
	minSleep time.Duration = time.Second * 10
)

func cfgFilename() string {
	home := os.Getenv("HOME")
	if home == "" {
		log.Fatal(ErrHomeless)
	}
	return home + "/.polldot.json"
}

// Load loads the configuration from disk. If the configuration
// file is not found, a default configuration is used and written
// to disk.
func Load() (*Config, error) {

	cfg = new(Config)
	err := read()

	if err != nil {

		switch err.(type) {

		case *os.PathError: // create new from defaults
			defaults()
			err = write()
			if err != nil {
				return nil, err
			}
			return cfg, ErrVanilla{path: cfgFilename()}

		default:
			return nil, err

		}
	}

	return cfg, nil
}

// read reads the file and marshals it into cfg. It also gives the
// Sleep variable its value.
func read() error {

	// open file
	fd, err := os.Open(cfgFilename())
	if err != nil {
		return err
	}
	defer fd.Close()

	// marshal file contents into a configuration
	r := io.Reader(fd)
	err = json.NewDecoder(r).Decode(cfg)
	if err != nil {
		return err
	}

	// calculate value for Sleep
	Sleep, err = calcSleep()
	if err != nil {
		return err
	}

	return nil
}

// defaults sets some sane default values for cfg
func defaults() {
	mailcfg := mailCfg{
		From:    "from@some.host.net",
		To:      "to@another.host.org",
		Subject: "subject text",
		Body:    "Contents\nof the mail body.\n",
		Host:    "smtp.mailserver.org",
		Port:    25,
	}
	cfg = &Config{
		"http://www.example.net/path/dotfile",
		mailcfg,
		10,
		"minutes",
	}
}

// calcSleep calculates the Sleep variable using CycleLen and CycleUnit.
// If Sleep is too small, the function returns an ErrMinsleep error.
func calcSleep() (d time.Duration, e error) {

	switch cfg.CycleUnit {
	case "seconds":
		d = time.Second * time.Duration(cfg.CycleLen)
		e = nil
	case "minutes":
		d = time.Minute * time.Duration(cfg.CycleLen)
		e = nil
	default:
		d = time.Hour * 24 * 365 // some 'random' very long duration
		e = ErrUnit
	}

	if d < minSleep {
		d = time.Hour * 24 * 365 // some 'random' very long duration
		e = ErrMinsleep
	}

	return d, e
}

// write writes cfg in json encoded format to the file
func write() error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(cfgFilename(), data, 0644)
}
