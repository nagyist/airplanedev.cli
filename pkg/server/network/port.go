package network

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/pkg/errors"
)

// IsPortOpen returns whether the given port is bound to any network interface.
func IsPortOpen(port int) bool {
	// Attempt to dial the port. If we establish a connection, it's in use.
	conn, _ := net.DialTimeout("tcp", fmt.Sprintf(":%d", port), time.Second)
	if conn == nil {
		return true
	}
	defer conn.Close()
	return false
}

// FindOpenPort finds any open port on the host machine.
func FindOpenPort() (int, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, errors.Wrap(err, "failed to listen on any port")
	}

	return listener.Addr().(*net.TCPAddr).Port, nil
}

// FindOpenPortFrom finds an open port on the host machine, starting at the given port.
func FindOpenPortFrom(basePort, numAttempts int) (int, error) {
	for port := basePort; port <= basePort+numAttempts; port++ {
		if IsPortOpen(port) {
			return port, nil
		}
	}

	return 0, errors.Errorf("could not find an open port in %d attempts", numAttempts)
}

// ViewPortProxy returns a reverse proxy that proxies requests with path beginning with /dev/views/{port} to the given
// port.
func ViewPortProxy() *httputil.ReverseProxy {
	director := func(req *http.Request) {
		if req.URL == nil {
			logger.Error("request URL is nil")
			return
		}

		// We proxy requests of the form /dev/views/{port}/...
		if !strings.HasPrefix(req.URL.Path, "/dev/views/") {
			logger.Error("request path does not start with /dev/views/")
			return
		}

		pathComponents := strings.Split(req.URL.Path, "/")
		if len(pathComponents) < 4 {
			logger.Error("request path is malformed, not of the form /dev/views/{port}/...")
			return
		}

		// The components of the path should look like ["", "dev", "views", {port}, ...]
		portComponent := pathComponents[3]
		port, err := strconv.Atoi(portComponent)
		if err != nil {
			logger.Error("failed to parse port from path: %s", req.URL.Path)
			return
		}

		targetAddress := fmt.Sprintf("localhost:%d", port)
		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", targetAddress)
		req.URL.Scheme = "http"
		req.URL.Host = targetAddress
	}

	return &httputil.ReverseProxy{Director: director}
}