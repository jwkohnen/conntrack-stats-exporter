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
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"

	"github.com/jwkohnen/conntrack-stats-exporter/exporter"
)

func main() {
	// Set Go max procs to 1 even if number of (logical) CPUs is > 1.  This is a low performance program that might run
	// in an environment with very limited CPU resources via cgroups (e.g. Kubernetes resource limit).  GOMAXPROCS of 1
	// prevents the Go scheduler from using too much scheduler overhead in such environments.
	//
	// Usually I'd use go.uber.org/automaxprocs/maxprocs, but hard coding 1 is a better solution than having another
	// dependency.
	_ = runtime.GOMAXPROCS(1)

	if os.Getenv("GOGC") == "" {
		// Reduce memory overhead. This is a low performance program;
		// the CPU penalty is negligible.
		debug.SetGCPercent(10)
	}

	cfg, opts := configure()

	const procPath = "/proc/net/stat/nf_conntrack"
	if !cfg.quiet && checkProc(procPath) {
		cfg.logf("HINT: the file %q is available, you may use prometheus/node_exporter instead.", procPath)
	}

	mux := http.NewServeMux()
	mux.Handle(cfg.path, newAbortHandler(exporter.Handler(opts...)))

	srv := &http.Server{
		Addr:         cfg.addr,
		Handler:      mux,
		ReadTimeout:  cfg.timeoutHTTP,
		WriteTimeout: cfg.timeoutHTTP,
	}

	shutdown := make(chan os.Signal, 1)

	var (
		receivedSignal os.Signal
		wg             sync.WaitGroup
	)

	wg.Add(1)

	go func() {
		defer wg.Done()

		// Sadly Kubernetes sends SIGTERM, not SIGINT.  CTRL+C on a TTY sends SIGINT.
		signal.Notify(shutdown, os.Interrupt)
		signal.Notify(shutdown, syscall.SIGTERM)

		receivedSignal = <-shutdown

		signal.Stop(shutdown)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.timeoutShutdown)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			abort(fmt.Errorf("error shutting down server: %w", err))
		}
	}()

	cfg.logf("listening on %s with endpoint %q\n", cfg.addr, cfg.path)

	err := srv.ListenAndServe()

	wg.Wait()

	if errors.Is(err, http.ErrServerClosed) {
		const signaledExitCodeBase = 128

		os.Exit(signaledExitCodeBase + int(receivedSignal.(syscall.Signal)))
	}

	if err != nil {
		abort(err)
	}
}
