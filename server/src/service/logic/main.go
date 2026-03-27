package main

import "backend/src/common/server"

func main() {
	server.ExitOnError("logic", "conf/logic.yaml")
}
