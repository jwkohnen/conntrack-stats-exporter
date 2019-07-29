//    This file is part of conntrack-stats-exporter.
//
//    conntrack-stats-exporter is free software: you can redistribute it and/or
//    modify it under the terms of the GNU General Public License as published
//    by the Free Software Foundation, either version 3 of the License, or (at
//    your option) any later version.
//
//    conntrack-stats-exporter is distributed in the hope that it will be
//    useful, but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General
//    Public License for more details.
//
//    You should have received a copy of the GNU General Public License along
//    with conntrack-stats-exporter.  If not, see
//    <http://www.gnu.org/licenses/>.

package main

import (
	"flag"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jwkohnen/conntrack-stats-exporter/exporter"
)

func main() {
	if os.Getenv("GOGC") == "" {
		// Reduce memory overhead. This is a low performance program;
		// the CPU penalty is negligible.
		debug.SetGCPercent(10)
	}

	addr := ":9371"
	path := "/metrics"
	flag.StringVar(&path, "path", path, "metrics endpoint path")
	flag.StringVar(&addr, "addr", addr, "TCP address to listen on")
	flag.Parse()

	reg := prometheus.NewRegistry()
	reg.MustRegister(exporter.New())

	mux := http.NewServeMux()
	mux.Handle(
		path,
		newAbortHandler(
			promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
		),
	)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  3e9,
		WriteTimeout: 3e9,
	}
	err := srv.ListenAndServe()
	if err != nil {
		abort(err)
	}
}
