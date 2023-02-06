package status

type ServerStatus string

const (
	ServerDiscovering ServerStatus = "discovering"
	ServerReady       ServerStatus = "ready"
)
