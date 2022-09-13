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

package exporter_test

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"testing"

	"github.com/jwkohnen/conntrack-stats-exporter/exporter"
)

func TestConntrackMock(t *testing.T) {
	mockConntrackTool(t)

	out, err := exec.Command("conntrack", "--version").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, []byte("conntrack v0.0.0-mock (conntrack-stats-exporter)\n")) {
		t.Error("whatever it is we've executed, it was not our mock script")
	}
}

func TestMetrics(t *testing.T) {
	mockConntrackTool(t)

	recorder := httptest.NewRecorder()
	exporter.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", http.NoBody))

	resp := recorder.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}

	for metricName, cpuValues := range map[string][4]int{
		// not checking "conntrack_stats_count" here, because it has a CPU label by accident and that is subject to change
		"conntrack_stats_drop":           {3, 8, 13, 0},
		"conntrack_stats_early_drop":     {4, 9, 14, 0},
		"conntrack_stats_error":          {5, 10, 2, 0},
		"conntrack_stats_found":          {13, 6, 16, 15},
		"conntrack_stats_insert":         {1, 6, 11, 0},
		"conntrack_stats_insert_failed":  {2, 7, 12, 0},
		"conntrack_stats_invalid":        {11258, 10298, 17439, 12065},
		"conntrack_stats_search_restart": {76531, 64577, 75364, 66740},
	} {
		metricName, cpuValues := metricName, cpuValues

		t.Run("Header+Type+Metric: "+metricName, func(t *testing.T) {
			t.Parallel()

			// Apologies for using regex! As a reminder (?m) enables multi-line mode.  This regex is supposed to make
			// sure the metric is prepended by a HELP as well as a TYPE header and that each type is `counter`.
			regex := regexp.MustCompile(
				fmt.Sprintf(
					`(?m)`+
						`^# HELP %s.*?\n`+
						`^# TYPE %s counter\n`+
						`^%s\{`,
					metricName, metricName, metricName,
				),
			)
			if !regex.Match(body) {
				t.Errorf("expected to find HELP header, TYPE header and metric for %s, but didn't", metricName)
			}
		})

		for cpu, cpuValue := range cpuValues {
			cpu, cpuValue := cpu, cpuValue

			t.Run(fmt.Sprintf("%s{cpu=%d}", metricName, cpu), func(t *testing.T) {
				t.Parallel()

				// Again, apologies for using regex.  This regex is supposed to match the metric line for a given CPU
				// and its value.  The group should match a metric with only cpu label, any label prepending the cpu
				// label as well as any label following the cpu label.  If adding any label to the metric, this regex
				// should match or this is a bug!
				regex := regexp.MustCompile(fmt.Sprintf(`(?m)^%s\{(?:[^{}]+?,|)cpu="%d".*\} %d$`,
					metricName, cpu, cpuValue,
				))

				if !regex.Match(body) {
					t.Errorf("expected to find metric for %s{cpu=%d}, but didn't", metricName, cpu)
				}
			})
		}
	}

	t.Run("conntrack_stats_count", func(t *testing.T) {
		t.Parallel()

		// A regex again, but this one is not too bad, or is it?
		regex := regexp.MustCompile(`(?m)^conntrack_stats_count({.+?}|) 434$`)

		if !regex.Match(body) {
			t.Errorf("expected to find conntrack_stats_count, but didn't")
		}
	})
}

// TestScrapeError tests that the exporter counts scrape errors correctly. Also, it runs a bunch of requests in parallel
// in order to provoke the race detector.
func TestScrapeError(t *testing.T) {
	mockConntrackTool(t)

	t.Setenv("CONNTRACK_STATS_EXPORTER_KAPUTT", "true")

	handler := exporter.Handler()

	request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	const preload = 50

	wg := new(sync.WaitGroup)
	wg.Add(preload)

	for i := 0; i < preload; i++ {
		go func() {
			defer wg.Done()
			handler.ServeHTTP(new(nilResponseWriter), request)
		}()
	}
	wg.Wait()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	resp := recorder.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}

	regex := regexp.MustCompile(`(?m)^conntrack_stats_scrape_error({.+?}|) ` + strconv.Itoa(preload+1) + `$`)

	if !regex.Match(body) {
		t.Errorf("expected to find conntrack_stats_scrape_error with count %d, but didn't", preload+1)
	}
}

func mockConntrackTool(tb testing.TB) {
	tb.Helper()

	if len(conntrackMockScript) == 0 {
		tb.Fatal("conntrackMockScript is empty")
	}

	dir := tb.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "conntrack"), conntrackMockScript, 0755); err != nil {
		tb.Fatal(err)
	}

	tb.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

type nilResponseWriter struct{}

func (nilResponseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (nilResponseWriter) Header() http.Header         { return http.Header{} }
func (nilResponseWriter) WriteHeader(int)             {}

//go:embed conntrack_mock.sh
var conntrackMockScript []byte
