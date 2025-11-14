package passive

type Server interface {
	// start the server.
	Start(isLeader bool) error
	// stop the server. This function will close all active connections and stop the server.
	Stop() error
	// return the current status of the server. Possible values are "running", "stopped".
	Status() string
	// will return true if the server is ready to accept connections.
	Ready() bool
}
