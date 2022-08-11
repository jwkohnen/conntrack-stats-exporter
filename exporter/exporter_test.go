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

func mockConntrackTool(t *testing.T) {
	t.Helper()

	if len(conntrackMockScript) == 0 {
		t.Fatal("conntrackMockScript is empty")
	}

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "conntrack"), conntrackMockScript, 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

//go:embed conntrack_mock.sh
var conntrackMockScript []byte
