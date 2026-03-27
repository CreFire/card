package main

import (
	"fmt"
	"net/http"

	"backend/deps/xlog"
	"backend/src/common/server"
	robotmodule "backend/src/service/robot/module"
)

var robotSvr = NewRobotSvr()
var _ server.Service = (*RobotSvr)(nil)
var _ server.RuntimeAware = (*RobotSvr)(nil)
var _ server.HTTPRouteRegistrar = (*RobotSvr)(nil)

type RobotSvr struct {
	runtime *server.Runtime
	module  *robotmodule.RobotModule
}

func NewRobotSvr() *RobotSvr {
	return &RobotSvr{}
}

func (s *RobotSvr) SetRuntime(runtime *server.Runtime) error {
	s.runtime = runtime
	return nil
}

func (s *RobotSvr) OnInit() error {
	if s.runtime == nil {
		return fmt.Errorf("robot runtime is not configured")
	}

	module, err := robotmodule.New(s.runtime)
	if err != nil {
		return err
	}
	if err := module.Init(); err != nil {
		return err
	}
	s.module = module
	return nil
}

func (s *RobotSvr) Start() error {
	xlog.Infof("[robot] robot module started")
	return nil
}

func (s *RobotSvr) Stop() error {
	return nil
}

func (s *RobotSvr) BeforeStop() error {
	return nil
}

func (s *RobotSvr) BeforeStart() error {
	return nil
}

func (s *RobotSvr) AfterStart() error {
	return nil
}

func (s *RobotSvr) AfterStop() error {
	return nil
}

func (s *RobotSvr) OnReload() error {
	return nil
}

func (s *RobotSvr) RegisterHTTP(mux *http.ServeMux) {
	if s.module == nil {
		return
	}
	s.module.RegisterHTTP(mux)
}
