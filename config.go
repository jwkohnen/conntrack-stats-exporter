package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jwkohnen/conntrack-stats-exporter/exporter"
)

type config struct {
	addr             string
	path             string
	netns            []string
	prefix           string
	quiet            bool
	timeoutGathering time.Duration
	timeoutShutdown  time.Duration
	timeoutHTTP      time.Duration
	fixMetricNames   bool
	logf             func(string, ...any)
}

func configure() (config, []exporter.Option) {
	// default values
	c := config{
		addr:             ":9371",
		path:             "/metrics",
		prefix:           "conntrack_stats",
		quiet:            false,
		timeoutGathering: time.Second * 5,
		timeoutShutdown:  time.Second * 3,
		timeoutHTTP:      time.Second * 10,
		fixMetricNames:   false,
		logf:             func(string, ...any) {},
	}

	var (
		tmpNetns string
	)

	var fs = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.StringVar(&c.path, "path", c.path, "metrics endpoint path")
	fs.StringVar(&c.addr, "addr", c.addr, "TCP address to listen on")
	fs.StringVar(&c.prefix, "prefix", c.prefix, "metrics prefix")
	fs.BoolVar(&c.quiet, "quiet", c.quiet, "don't log anything")
	fs.DurationVar(&c.timeoutGathering, "timeout-gathering", c.timeoutGathering, "timeout for gathering metrics")
	fs.DurationVar(&c.timeoutShutdown, "timeout-shutdown", c.timeoutShutdown, "timeout for graceful shutdown")
	fs.DurationVar(&c.timeoutHTTP, "timeout-http", c.timeoutHTTP, "timeout for HTTP requests")
	fs.BoolVar(&c.fixMetricNames, "fix-metric-names", c.fixMetricNames, "fix historic metric name choices")
	fs.StringVar(&tmpNetns, "netns", "", "List of netns names separated by comma")

	_ = fs.Parse(os.Args[1:])

	c.netns = strings.Split(tmpNetns, ",")

	if !c.quiet {
		c.logf = log.New(os.Stderr, "", 0).Printf
	}

	opts := []exporter.Option{
		exporter.WithNetNs(c.netns),
		exporter.WithTimeout(c.timeoutGathering),
		exporter.WithPrefix(c.prefix),
		exporter.WithErrorLogger(c.logf),
	}

	if c.fixMetricNames {
		opts = append(opts, exporter.WithFixMetricNames())
	}

	return c, opts
}
