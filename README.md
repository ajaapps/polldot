# polldot

polldot is a Go program that regularly polls a remote http file.  If
the file exists and starts with '.' (without the quotes), polldot
sends a mail and exits.  Never more than one mail is sent.

If the file cannot be retrieved, or if it starts with something else
than '.', the program undertakes no action and sleeps until next
cycle.

The URL of the file, the mail settings (To, From, mailserver, etc) and
the frequency of polling for the file are configurable via a
configuration file ~/.polldot.json. Program execution is logged to
~/polldot.log.  The configuration will be loaded on startup or if a
SIGHUP signal is received.

The program exits when any one of these things has happened: * the
file is retrieved and starts with a dot '.' * upon receipt of SIGINT,
SIGTERM or SIGUSR1

