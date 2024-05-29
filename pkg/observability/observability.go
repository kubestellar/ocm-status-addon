/*
Copyright 2024 The KubeStellar Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package observability

import (
	"context"
	"net"
	"net/http"

	"k8s.io/apiserver/pkg/server/mux"
	"k8s.io/apiserver/pkg/server/routes"
	"k8s.io/component-base/metrics/legacyregistry"
	_ "k8s.io/component-base/metrics/prometheus/clientgo"
	_ "k8s.io/component-base/metrics/prometheus/version"
	"k8s.io/klog/v2"
)

type FlagSet interface {
	Float64Var(p *float64, name string, value float64, usage string)
	IntVar(p *int, name string, value int, usage string)
	StringVar(p *string, name string, value string, usage string)
}

// ObservabilityOptions covers offering Prometheus metrics and /debug/pprof .
type ObservabilityOptions[FS FlagSet] struct {

	// MetricsBindAddr is the local `:$port` or `$host:$port`
	// that the listening socket gets bound to.
	// More specifically, this is the sort of string that can
	// be used as the `Addr` in a `net/http.Server`.
	MetricsBindAddr string

	// PprofBindAddr is the local `:$port` or `$host:$port`
	// that the listening socket gets bound to.
	// More specifically, this is the sort of string that can
	// be used as the `Addr` in a `net/http.Server`.
	PprofBindAddr string
}

func (opts *ObservabilityOptions[FS]) AddToFlagSet(flags FS) {
	flags.StringVar(&opts.MetricsBindAddr, "metrics-bind-addr", opts.MetricsBindAddr, "[host]:port at which to listen for HTTP requests for Prometheus /metrics requests")
	flags.StringVar(&opts.PprofBindAddr, "pprof-bind-addr", opts.PprofBindAddr, "[host]:port at which to listen for HTTP requests for go /debug/pprof requests")
}

func (opts *ObservabilityOptions[FS]) StartServing(ctx context.Context) {
	logger := klog.FromContext(ctx)
	go func() {
		metricsServer := http.Server{
			Addr:        opts.MetricsBindAddr,
			Handler:     legacyregistry.Handler(),
			BaseContext: func(net.Listener) context.Context { return ctx },
		}
		err := metricsServer.ListenAndServe()
		if err != nil {
			logger.Error(err, "Failed to serve Prometheus metrics", "bindAddress", opts.MetricsBindAddr)
			panic(err)
		}
	}()

	go func() {
		mymux := mux.NewPathRecorderMux("transport-controller")
		pprofServer := http.Server{
			Addr:        opts.PprofBindAddr,
			Handler:     mymux,
			BaseContext: func(net.Listener) context.Context { return ctx },
		}
		routes.Profiling{}.Install(mymux)
		err := pprofServer.ListenAndServe()
		if err != nil {
			logger.Error(err, "Failure in serving /debug/pprof", "bindAddress", opts.PprofBindAddr)
			panic(err)
		}
	}()
}
