// package account

// const (
// 	// ServerPort tells the gRPC server what port to listen on
// 	ServerPort = ":1000"
// 	// Endpoint defines the DNS of the account server for clients
// 	// to access the server in Kubernetes.
// 	Endpoint = "accountserver-service" + ServerPort
// )

package account

import "os"

var (
	// ServerPort tells the gRPC server what port to listen on
	ServerPort = ":" + os.Getenv("ACCOUNT_SERVICE_PORT")
	ServerIP   = os.Getenv("ACCOUNT_SERVICE_IP")
	// Endpoint defines the DNS of the account server for clients
	// to access the server in Kubernetes.
	Endpoint = ServerIP + ServerPort
)
