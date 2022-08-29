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
	"bytes"
	"fmt"
	"net/http"
	"os"
	"runtime"
)

// newAbortHandler augments a handler, so it escalates any panic to termination
// of the process.
func newAbortHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					abort(err)
				}
			}()
			next.ServeHTTP(w, r)
		},
	)
}

// abort writes at most 2kB of the error message and a stacktrace of the
// current goroutine to STDERR, then terminates the process with exit code 1.
//
// Kubernetes' termination log is limited to 2 kB or 80 lines, whichever is
// smaller. Kubernetes will only log the last bytes, not the first.  Hence, it
// is important to write not more than the limit. Most likely the first lines
// are most valuable for debugging.
//
// pod.spec.containers.terminationMessagePolicy should be set to
// FallbackToLogsOnError.
func abort(err interface{}) {
	const size = 2 << 10

	buf := bytes.NewBuffer(make([]byte, 0, 2*size))

	_, _ = fmt.Fprintf(buf, "ERROR: %v\n\n", err)

	stack := make([]byte, size)
	stack = stack[:runtime.Stack(stack, false)] // https://i.imgur.com/hb2a9kI.jpg
	_, _ = buf.Write(stack)

	_, _ = os.Stderr.Write(buf.Next(size))

	os.Exit(1)
}
