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
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"time"

	"github.com/jwkohnen/conntrack-stats-exporter/exporter/internal"
)

type Option func(cfg *config)

func WithErrorLogger(log func(string, ...any)) Option { return func(cfg *config) { cfg.logger = log } }
func WithNetNs(netnsList []string) Option             { return func(cfg *config) { cfg.netnsList = netnsList } }
func WithTimeout(timeout time.Duration) Option        { return func(cfg *config) { cfg.timeout = timeout } }
func WithPrefix(prefix string) Option                 { return func(cfg *config) { cfg.prefix = prefix } }

func Handler(opts ...Option) http.Handler {
	// default config values
	cfg := config{
		netnsList: []string{""},
		timeout:   3 * time.Second,
		prefix:    "conntrack_stats",
		logger:    nil,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	scrapeErrors := internal.NewScrapeErrors(cfg.netnsList)

	logger := func(string, ...any) {}
	if cfg.logger != nil {
		logger = cfg.logger
	}

	return &exporter{
		cfg:          cfg,
		scrapeErrors: scrapeErrors,
		log:          logger,
	}
}

type config struct {
	netnsList []string
	timeout   time.Duration
	prefix    string
	logger    func(string, ...any)
}

type exporter struct {
	cfg          config
	scrapeErrors *internal.ScrapeErrors
	log          func(string, ...any)
}

func (e *exporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), e.cfg.timeout)
	defer cancel()

	metrics := make(internal.Metrics, internal.NumberOfMetrics())

	for _, netns := range e.cfg.netnsList {
		err := e.gatherMetricsForNetNs(ctx, netns, metrics)
		if err != nil {
			e.log("error gathering metrics for netns %q: %v\n", netns, err)
		}
	}

	metrics.GatherScrapeErrors(e.cfg.prefix, e.scrapeErrors)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	_, err := metrics.WriteTo(w)
	if err != nil {
		e.log("error writing metrics to response writer: %v\n", err)
		return
	}
}

func (e *exporter) gatherMetricsForNetNs(ctx context.Context, netns string, metrics internal.Metrics) error {
	statsOutput, countOutput, err := e.execConntrackTool(ctx, netns)
	if err != nil {
		return err
	}

	matches := _regex.FindAllSubmatch(statsOutput, -1)

	if len(matches) == 0 {
		return e.scrapeErrors.Count(netns, internal.OpToolOutputNoMatch, nil)
	}

	for _, match := range matches {
		var cpu string

		for i, metricShortName := range _regex.SubexpNames() {
			value := match[i]

			switch metricShortName {
			case "":
				// skip full match
				continue
			case "cpu":
				cpu = string(value)
			default:
				metrics.GetOrInit(e.cfg.prefix, "counter", metricShortName).AddSample(
					internal.Labels{
						{
							Key:   "cpu",
							Value: cpu,
						},
						{
							Key:   "netns",
							Value: netns,
						},
					},
					string(value),
				)
			}
		}
	}

	metrics.GetOrInit(e.cfg.prefix, "gauge", "count").AddSample(
		internal.Labels{
			internal.Label{
				Key:   "netns",
				Value: netns,
			},
		},
		countOutput,
	)

	return nil
}

func (e *exporter) execConntrackTool(
	ctx context.Context,
	netns string,
) (
	statsOutput []byte,
	countOutput string,
	err error,
) {
	for _, fn := range []func() error{
		func() error {
			var err error

			statsOutput, err = getConntrackStats(ctx)
			return err
		},
		func() error {
			var err error

			countOutput, err = getConntrackCounter(ctx)
			return err
		},
	} {
		var errExec error

		errNs := e.execInNetns(netns, func() { errExec = fn() })

		if errNs != nil {
			return nil, "", fmt.Errorf("error executing in netns %q: %w", netns, errNs)
		}

		ctxErr := ctx.Err()

		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return nil, "", e.scrapeErrors.Count(
				netns,
				internal.OpTimeout,
				errExec,
			)
		}

		if errors.Is(ctxErr, context.Canceled) {
			return nil, "", e.scrapeErrors.Count(
				netns,
				internal.OpClientGone,
				errExec,
			)
		}

		if errExec != nil {
			return nil, "", e.scrapeErrors.Count(
				netns,
				internal.OpExecTool,
				fmt.Errorf("failed to exec conntrack tool: %w", errExec),
			)
		}
	}

	return statsOutput, countOutput, nil
}

func getConntrackCounter(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "conntrack", "--count")

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error running the conntrack command with the --count flag: %w", err)
	}

	return string(bytes.TrimSpace(out)), nil
}

func getConntrackStats(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "conntrack", "--stats")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error running the conntrack command with the --stats flag: %w", err)
	}

	return output, nil
}

var _regex = regexp.MustCompile(`` +
	`(?m)` +
	`cpu=(?P<cpu>\d+)\s+` +
	`found=(?P<found>\d+)\s+` +
	`invalid=(?P<invalid>\d+)\s+` +
	`insert=(?P<insert>\d+)\s+` +
	`insert_failed=(?P<insert_failed>\d+)\s+` +
	`drop=(?P<drop>\d+)\s+` +
	`early_drop=(?P<early_drop>\d+)\s+` +
	`error=(?P<error>\d+)\s+` +
	`search_restart=(?P<search_restart>\d+)`,
)
