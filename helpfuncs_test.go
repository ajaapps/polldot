package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
)

func valid(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "%s", ".")
}
func invalid(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "%s", "invalid content")
}

var hasFlipped bool = false

func flipping(w http.ResponseWriter, req *http.Request) {
	if hasFlipped {
		valid(w, req)
		return
	}
	invalid(w, req)
	hasFlipped = true
}

// testFatal is a helper function. It runs a subtest by starting gorun
// again, this time using -test.run flag and setting an environment variable.
//
// This way we can test functions that should can end with an exit(1), like
// with a log.Fatal(). TODO see if this also works for exit(2) and
// friends. This way of testing these functions is found here:
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
