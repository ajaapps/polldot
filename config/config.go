// Package config loads the configuration for polldot.
// If a configuration file is not found, a new one is created from
// default values.
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"
)

// Config contains all the fields from the configuration file.
type Config struct {
	URL       string        `json:"url"` // the file to retreive and check for '.'
	From      string        `json:"from"`
	To        string        `json:"to"`
	Subject   string        `json:"subject"`
	Body      string        `json:"body"`
	Host      string        `json:"host"` // mailserver hostname
	Port      int           `json:"port"` // mailserver port no
	CycleLen  int           `json:"cycle_length"`
	CycleUnit string        `json:"cycle_unit"` // "minutes" or "seconds"
	Sleep     time.Duration // duration between fetch cycles; calculated value
}

var (
	cfg    *Config
	minDur time.Duration = time.Second * 10 // lower limit on poll frequency
)

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
			return cfg, fmt.Errorf("%s", "edit config file and retry")

		default:
			return nil, err

		}
	}

	return cfg, nil
}

// read reads the file and marshals it into cfg. It gives the
// Sleep field its value.
func read() error {

	// open file
	home := os.Getenv("HOME")
	if home == "" {
		return fmt.Errorf("HOME must be set")
	}
	fd, err := os.Open(home + "/.polldot.json")
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

	// calculate value for Sleep field
	cfg.Sleep, err = calcSleep()
	if err != nil {
		return err
	}

	return nil
}

// defaults sets some sane default values for cfg
func defaults() {
	cfg = &Config{
		URL:       "http://www.example.net/path/dotfile",
		From:      "from@some.host.net",
		To:        "to@another.host.org",
		Subject:   "subject text",
		Body:      "Contents\nof the mail body.\n",
		Host:      "smtp.mailserver.org",
		Port:      25,
		CycleLen:  10,
		CycleUnit: "minutes",
		Sleep:     time.Minute * 10,
	}
}

// calcSleep calculates the Sleep field using CycleLen and CycleUnit.
// A minimum value is enforced.
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
		e = fmt.Errorf("wrong unit: %+v", cfg.CycleUnit)
	}

	if d < minDur {
		// conform to minimal duration between cycles
		d = minDur
	}

	return d, e
}

// write writes cfg in json encoded format to the file
func write() error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	home := os.Getenv("HOME")
	if home == "" {
		return fmt.Errorf("homedirectory not in environment")
	}

	return ioutil.WriteFile(home+"/.polldot.json", data, 0644)
}
