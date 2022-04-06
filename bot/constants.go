// package bot

// const (
// 	// ServerPort tells the gRPC server what port to listen on
// 	ServerPort = ":1000"
// 	// Endpoint defines the DNS of the account server for clients
// 	// to access the server in Kubernetes.
// 	Endpoint = "botserver-service" + ServerPort
// )

package bot

import "os"

var (
	// ServerPort tells the gRPC server what port to listen on
	ServerPort = ":" + os.Getenv("BOT_SERVICE_PORT")
	ServerIP   = os.Getenv("BOT_SERVICE_IP")
	// Endpoint defines the DNS of the account server for clients
	// to access the server in Kubernetes.
	Endpoint = ServerIP + ServerPort
)
