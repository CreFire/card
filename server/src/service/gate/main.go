package main

import "backend/src/common/server"

func main() {
	server.ExitOnError("gate", "conf/gate.yaml", gateSvr)
}
