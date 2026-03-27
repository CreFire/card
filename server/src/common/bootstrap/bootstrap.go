package bootstrap

import Server "backend/src/common/server"

func Run(serviceName, defaultConfigPath string, services ...Server.Service) error {
	return Server.Run(serviceName, defaultConfigPath, services...)
}

func ExitOnError(serviceName, defaultConfigPath string, services ...Server.Service) {
	Server.ExitOnError(serviceName, defaultConfigPath, services...)
}
