package main

import "backend/src/common/server"

func main() {
	server.ExitOnError("public", "conf/public.yaml")
}
