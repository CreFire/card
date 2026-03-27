package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"backend/deps/xlog"
	"backend/src/common/configdoc"
	"backend/src/common/discovery"
)

func runHTTP(ctx context.Context, serviceName string, cfg *configdoc.ConfigBase, registry *discovery.Registry, services ...Service) error {
	started := make([]Service, 0, len(services))
	for _, svc := range services {
		if svc == nil {
			continue
		}
		if err := svc.OnInit(); err != nil {
			return fmt.Errorf("init service %s: %w", serviceName, err)
		}
		if err := svc.BeforeStart(); err != nil {
			return fmt.Errorf("before start service %s: %w", serviceName, err)
		}
		if err := svc.Start(); err != nil {
			return fmt.Errorf("start service %s: %w", serviceName, err)
		}
		if err := svc.AfterStart(); err != nil {
			return fmt.Errorf("after start service %s: %w", serviceName, err)
		}
		started = append(started, svc)
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddress(),
		Handler:           buildHTTPHandler(serviceName, cfg, registry, services...),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			_ = stopServices(serviceName, started)
			return err
		}
		return stopServices(serviceName, started)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = stopServices(serviceName, started)
		return fmt.Errorf("shutdown server: %w", err)
	}

	if err := stopServices(serviceName, started); err != nil {
		return err
	}
	return nil
}

func buildHTTPHandler(serviceName string, cfg *configdoc.ConfigBase, registry *discovery.Registry, services ...Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"service": serviceName,
			"status":  "ok",
			"env":     cfg.App.Env,
			"address": cfg.ListenAddress(),
		})
	})

	mux.HandleFunc("/discover/", func(w http.ResponseWriter, r *http.Request) {
		if registry == nil || !cfg.Etcd.EnableDiscovery {
			http.Error(w, "discovery is disabled", http.StatusNotImplemented)
			return
		}

		targetService := r.URL.Path[len("/discover/"):]
		if targetService == "" {
			http.Error(w, "service name is required", http.StatusBadRequest)
			return
		}

		instances, err := registry.Discover(r.Context(), targetService)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"service":   targetService,
			"instances": instances,
		})
	})

	for _, svc := range services {
		registrar, ok := svc.(HTTPRouteRegistrar)
		if !ok {
			continue
		}
		registrar.RegisterHTTP(mux)
	}

	return requestLogMiddleware(serviceName, mux)
}

func requestLogMiddleware(serviceName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		xlog.Infof("[%s] %s %s", serviceName, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		xlog.Errorf("write response: %v", err)
	}
}
