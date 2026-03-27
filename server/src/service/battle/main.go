package main

import "backend/src/common/server"

func main() {
	server.ExitOnError("battle", "conf/battle.yaml")
}
