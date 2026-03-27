package main

import "backend/src/common/server"

func main() {
	server.ExitOnError("auth", "conf/auth.yaml", authSvr)
}
