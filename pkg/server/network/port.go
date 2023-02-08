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

// IsPortOpen returns whether the given port is bound to the given prefix. If prefix is empty, this is assumed to be
// 0.0.0.0. Note that 0.0.0.0 does not necessarily mean all network interfaces have been checked. For example, Vite
// ports (bound to localhost) will not be discovered on 0.0.0.0.
func IsPortOpen(prefix string, port int) bool {
	// Attempt to dial the port. If we establish a connection, it's in use.
	conn, _ := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", prefix, port), time.Second)
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
func FindOpenPortFrom(prefix string, basePort, numAttempts int) (int, error) {
	for port := basePort; port <= basePort+numAttempts; port++ {
		if IsPortOpen(prefix, port) {
			return port, nil
		}
	}

	return 0, errors.Errorf("could not find an open port in %d attempts", numAttempts)
}

// VerifyDevViewPath verifies that the encoded token in the path matches the dev server token (if provided), and
// returns the port that views are proxied to.
func VerifyDevViewPath(path string, token *string) (int, error) {
	// The components of the path should look like ["", "dev", "views", {port}, {token}, ...]
	pathComponents := strings.Split(path, "/")
	if token != nil {
		if len(pathComponents) < 5 {
			return 0, errors.New("request path is malformed, not of the form /dev/views/{port}/{token}...")
		}

		if pathComponents[4] != *token {
			return 0, errors.New("request token does not match dev token")
		}
	} else if len(pathComponents) < 4 {
		return 0, errors.New("request path is malformed, not of the form /dev/views/{port}...")
	}

	port, err := strconv.Atoi(pathComponents[3])
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse port")
	}

	return port, nil
}

// ViewPortProxy returns a reverse proxy that proxies requests with path beginning with /dev/views/{port} to the given
// port.
func ViewPortProxy(devToken *string) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		if req.URL == nil {
			logger.Error("request URL is nil")
			return
		}

		// We proxy requests of the form /dev/views/{port}/{token}/...
		if !strings.HasPrefix(req.URL.Path, "/dev/views/") {
			logger.Error("request path does not start with /dev/views/")
			return
		}

		port, err := VerifyDevViewPath(req.URL.Path, devToken)
		if err != nil {
			logger.Error(err.Error())
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
