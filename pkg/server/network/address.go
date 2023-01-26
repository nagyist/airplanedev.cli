package network

import "fmt"

const containerAddress = "0.0.0.0"
const loopbackAddress = "127.0.0.1"

// LocalAddress returns the TCP address that localhost listens on.
func LocalAddress(port int, expose bool) string {
	var addr string
	if expose {
		addr = containerAddress
	} else {
		addr = loopbackAddress
	}
	return fmt.Sprintf("%s:%d", addr, port)
}
