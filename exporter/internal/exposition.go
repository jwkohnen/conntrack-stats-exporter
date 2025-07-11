package internal

import (
	_ "embed"
	"io"
	"slices"
	"sort"
	"strings"
	"text/template"
)

type (
	Metrics struct {
		metrics        metrics
		fixMetricNames bool
	}

	// metrics is a map from the metric short name to a Metric.
	metrics map[string]*Metric

	Metric struct {
		Name string
		Help string
		Type string

		Samples Samples
	}

	Samples []Sample

	Sample struct {
		Labels Labels
		Value  string
	}

	Labels []Label

	Label struct {
		Key   string
		Value string
	}
)

func SamplesCmp(i, j Sample) int {
	li := len(i.Labels)
	lj := len(j.Labels)

	if x := li - lj; x != 0 {
		return x
	}

	for l := 0; l < li; l++ {
		ki := i.Labels[l].Key
		kj := j.Labels[l].Key

		if x := strings.Compare(ki, kj); x != 0 {
			return x
		}

		vi := i.Labels[l].Value
		vj := j.Labels[l].Value

		if x := strings.Compare(vi, vj); x != 0 {
			return x
		}
	}

	return strings.Compare(i.Value, j.Value)
}

func NewMetrics(fixMetricNames bool) Metrics {
	return Metrics{
		metrics:        make(metrics, len(_help)),
		fixMetricNames: fixMetricNames,
	}
}

func (mm Metrics) GetOrInit(prefix, metricType, metricName string) *Metric {
	if _, ok := mm.metrics[metricName]; ok {
		return mm.metrics[metricName]
	}

	var suffix string
	if mm.fixMetricNames {
		suffix = "_total"

		if metricType == "gauge" {
			suffix = "_current"
		}
	}

	m := &Metric{
		Name: prefix + "_" + metricName + suffix,
		Help: _help[metricName],
		Type: metricType,
	}

	mm.metrics[metricName] = m

	return m
}

func (mm Metrics) GatherScrapeErrors(prefix string, scrapeErrors *ScrapeErrors) {
	var suffix string

	if mm.fixMetricNames {
		suffix = "_total"
	}

	m := &Metric{
		Name:    prefix + "_scrape_error" + suffix,
		Help:    _help["scrape_error"],
		Type:    "counter",
		Samples: scrapeErrors.Samples(),
	}

	mm.metrics["scrape_error"] = m
}

func (mm Metrics) WriteTo(w io.Writer) (int64, error) { return mm.metrics.WriteTo(w) }

func (mm metrics) WriteTo(w io.Writer) (n int64, err error) {
	// Write out metrics sorted, because that generates less
	// headache when debugging the output à la `watch curl`.
	names := make([]string, 0, len(mm))
	for name := range mm {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		m := mm[name]

		if len(m.Samples) == 0 {
			continue
		}

		n2, err := m.WriteTo(w)
		n += n2

		if err != nil {
			return n, err
		}
	}

	return n, err
}

func (m *Metric) AddSample(labels Labels, value string) {
	m.Samples = append(
		m.Samples,
		Sample{
			Labels: labels,
			Value:  value,
		},
	)

	// TODO: this is doing nothing
	slices.SortFunc(m.Samples, SamplesCmp)
}

func (m *Metric) WriteTo(w io.Writer) (n int64, err error) {
	cw := countWriter{w: w}
	err = _expositionTmpl.Execute(&cw, m)

	return cw.count, err
}

func (ll Labels) String() string {
	labels := make([]string, len(ll))
	for i, l := range ll {
		labels[i] = l.String()
	}

	return strings.Join(labels, ",")
}

func (l Label) String() string {
	return l.Key + `="` + l.Value + `"`
}

// TODO(jwkohnen): improve help texts!
var _help = map[string]string{
	"found":          "Total of conntrack found",
	"invalid":        "Total of conntrack invalid",
	"ignore":         "Total of conntrack ignore",
	"insert":         "Total of conntrack insert",
	"insert_failed":  "Total of conntrack insert_failed",
	"drop":           "Total of conntrack drop",
	"early_drop":     "Total of conntrack early_drop",
	"error":          "Total of conntrack error",
	"search_restart": "Total of conntrack search_restart",
	"count":          "Total of conntrack count",
	"scrape_error":   "Total of error when calling/parsing conntrack command",
}

type countWriter struct {
	w     io.Writer
	count int64
}

func (c *countWriter) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	c.count += int64(n)

	return n, err
}

var (
	_expositionTmpl = template.Must(template.New("exposition").Parse(_tmpl))

	//go:embed exposition.tmpl
	_tmpl string
)
