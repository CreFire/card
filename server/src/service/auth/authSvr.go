package main

import (
	"fmt"
	"net/http"

	"backend/deps/xlog"
	"backend/src/common/server"
	authmodule "backend/src/service/auth/module"
)

var authSvr = NewAuthSvr()
var _ server.Service = (*AuthSvr)(nil)
var _ server.RuntimeAware = (*AuthSvr)(nil)
var _ server.HTTPRouteRegistrar = (*AuthSvr)(nil)

type AuthSvr struct {
	runtime *server.Runtime
	module  *authmodule.AuthModule
}

func NewAuthSvr() *AuthSvr {
	return &AuthSvr{}
}

func (s *AuthSvr) SetRuntime(runtime *server.Runtime) error {
	s.runtime = runtime
	return nil
}

func (s *AuthSvr) OnInit() error {
	if s.runtime == nil {
		return fmt.Errorf("auth runtime is not configured")
	}

	module, err := authmodule.New(s.runtime)
	if err != nil {
		return err
	}
	if err := module.Init(); err != nil {
		return err
	}
	s.module = module
	return nil
}

func (s *AuthSvr) Start() error {
	xlog.Infof("[auth] auth module started")
	return nil
}

func (s *AuthSvr) Stop() error {
	return nil
}

func (s *AuthSvr) BeforeStop() error {
	return nil
}

func (s *AuthSvr) BeforeStart() error {
	return nil
}

func (s *AuthSvr) AfterStart() error {
	return nil
}

func (s *AuthSvr) AfterStop() error {
	return nil
}

func (s *AuthSvr) OnReload() error {
	return nil
}

func (s *AuthSvr) RegisterHTTP(mux *http.ServeMux) {
	if s.module == nil {
		return
	}
	s.module.RegisterHTTP(mux)
}
