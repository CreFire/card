package main

import "backend/src/common/server"

func main() {
	server.ExitOnError("robot", "conf/robot.yaml", robotSvr)
}
