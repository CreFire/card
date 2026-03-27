package server

import (
	"net/http"

	"backend/src/common/configdoc"
	"backend/src/common/discovery"
	mongostore "backend/src/persist/mongo"
	redisstore "backend/src/persist/redis"
)

type Service interface {
	OnInit() error
	Start() error
	Stop() error
	BeforeStop() error
	BeforeStart() error
	AfterStart() error
	AfterStop() error
	OnReload() error
}

type Runtime struct {
	ServiceName string
	Config      *configdoc.ConfigBase
	Registry    *discovery.Registry
	Redis       *redisstore.Client
	Mongo       *mongostore.Client
}

type RuntimeAware interface {
	SetRuntime(*Runtime) error
}

type HTTPRouteRegistrar interface {
	RegisterHTTP(*http.ServeMux)
}
