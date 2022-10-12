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
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jwkohnen/conntrack-stats-exporter/exporter"
	"github.com/jwkohnen/conntrack-stats-exporter/exporter/internal"
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

func TestPromtoolCheckMetrics(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		t.Skip("Skipping test because promtool is not in $PATH")
	}

	mockConntrackTool(t)

	var (
		handler  = exporter.Handler(exporter.WithFixMetricNames())
		recorder = httptest.NewRecorder()
		request  = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	)

	handler.ServeHTTP(recorder, request)

	body := recorder.Body.Bytes()

	if len(bytes.TrimSpace(body)) == 0 {
		t.Fatal("empty response body")
	}

	cmd := exec.Command(promtool, "check", "metrics")
	cmd.Stdin = bytes.NewReader(body)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("promtool check metrics failed: %v", err)
	}

	if len(bytes.TrimSpace(out)) > 0 {
		t.Log(string(out))
		t.Logf("response:\n%s", string(body))
	}
}

func testMetrics(t *testing.T, want map[string][]int) {
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

	for metricName, cpuValues := range want {
		metricName, cpuValues := metricName, cpuValues

		t.Run("Header+Type+Metric: "+metricName, func(t *testing.T) {
			// Apologies for using regex! As a reminder (?m) enables multi-line mode.  This regex is supposed to make
			// sure the metric is prepended by a HELP as well as a TYPE header and that each type is `counter`.
			regex := regexp.MustCompile(
				fmt.Sprintf(
					`(?m)`+
						`^# HELP %s .*?\n`+
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
		// A regex again, but this one is not too bad, or is it?
		regex := regexp.MustCompile(`(?m)` +
			`^# HELP conntrack_stats_count .*?\n` +
			`^# TYPE conntrack_stats_count gauge\n` +
			`^conntrack_stats_count({.+?}) 434$`)

		if !regex.Match(body) {
			t.Errorf("expected to find conntrack_stats_count, but didn't")
		}
	})

	t.Run("conntrack_stats_scrape_error", func(t *testing.T) {
		// A regex again, but this one is not too bad, or is it?
		regex := regexp.MustCompile(`(?m)` +
			`^# HELP conntrack_stats_scrape_error .*?\n` +
			`^# TYPE conntrack_stats_scrape_error counter\n` +
			`^conntrack_stats_scrape_error({.+?}) \d+$`)

		if !regex.Match(body) {
			t.Errorf("expected to find conntrack_stats_scrape_error, but didn't")
		}
	})

	if t.Failed() {
		t.Log(string(body))
	}
}

func TestMetrics(t *testing.T) {
	mockConntrackTool(t)

	testMetrics(t, map[string][]int{
		// not checking "conntrack_stats_count" here, because it has a CPU label by accident and that is subject to change
		"conntrack_stats_drop":           {3, 8, 13, 0},
		"conntrack_stats_early_drop":     {4, 9, 14, 0},
		"conntrack_stats_error":          {5, 10, 2, 0},
		"conntrack_stats_found":          {13, 6, 16, 15},
		"conntrack_stats_insert":         {1, 6, 11, 0},
		"conntrack_stats_insert_failed":  {2, 7, 12, 0},
		"conntrack_stats_invalid":        {11258, 10298, 17439, 12065},
		"conntrack_stats_search_restart": {76531, 64577, 75364, 66740},
	})
}

func TestIssue19(t *testing.T) {
	mockConntrackTool(t)

	t.Setenv("CONNTRACK_STATS_EXPORTER_ISSUE_19", "true")

	testMetrics(t, map[string][]int{
		"conntrack_stats_found": {50, 28, 32, 20, 19, 20, 37, 22, 13, 43, 32, 41, 50, 40, 66, 49},
		"conntrack_stats_invalid": {43271, 29808, 31119, 31296, 30548, 30001, 31807, 29568, 31952, 42525, 44387, 42617,
			43855, 42069, 43443, 42856},
		"conntrack_stats_ignore": {26302191, 8379307, 26088734, 8265418, 26002588, 8340823, 25968413, 8430048,
			25949280, 8652643, 26330305, 8881758, 26362895, 8843429, 26356212, 8764532},
		"conntrack_stats_insert":        {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		"conntrack_stats_insert_failed": {30, 9, 5, 1, 2, 2, 16, 1, 1, 17, 32, 53, 37, 34, 44, 34},
		"conntrack_stats_drop":          {30, 9, 5, 1, 2, 2, 16, 1, 1, 17, 32, 53, 37, 34, 44, 34},
		"conntrack_stats_early_drop":    {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		"conntrack_stats_error":         {24, 5, 25, 23, 22, 13, 23, 9, 27, 30, 28, 17, 34, 17, 33, 32},
		"conntrack_stats_search_restart": {1053423, 455902, 914079, 446014, 943922, 465287, 950686, 451268, 927906,
			527808, 1025931, 547542, 1006812, 543103, 1009081, 526916},
	})
}

// TestScrapeError tests that the exporter counts scrape errors correctly. Also, it runs a bunch of requests in parallel
// in order to provoke the race detector.
//
//nolint:funlen
func TestScrapeError(t *testing.T) {
	mockConntrackTool(t)

	const concurrency = 50

	var (
		errorLogBuf = new(bytes.Buffer)

		srv = httptest.NewServer(
			exporter.Handler(
				exporter.WithTimeout(500*time.Millisecond),
				exporter.WithErrorLogger(logger(errorLogBuf)),
			),
		)

		client = srv.Client()
	)

	t.Cleanup(srv.Close)

	request, err := http.NewRequest(http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("slow tool", func(t *testing.T) {
		t.Setenv("CONNTRACK_STATS_EXPORTER_SLEEP", "1")

		timings := make([]time.Duration, concurrency)

		wg := new(sync.WaitGroup)
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			i := i
			req := request.WithContext(context.Background())

			go func() {
				defer wg.Done()

				start := time.Now()
				resp, err := client.Do(req)
				if err != nil {
					t.Error(err)
					return
				}

				if _, err := io.Copy(io.Discard, resp.Body); err != nil {
					t.Error(err)
					return
				}

				if err := resp.Body.Close(); err != nil {
					t.Error(err)
					return
				}

				timings[i] = time.Since(start)
			}()
		}
		wg.Wait()

		t.Logf("timing median: %v", median(timings))
	})

	t.Run("inpatient client", func(t *testing.T) {
		t.Setenv("CONNTRACK_STATS_EXPORTER_SLEEP", "1")

		timings := make([]time.Duration, concurrency)

		wg := new(sync.WaitGroup)
		wg.Add(concurrency)
		for i := 0; i < concurrency; i++ {
			i := i

			go func() {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
				defer cancel()

				req := request.WithContext(ctx)
				start := time.Now()

				//nolint:bodyclose
				if _, err := client.Do(req); !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("expected context.DeadlineExceeded, but got %v", err)
				}

				timings[i] = time.Since(start)
			}()
		}
		wg.Wait()

		t.Logf("timing median: %v", median(timings))
	})

	for _, code := range []string{"0", "1"} {
		code := code

		t.Run("broken tool code "+code, func(t *testing.T) {
			t.Setenv("CONNTRACK_STATS_EXPORTER_KAPUTT", "true")
			t.Setenv("CONNTRACK_STATS_EXPORTER_EXIT_CODE", code)

			timings := make([]time.Duration, concurrency)

			wg := new(sync.WaitGroup)
			wg.Add(concurrency)

			for i := 0; i < concurrency; i++ {
				i := i
				req := request.WithContext(context.Background())

				go func() {
					defer wg.Done()

					start := time.Now()
					resp, err := client.Do(req)
					if err != nil {
						t.Error(err)
						return
					}

					if _, err := io.Copy(io.Discard, resp.Body); err != nil {
						t.Error(err)
						return
					}

					if err := resp.Body.Close(); err != nil {
						t.Error(err)
						return
					}

					timings[i] = time.Since(start)
				}()
			}
			wg.Wait()

			t.Logf("timing median: %v", median(timings))
		})
	}

	t.Run("check counters", func(t *testing.T) {
		time.Sleep(1 * time.Second)

		start := time.Now()

		resp, err := client.Do(request.WithContext(context.Background()))
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status %s, got status %s", http.StatusText(http.StatusOK), resp.Status)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		if err := resp.Body.Close(); err != nil {
			t.Fatal(err)
		}

		t.Logf("timing: %v", time.Since(start))

		regex := regexp.MustCompile(
			`(?m)^conntrack_stats_scrape_error\{.*?cause="(?P<cause>[a-z_]+)".*?} (?P<count>\d+)$`,
		)
		matches := regex.FindAllSubmatch(body, -1)

		counts := make(map[string]int, len(matches))

		for _, match := range matches {
			cause := string(match[1])

			count, err := strconv.Atoi(string(match[2]))
			if err != nil {
				t.Error(err)

				continue
			}

			counts[cause] = count
		}

		wantt := []struct {
			cause string
			count int
		}{
			{string(internal.OpTimeout), concurrency},
			{string(internal.OpClientGone), concurrency},
			{string(internal.OpExecTool), concurrency},
			{string(internal.OpToolOutputNoMatch), concurrency},
		}

		for _, want := range wantt {
			got, ok := counts[want.cause]
			if !ok {
				t.Errorf("expected to find cause %q, but didn't", want.cause)
			}

			if got != want.count {
				t.Errorf("expected count %q=%d, got %d", want.cause, want.count, got)
			}
		}

		totalWants := 0
		for _, want := range wantt {
			totalWants += want.count
		}

		totalCounts := 0
		for _, count := range counts {
			totalCounts += count
		}

		if totalCounts != totalWants {
			t.Errorf("expected total count %d, got %d", totalWants, totalCounts)
		}

		if t.Failed() {
			t.Logf("error log:\n%s", errorLogBuf.String())
			t.Logf("response:\n%s", string(body))
		}
	})
}

func TestUndefinedNetns(t *testing.T) {
	t.Parallel()

	var (
		errorLogBuf = new(bytes.Buffer)

		request = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		handler = exporter.Handler(
			exporter.WithNetNs([]string{"this-ns-does-not-exist"}),
			exporter.WithErrorLogger(logger(errorLogBuf)),
		)
	)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	resp := recorder.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %s, got status %s", http.StatusText(http.StatusOK), resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}

	regex := regexp.MustCompile(`(?m)^conntrack_stats_scrape_error\{.*?cause="netns_prepare".*?} 1$`)
	if !regex.Match(body) {
		t.Errorf("expected to find scrape error with cause \"netns_prepare\", but didn't")
	}

	if t.Failed() {
		t.Logf("error log:\n%s", errorLogBuf.String())
		t.Logf("response:\n%s", string(body))
	}
}

func mockConntrackTool(t testing.TB) {
	t.Helper()

	if len(_conntrackMockScript) == 0 {
		t.Fatal("conntrackMockScript is empty")
	}

	const (
		perm = 0755
		sep  = string(os.PathListSeparator)
	)

	var (
		dir  = t.TempDir()
		path = filepath.Join(dir, "conntrack")
	)

	if err := os.WriteFile(path, _conntrackMockScript, perm); err != nil {
		t.Fatal(err)
	}

	if envPath, ok := os.LookupEnv("PATH"); ok {
		t.Setenv("PATH", dir+sep+envPath)
	} else {
		t.Setenv("PATH", dir)
	}
}

func logger(w io.Writer) func(string, ...any) {
	mu := new(sync.Mutex)

	return func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()

		_, _ = fmt.Fprintf(w, format, args...)
	}
}

func median(timings []time.Duration) time.Duration {
	sort.Slice(timings, func(i, j int) bool { return timings[i] < timings[j] })
	return timings[len(timings)/2]
}

//go:embed conntrack_mock.sh
var _conntrackMockScript []byte
