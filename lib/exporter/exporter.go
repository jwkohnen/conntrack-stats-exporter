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
	"os/exec"
	"regexp"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
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

type metricList []map[string]int

// Exporter exports stats from the conntrack CLI. The metrics are named with
// prefix `conntrack_stats_*`.
type Exporter struct {
	descriptors map[string]*prometheus.Desc
	scrapeError map[string]int
	netnsList   []string
}

// New creates a new conntrack stats exporter.
func New(netnsList []string) *Exporter {
	scrapeError := make(map[string]int, len(netnsList))
	for _, netns := range netnsList {
		scrapeError[netns] = 0
	}
	e := &Exporter{descriptors: make(map[string]*prometheus.Desc, len(metricNames)), scrapeError: scrapeError, netnsList: netnsList}
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
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, g := range e.descriptors {
		ch <- g
	}
}

// Collect implements the collect method of the prometheus.Collector interface.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	metricsPerNetns := make(map[string]metricList)
	var err error
	for _, netns := range e.netnsList {
		metricsPerNetns[netns], err = getMetrics(netns)
		if err != nil {
			e.scrapeError[netns]++
			log.Errorf("failed to get conntrack metrics netns: %s", err)
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

func getMetrics(netns string) (metricList, error) {
	var lines []string
	var err error
	err = execInNetns(netns, func() error {
		lines, err = getConntrackStats()
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get conntrack stats: %s", err)
	}
	total, err := getConntrackCounter()
	if err != nil {
		return nil, fmt.Errorf("failed to get conntrack stats: %s", err)
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
				return nil, fmt.Errorf("some key=value has a non integer value: %q", line)
			}
			metric[key] = value
		}
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func getConntrackCounter() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3e9)
	defer cancel()

	cmd := exec.CommandContext(ctx, "conntrack", "--count")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error happened while calling conntrack: %s", err)
	}

	return fmt.Sprintf("count=%s", out), nil
}

func getConntrackStats() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3e9)
	defer cancel()

	cmd := exec.CommandContext(ctx, "conntrack", "--stats")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error happened while calling conntrack: %s", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if scanner.Err() != nil {
		return nil, fmt.Errorf("error while parsing conntrack output: %s", err)
	}
	return lines, nil
}
