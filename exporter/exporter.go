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

package exporter

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	promNamespace = "conntrack"
	promSubSystem = "stats"
)

var metricNames = []string{
	"found",
	"invalid",
	"ignore",
	"insert",
	"insert_failed",
	"drop",
	"early_drop",
	"error",
	"search_restart",
	"count",
}

type Option func(opts *options)

func WithErrorLogWriter(w io.Writer) Option {
	return func(opts *options) { opts.errorLogWriter = w }
}

func Handler(opts ...Option) http.Handler {
	cfg := new(options)
	for _, opt := range opts {
		opt(cfg)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(newExporter(cfg))

	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

type options struct {
	errorLogWriter io.Writer
}

// exporter exports stats from the conntrack CLI. The metrics are named with
// prefix `conntrack_stats_*`.
type exporter struct {
	descriptors    map[string]*prometheus.Desc
	errorLogWriter io.Writer
}

// newExporter creates a newExporter conntrack stats exporter.
func newExporter(ops *options) *exporter {
	e := &exporter{
		descriptors:    make(map[string]*prometheus.Desc, len(metricNames)),
		errorLogWriter: ops.errorLogWriter,
	}

	for _, mn := range metricNames {
		e.descriptors[mn] = prometheus.NewDesc(
			prometheus.BuildFQName(promNamespace, promSubSystem, mn),
			"Total of conntrack "+mn,
			[]string{"cpu"},
			nil,
		)
	}

	return e
}

// Describe implements the describe method of the prometheus.Collector
// interface.
func (e *exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, g := range e.descriptors {
		ch <- g
	}
}

// Collect implements the collect method of the prometheus.Collector interface.
func (e *exporter) Collect(ch chan<- prometheus.Metric) {
	metrics, err := e.getMetrics()
	if err != nil {
		err = fmt.Errorf("error getting metrics: %w", err)

		if e.errorLogWriter != nil {
			_, _ = fmt.Fprintln(e.errorLogWriter, err)
		}

		panic(err)
	}

	for metricName, desc := range e.descriptors {
		for _, metricPerCPU := range metrics {
			cpu, ok := metricPerCPU["cpu"]
			if !ok {
				err := fmt.Errorf("no CPU in metric %+v", metricPerCPU)

				if e.errorLogWriter != nil {
					_, _ = fmt.Fprintln(e.errorLogWriter, err)
				}

				panic(err)
			}

			metricValue, ok := metricPerCPU[metricName]
			if !ok {
				continue
			}
			ch <- prometheus.MustNewConstMetric(
				desc,
				prometheus.CounterValue,
				float64(metricValue),
				strconv.Itoa(cpu),
			)
		}
	}
}

type metricsPerCPU []map[string]int

func (e *exporter) getMetrics() (metricsPerCPU, error) {
	lines, err := e.callConntrackTool()
	if err != nil {
		return nil, fmt.Errorf("error running the conntrack command: %w", err)
	}

	metrics := make(metricsPerCPU, len(lines))
ParseEachOutputLine:
	for _, line := range lines {
		matches := regex.FindAllStringSubmatch(line, -1)
		if matches == nil {
			continue ParseEachOutputLine
		}
		metric := make(map[string]int)
		for _, match := range matches {
			if len(match) != 3 {
				return nil, fmt.Errorf("len(%v) != 3", match)
			}
			key, v := match[1], match[2]
			value, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("some key=value has a non integer value: %q: %w", line, err)
			}
			metric[key] = value
		}
		if cpu, ok := metric["cpu"]; ok {
			metrics[cpu] = metric
		}
	}

	return metrics, nil
}

func (e *exporter) getGeneralCounter(cpu int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3e9)
	defer cancel()

	cmd := exec.CommandContext(ctx, "conntrack", "--count")

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error running the conntrack command with the --count flag: %w", err)
	}

	return fmt.Sprintf("cpu=%d count=%s", cpu, out), nil
}

func (e *exporter) callConntrackTool() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3e9)
	defer cancel()

	cmd := exec.CommandContext(ctx, "conntrack", "--stats")

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error running the conntrack command with the --stats flag: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))

	var lines []string

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if scanner.Err() != nil {
		return nil, fmt.Errorf("error reading the output of the conntrack command: %w", scanner.Err())
	}

	total, err := e.getGeneralCounter(len(lines))
	if err != nil {
		return nil, fmt.Errorf("error getting the count from the conntrack command: %w", err)
	}

	lines = append(lines, total)

	return lines, nil
}

var regex = regexp.MustCompile(`([a-z_]+)=(\d+)`)
