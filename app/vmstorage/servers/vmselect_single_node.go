package servers

import (
	"flag"
	"fmt"
	"math"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricnamestats"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmselectapi"
)

var (
	accountID = flag.Uint64("clusternative.accountID", 0, "The accountID of the stored data")
	projectID = flag.Uint64("clusternative.projectID", 0, "The projectID of the stored data")
)

const (
	maxAccountID = uint64(math.MaxUint32)
	maxProjectID = uint64(math.MaxUint32)
)

func newVMSingleAPI(s *storage.Storage) *vmsingleAPI {
	if *accountID > maxAccountID {
		logger.Fatalf("-clusternative.accountID must to be in the range [0, %d], got %d", maxAccountID, *accountID)
	}
	if *projectID > maxProjectID {
		logger.Fatalf("-clusternative.projectID must to be in the range [0, %d], got %d", maxProjectID, *projectID)
	}
	api := &vmsingleAPI{
		s:         &vmstorageAPI{s: s},
		accountID: uint32(*accountID),
		projectID: uint32(*projectID),
	}
	return api
}

// vmsingleAPI impelements vmselectapi.API for single node.
type vmsingleAPI struct {
	s         *vmstorageAPI
	accountID uint32
	projectID uint32
}

// marshalMetricBlock serializes a metric block in the format expected by
// vmselect.
//
// vmselect expects metric names and data blocks to have the tenantID but
// vmsingle does not have it. Therefore the tenantID needs to be included to
// every metric name and block.
func marshalMetricBlock(dst []byte, src *storage.MetricBlock) []byte {
	// Marshal metric name.
	dst = encoding.MarshalVarUint64(dst, uint64(len(src.MetricName))+8)
	dst = encoding.MarshalUint32(dst, uint32(*accountID))
	dst = encoding.MarshalUint32(dst, uint32(*projectID))
	dst = append(dst, src.MetricName...)

	// Marshal data block.
	dst = encoding.MarshalUint32(dst, uint32(*accountID))
	dst = encoding.MarshalUint32(dst, uint32(*projectID))
	dst = storage.MarshalBlock(dst, &src.Block)

	return dst
}

// emptyBlockIterator is an implementation of vmselectapi.BlockIterator that
// always returns no data.
type emptyBlockIterator struct{}

func (*emptyBlockIterator) MustClose() {}

func (*emptyBlockIterator) NextBlock(dst []byte) ([]byte, bool) {
	return dst, false
}

func (*emptyBlockIterator) Error() error {
	return nil
}

var emptyBI = &emptyBlockIterator{}

func (api *vmsingleAPI) InitSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (vmselectapi.BlockIterator, error) {
	if !sq.IsMultiTenant && (sq.AccountID != api.accountID || sq.ProjectID != api.projectID) {
		return emptyBI, nil
	}
	return api.s.InitSearch(qt, sq, deadline)
}

func (api *vmsingleAPI) SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) ([]string, error) {
	if !sq.IsMultiTenant && (sq.AccountID != api.accountID || sq.ProjectID != api.projectID) {
		return nil, nil
	}

	metricNames, err := api.s.SearchMetricNames(qt, sq, deadline)
	if err != nil {
		return nil, err
	}

	// vmselect expects metric names to have the tenantID but vmsingle does not
	// have it. Therefore the tenantID needs to be appended to every metric
	// name.
	dst := make([]byte, 0, 8)
	dst = encoding.MarshalUint32(dst, sq.AccountID)
	dst = encoding.MarshalUint32(dst, sq.ProjectID)
	tenantID := string(dst)

	for i, metricName := range metricNames {
		metricNames[i] = tenantID + metricName
	}
	return metricNames, nil
}

func (api *vmsingleAPI) LabelValues(qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int, deadline uint64) ([]string, error) {
	if !sq.IsMultiTenant && (sq.AccountID != api.accountID || sq.ProjectID != api.projectID) {
		return nil, nil
	}
	return api.s.LabelValues(qt, sq, labelName, maxLabelValues, deadline)
}

func (api *vmsingleAPI) TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte,
	maxSuffixes int, deadline uint64) ([]string, error) {
	if accountID != api.accountID || projectID != api.projectID {
		return nil, nil
	}
	return api.s.TagValueSuffixes(qt, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, deadline)
}

func (api *vmsingleAPI) LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline uint64) ([]string, error) {
	if !sq.IsMultiTenant && (sq.AccountID != api.accountID || sq.ProjectID != api.projectID) {
		return nil, nil
	}
	return api.s.LabelNames(qt, sq, maxLabelNames, deadline)
}

func (api *vmsingleAPI) SeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, deadline uint64) (uint64, error) {
	if accountID != api.accountID || projectID != api.projectID {
		return 0, nil
	}
	return api.s.SeriesCount(qt, accountID, projectID, deadline)
}

func (api *vmsingleAPI) Tenants(_ *querytracer.Tracer, _ storage.TimeRange, _ uint64) ([]string, error) {
	tenantID := fmt.Sprintf("%d:%d", api.accountID, api.projectID)
	return []string{tenantID}, nil
}

func (api *vmsingleAPI) TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline uint64) (*storage.TSDBStatus, error) {
	if !sq.IsMultiTenant && (sq.AccountID != api.accountID || sq.ProjectID != api.projectID) {
		return &storage.TSDBStatus{}, nil
	}
	return api.s.TSDBStatus(qt, sq, focusLabel, topN, deadline)
}

func (api *vmsingleAPI) DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (int, error) {
	if !sq.IsMultiTenant && (sq.AccountID != api.accountID || sq.ProjectID != api.projectID) {
		return 0, nil
	}
	return api.s.DeleteSeries(qt, sq, deadline)
}

func (api *vmsingleAPI) RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, _ uint64) error {
	return fmt.Errorf("not implemented")
}

func (api *vmsingleAPI) GetMetricNamesUsageStats(qt *querytracer.Tracer, tt *storage.TenantToken, limit, le int, matchPattern string, deadline uint64) (metricnamestats.StatsResult, error) {
	if tt != nil && (tt.AccountID != api.accountID || tt.ProjectID != api.projectID) {
		return metricnamestats.StatsResult{}, nil
	}
	return api.s.GetMetricNamesUsageStats(qt, tt, limit, le, matchPattern, deadline)
}

func (api *vmsingleAPI) ResetMetricNamesUsageStats(qt *querytracer.Tracer, deadline uint64) error {
	return api.s.ResetMetricNamesUsageStats(qt, deadline)
}

func (api *vmsingleAPI) GetMetadataRecords(qt *querytracer.Tracer, tt *storage.TenantToken, limit int, metricName string, deadline uint64) ([]*metricsmetadata.Row, error) {
	if tt != nil && (tt.AccountID != api.accountID || tt.ProjectID != api.projectID) {
		return nil, nil
	}
	return api.s.GetMetadataRecords(qt, tt, limit, metricName, deadline)
}
