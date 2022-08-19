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

type Option func(opts *options)

type metricList []map[string]int

func WithErrorLogWriter(w io.Writer) Option {
	return func(opts *options) { opts.errorLogWriter = w }
}

func WithNetNs(netnsList []string) Option {
	return func(opts *options) { opts.netnsList = netnsList }
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
	netnsList      []string
}

// exporter exports stats from the conntrack CLI. The metrics are named with
// prefix `conntrack_stats_*`.
type exporter struct {
	descriptors    map[string]*prometheus.Desc
	errorLogWriter io.Writer
	scrapeError    map[string]int
	netnsList      []string
}

// newExporter creates a newExporter conntrack stats exporter
func newExporter(ops *options) *exporter {
	scrapeError := make(map[string]int, len(ops.netnsList))
	for _, netns := range ops.netnsList {
		scrapeError[netns] = 0
	}
	e := &exporter{descriptors: make(map[string]*prometheus.Desc, len(metricNames)), errorLogWriter: ops.errorLogWriter, scrapeError: scrapeError, netnsList: ops.netnsList}
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
			e.scrapeError[netns]++
			err = fmt.Errorf("error getting metrics: %w", err)

			if e.errorLogWriter != nil {
				_, _ = fmt.Fprintln(e.errorLogWriter, err)
			}
		}
		metricsPerNetns[netns] = append(metricsPerNetns[netns], map[string]int{"scrape_error": e.scrapeError[netns]})
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
					labels = append([]string{strconv.Itoa(cpu)}, labels...)
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
	var lines []string
	var total string
	var err error
	err = execInNetns(netns, func() error {
		lines, err = e.getConntrackStats()
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get conntrack stats: %s", err)
	}
	err = execInNetns(netns, func() error {
		total, err = e.getConntrackCounter()
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get conntrack counter: %s", err)
	}
	lines = append(lines, total)
	metrics := make(metricList, len(lines))
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
	fmt.Println(out)

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
