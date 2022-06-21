package frontcache

import "os"

var (
	ServerPort = ":" + os.Getenv("FRONTCACHE_SERVICE_PORT")
	ServerIP   = os.Getenv("FRONTCACHE_SERVICE_IP")
	Endpoint   = ServerIP + ServerPort
)
