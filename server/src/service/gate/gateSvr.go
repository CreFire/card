package main

import (
	"fmt"
	"net/http"

	"backend/deps/xlog"
	"backend/src/common/server"
	gatemodule "backend/src/service/gate/module"
)

var gateSvr = NewGateSvr()
var _ server.Service = (*GateSvr)(nil)
var _ server.RuntimeAware = (*GateSvr)(nil)
var _ server.HTTPRouteRegistrar = (*GateSvr)(nil)

type GateSvr struct {
	runtime *server.Runtime
	module  *gatemodule.GateModule
}

func NewGateSvr() *GateSvr {
	return &GateSvr{}
}

func (s *GateSvr) SetRuntime(runtime *server.Runtime) error {
	s.runtime = runtime
	return nil
}

func (s *GateSvr) OnInit() error {
	if s.runtime == nil {
		return fmt.Errorf("gate runtime is not configured")
	}

	module, err := gatemodule.New(s.runtime)
	if err != nil {
		return err
	}
	if err := module.Init(); err != nil {
		return err
	}
	s.module = module
	return nil
}

func (s *GateSvr) Start() error {
	if s.module == nil {
		return fmt.Errorf("gate module is not initialized")
	}
	if err := s.module.Start(); err != nil {
		return err
	}
	xlog.Infof("[gate] gate module started")
	return nil
}

func (s *GateSvr) Stop() error {
	if s.module == nil {
		return nil
	}
	return s.module.Stop()
}

func (s *GateSvr) BeforeStop() error {
	return nil
}

func (s *GateSvr) BeforeStart() error {
	return nil
}

func (s *GateSvr) AfterStart() error {
	return nil
}

func (s *GateSvr) AfterStop() error {
	return nil
}

func (s *GateSvr) OnReload() error {
	return nil
}

func (s *GateSvr) RegisterHTTP(mux *http.ServeMux) {
	if s.module == nil {
		return
	}
	s.module.RegisterHTTP(mux)
}
