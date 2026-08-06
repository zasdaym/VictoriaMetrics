package appmetrics

import (
	"bytes"
	stdos "os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestUncleanShutdownMarkerLifecycle(t *testing.T) {
	resetUncleanShutdownStateForTest(t)

	dirPath := filepath.Join(t.TempDir(), "data")
	markerPath := filepath.Join(dirPath, UncleanShutdownMarkerFilename)

	var bb bytes.Buffer
	writePrometheusMetrics(&bb)
	if strings.Contains(bb.String(), "vm_app_started_after_unclean_shutdown") {
		t.Fatalf("unexpected unclean shutdown metric before starting the marker")
	}

	MustStartUncleanShutdownMarker(dirPath)
	mustContainUncleanShutdownMetric(t, 0)
	if _, err := stdos.Stat(markerPath); err != nil {
		t.Fatalf("cannot stat the running marker after the first start: %s", err)
	}
	MustStopUncleanShutdownMarker()
	if _, err := stdos.Stat(markerPath); !stdos.IsNotExist(err) {
		t.Fatalf("unexpected running marker after a clean shutdown; got error %v; want os.ErrNotExist", err)
	}

	if err := stdos.WriteFile(markerPath, nil, 0600); err != nil {
		t.Fatalf("cannot create test marker: %s", err)
	}
	MustStartUncleanShutdownMarker(dirPath)
	mustContainUncleanShutdownMetric(t, 1)
	MustStopUncleanShutdownMarker()
	if _, err := stdos.Stat(markerPath); !stdos.IsNotExist(err) {
		t.Fatalf("unexpected running marker after a clean shutdown; got error %v; want os.ErrNotExist", err)
	}

	MustStartUncleanShutdownMarker(dirPath)
	mustContainUncleanShutdownMetric(t, 1)
	if err := stdos.Remove(markerPath); err != nil {
		t.Fatalf("cannot remove running marker: %s", err)
	}
	MustStopUncleanShutdownMarker()
}

func TestMustStartUncleanShutdownMarkerCalledTwicePanics(t *testing.T) {
	resetUncleanShutdownStateForTest(t)

	dirPath := t.TempDir()
	MustStartUncleanShutdownMarker(dirPath)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expecting a panic")
		}
	}()
	MustStartUncleanShutdownMarker(dirPath)
}

func resetUncleanShutdownStateForTest(t *testing.T) {
	reset := func() {
		uncleanShutdownMarkerDirPath = ""
		uncleanShutdownMetricEnabled.Store(false)
		startedAfterUncleanShutdown.Store(0)
	}
	reset()
	t.Cleanup(reset)
}

func mustContainUncleanShutdownMetric(t *testing.T, value uint64) {
	t.Helper()

	var bb bytes.Buffer
	writePrometheusMetrics(&bb)
	want := "vm_app_started_after_unclean_shutdown " + strconv.FormatUint(value, 10) + "\n"
	if !strings.Contains(bb.String(), want) {
		t.Fatalf("missing %q in the exported app metrics", want)
	}
}
