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
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	promNamespace = "conntrack"
	promSubSystem = "stats"
)

var regex = regexp.MustCompile(`([a-z_]+)=(\d+)`)

var metricNames = map[string][]string{
	"found":          {"cpu", "netns"},
	"invalid":        {"cpu", "netns"},
	"ignore":         {"cpu", "netns"},
	"insert":         {"cpu", "netns"},
	"insert_failed":  {"cpu", "netns"},
	"drop":           {"cpu", "netns"},
	"early_drop":     {"cpu", "netns"},
	"error":          {"cpu", "netns"},
	"search_restart": {"cpu", "netns"},
	"count":          {"netns"},
}

type Option func(cfg *config)

type metricList []map[string]uint64

func WithErrorLogWriter(w io.Writer) Option {
	return func(opts *config) { opts.errWriter = w }
}

func WithNetNs(netnsList []string) Option {
	return func(opts *config) { opts.netnsList = netnsList }
}

func Handler(opts ...Option) http.Handler {
	cfg := &config{
		errWriter: nil,
		netnsList: []string{""},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(newExporter(cfg))

	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

type config struct {
	errWriter io.Writer
	netnsList []string
}

// exporter exports stats from the conntrack CLI. The metrics are named with
// prefix `conntrack_stats_*`.
type exporter struct {
	descriptors map[string]*prometheus.Desc
	errWriter   io.Writer
	scrapeError map[string]*uint64
	netnsList   []string
}

// newExporter creates a newExporter conntrack stats exporter.
func newExporter(cfg *config) *exporter {
	scrapeError := make(map[string]*uint64, len(cfg.netnsList))

	for _, netns := range cfg.netnsList {
		se := uint64(0)
		scrapeError[netns] = &se
	}

	e := &exporter{
		descriptors: make(map[string]*prometheus.Desc, len(metricNames)),
		errWriter:   cfg.errWriter,
		scrapeError: scrapeError,
		netnsList:   cfg.netnsList,
	}
	e.descriptors["scrape_error"] = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, promSubSystem, "scrape_error"),
		"Total of error when calling/parsing conntrack command",
		[]string{"netns"},
		nil,
	)

	for metricName, metricLabels := range metricNames {
		e.descriptors[metricName] = prometheus.NewDesc(
			prometheus.BuildFQName(promNamespace, promSubSystem, metricName),
			"Total of conntrack "+metricName,
			metricLabels,
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
	metricsPerNetns := make(map[string]metricList)

	var err error
	for _, netns := range e.netnsList {
		metricsPerNetns[netns], err = e.getMetrics(netns)
		if err != nil {
			atomic.AddUint64(e.scrapeError[netns], 1)

			err = fmt.Errorf("error getting metrics: %w", err)

			if e.errWriter != nil {
				_, _ = fmt.Fprintln(e.errWriter, err)
			}
		}

		metricsPerNetns[netns] = append(
			metricsPerNetns[netns],
			map[string]uint64{
				"scrape_error": atomic.LoadUint64(e.scrapeError[netns]),
			},
		)
	}

	for metricName, desc := range e.descriptors {
		for netns, metrics := range metricsPerNetns {
			for _, metric := range metrics {
				metricValue, ok := metric[metricName]
				if !ok {
					continue
				}

				labels := []string{netns}

				cpu, ok := metric["cpu"]
				if ok {
					labels = append([]string{strconv.FormatUint(cpu, 10)}, labels...)
				}
				ch <- prometheus.MustNewConstMetric(
					desc,
					prometheus.CounterValue,
					float64(metricValue),
					labels...,
				)
			}
		}
	}
}

func (e *exporter) getMetrics(netns string) (metricList, error) {
	var (
		lines   []string
		total   string
		errExec error
		errNs   error
	)

	errNs = execInNetns(netns, func() { lines, errExec = e.getConntrackStats() })
	if errNs != nil {
		return nil, fmt.Errorf("error executing in netns %q: %w", netns, errNs)
	}

	if errExec != nil {
		return nil, fmt.Errorf("failed to get conntrack stats: %w", errNs)
	}

	errNs = execInNetns(netns, func() { total, errExec = e.getConntrackCounter() })
	if errNs != nil {
		return nil, fmt.Errorf("error executing in netns %q: %w", netns, errNs)
	}

	if errExec != nil {
		return nil, fmt.Errorf("failed to get conntrack counter: %w", errExec)
	}

	lines = append(lines, total)
	metrics := make(metricList, 0, len(lines))
ParseEachOutputLine:
	for _, line := range lines {
		matches := regex.FindAllStringSubmatch(line, -1)
		if matches == nil {
			continue ParseEachOutputLine
		}
		metric := make(map[string]uint64)
		for _, match := range matches {
			if len(match) != 3 {
				return nil, fmt.Errorf("len(%v) != 3", match)
			}
			key, v := match[1], match[2]
			value, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("some key=value has a non integer value: %q: %w", line, err)
			}
			metric[key] = value
		}
		metrics = append(metrics, metric)
	}

	return metrics, nil
}

func (e *exporter) getConntrackCounter() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3e9)
	defer cancel()

	cmd := exec.CommandContext(ctx, "conntrack", "--count")

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error running the conntrack command with the --count flag: %w", err)
	}

	return fmt.Sprintf("count=%s", out), nil
}

func (e *exporter) getConntrackStats() ([]string, error) {
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

	return lines, nil
}
