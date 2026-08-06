package appmetrics

import (
	stdos "os"
	"path/filepath"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// UncleanShutdownMarkerFilename is the marker file used by MustStartUncleanShutdownMarker.
const UncleanShutdownMarkerFilename = ".vm_app_running"

var (
	uncleanShutdownMarkerDirPath string

	uncleanShutdownMetricEnabled atomic.Bool
	startedAfterUncleanShutdown  atomic.Uint64
)

// MustStartUncleanShutdownMarker creates a durable running marker in dirPath.
//
// If the marker already exists, vm_app_started_after_unclean_shutdown is set to 1.
// Must be paired with a single MustStopUncleanShutdownMarker call.
func MustStartUncleanShutdownMarker(dirPath string) {
	if uncleanShutdownMarkerDirPath != "" {
		logger.Panicf("BUG: MustStartUncleanShutdownMarker has been already called")
	}

	fs.MustMkdirIfNotExist(dirPath)
	fs.MustSyncPathAndParentDir(dirPath)

	markerPath := filepath.Join(dirPath, UncleanShutdownMarkerFilename)
	if fs.IsPathExist(markerPath) {
		startedAfterUncleanShutdown.Store(1)
		logger.Warnf("detected an unclean shutdown because %q exists", markerPath)
	} else {
		fs.MustWriteSync(markerPath, nil)
		fs.MustSyncPath(dirPath)
	}

	uncleanShutdownMarkerDirPath = dirPath
	uncleanShutdownMetricEnabled.Store(true)
}

// MustStopUncleanShutdownMarker removes the running marker created by MustStartUncleanShutdownMarker.
func MustStopUncleanShutdownMarker() {
	if uncleanShutdownMarkerDirPath == "" {
		logger.Panicf("BUG: MustStartUncleanShutdownMarker must be called before MustStopUncleanShutdownMarker")
	}

	markerPath := filepath.Join(uncleanShutdownMarkerDirPath, UncleanShutdownMarkerFilename)
	dirPath := uncleanShutdownMarkerDirPath
	uncleanShutdownMarkerDirPath = ""
	if err := stdos.Remove(markerPath); err != nil {
		if !stdos.IsNotExist(err) {
			logger.Errorf("cannot remove %q: %s", markerPath, err)
		}
		return
	}
	fs.MustSyncPath(dirPath)
}
