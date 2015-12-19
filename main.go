package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

var (
	username   string
	password   string
	binaryPath string = "openvpn"
)

// Read a traditional OpenVPN credential file and pull the username and password out. The format of the file should be:
//		$ cat /path/to/credentials
//		username
//		password
//
// I.e. a username followed by a password
func ReadCredentialsFromFile(path string) (err error) {
	if path == "" {
		return fmt.Errorf("no password file set")
	}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	ss := strings.Split(string(b), "\n")
	if username == "" && ss[0] != "" {
		username = ss[0]
	}
	if password == "" && ss[1] != "" {
		password = ss[1]
	}

	return
}

// Parse and check the enviornmental variables for userful information.
func ParseEnvironment() (err error) {
	for _, e := range os.Environ() {
		kv := strings.SplitN(e, "=", 2)

		switch kv[0] {
		case "OPENVPN_USER":
			username = kv[1]
		case "OPENVPN_PASSWORD":
			password = kv[1]
		case "OPENVPN_PATH":
			binaryPath = kv[1]
		case "OPENVPN_PWD_FILE":
			if err := ReadCredentialsFromFile(kv[1]); err != nil {
				return err
			}
		}
	}

	return
}

func ManageChild(c *exec.Cmd) {
	signals := make(chan os.Signal, 10)
	signal.Notify(signals)

	for sig := range signals {
		switch sig {
		case os.Interrupt, os.Kill:
			for i := 0; !c.ProcessState.Exited() && i < 10; i++ {
				if err := c.Process.Signal(sig); err != nil {
					log.Warning(err)
				}
				time.Sleep(time.Second)
			}

			if !c.ProcessState.Exited() {
				log.Error("Unable to stop subprocess :(")
			}

			os.Exit(0)
		default:
			if err := c.Process.Signal(sig); err != nil {
				log.Warning(err)
			}
		}
	}
}

func main() {
	if err := ParseEnvironment(); err != nil {
		log.Fatal("Error processing environment: ", err)
	}

	if username == "" {
		log.Warning("No username set")
	}
	if password == "" {
		log.Warning("No password set")
	}

	c := exec.Command(binaryPath, os.Args[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	w, err := c.StdinPipe()
	if err != nil {
		w.Write([]byte(username))
		w.Write([]byte("\n"))
		w.Write([]byte(password))
		w.Write([]byte("\n"))
		go func() {
			io.Copy(w, os.Stdin)
		}()
	}

	if err := c.Run(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
