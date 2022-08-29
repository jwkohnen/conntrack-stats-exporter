package internal

import (
	"io"
	"strings"
	"text/template"
)

type (
	// Metrics is a map from the metric short name to a Metric.
	Metrics map[string]*Metric

	Metric struct {
		Name string
		Help string

		Samples Samples
	}

	Samples []Sample

	Sample struct {
		Labels Labels
		Count  string
	}

	Labels []Label

	Label struct {
		Key   string
		Value string
	}
)

func (mm Metrics) GetOrInit(prefix, metricName string) *Metric {
	if _, ok := mm[metricName]; ok {
		return mm[metricName]
	}

	m := &Metric{
		Name: prefix + "_" + metricName,
		Help: _help[metricName],
	}

	mm[metricName] = m

	return m
}

func (mm Metrics) GatherScrapeErrors(prefix string, scrapeErrors *ScrapeErrors) {
	m := &Metric{
		Name:    prefix + "_scrape_error",
		Help:    _help["scrape_error"],
		Samples: scrapeErrors.Samples(),
	}

	mm["scrape_error"] = m
}

func (mm Metrics) WriteTo(w io.Writer) (n int64, err error) {
	for _, metric := range mm {
		if len(metric.Samples) == 0 {
			continue
		}

		n2, err := metric.WriteTo(w)
		n += n2

		if err != nil {
			return n, err
		}
	}

	return n, err
}

func (m *Metric) AddSample(labels Labels, count string) {
	m.Samples = append(
		m.Samples,
		Sample{
			Labels: labels,
			Count:  count,
		},
	)
}

func (m *Metric) WriteTo(w io.Writer) (n int64, err error) {
	cw := &countWriter{w: w}
	err = _tmpl.Execute(cw, m)

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

func NumberOfMetrics() int { return len(_help) }

// TODO(jwkohnen): improve help texts!
var _help = map[string]string{
	"found":          "Total of conntrack found",
	"invalid":        "Total of conntrack invalid",
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

var _tmpl = template.Must(
	template.New("metric").Parse(
		"# HELP {{ $.Name }} {{ $.Help }}\n" +
			"# TYPE {{ $.Name }} counter\n" +
			"{{ range $.Samples }}" +
			"{{ $.Name }}{{ `{` }}{{ .Labels.String }}{{ `}` }} {{ .Count }}\n" +
			"{{ end }}",
	),
)
