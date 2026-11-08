// moki is a local mockup viewer. Agents write mockups over MCP. You look at them in the browser.
package main

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

var version = "dev"

// Config holds the settings of moki. cli.go reads them from flags and env vars.
type Config struct {
	Host, Dir, Username, Password, URL string
	Port                               int
	Background                         bool
}

// serve starts the server, or prints the URL when moki already runs on the port.
func serve(cfg Config) error {
	local := cfg.localURL()
	if running(local) {
		printRunning(cfg)
		return nil
	}
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", joinHostPort(cfg.Host, cfg.Port))
	if err != nil {
		return err
	}
	s := NewServer(cfg)
	printBanner(cfg, s)
	return http.Serve(ln, s)
}

// localURL is the address that moki itself and Chrome use to reach the server.
func (c Config) localURL() string {
	h := c.Host
	if h == "" || h == "0.0.0.0" || h == "::" {
		h = "127.0.0.1"
	}
	return "http://" + joinHostPort(h, c.Port)
}

func running(url string) bool {
	client := http.Client{Timeout: time.Second}
	res, err := client.Get(url + "/health")
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode == 200
}

// publicHost turns an "any" address into a name that other machines can use.
func publicHost(h string) string {
	if h == "" || h == "0.0.0.0" || h == "::" {
		if name, err := os.Hostname(); err == nil {
			return name
		}
		return "127.0.0.1"
	}
	return h
}

func isLoopback(h string) bool {
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func joinHostPort(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}
