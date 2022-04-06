// package sms

// const (
// 	// ServerPort tells the gRPC server what port to listen on
// 	ServerPort = ":1000"
// 	// Endpoint defines the DNS of the account server for clients
// 	// to access the server in Kubernetes.
// 	Endpoint = "smsserver-service" + ServerPort
// )

package sms

import "os"

var (
	// ServerPort tells the gRPC server what port to listen on
	ServerPort = ":" + os.Getenv("SMS_SERVICE_PORT")
	ServerIP   = os.Getenv("SMS_SERVICE_IP")
	// Endpoint defines the DNS of the account server for clients
	// to access the server in Kubernetes.
	Endpoint = ServerIP + ServerPort
)
