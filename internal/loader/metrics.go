package loader

import (
	"slices"
	"sync"
	"time"
)

type MetricsSnapshot struct {
	SuccessfulBatches int           `json:"successfulBatches"`
	FailedBatches     int           `json:"failedBatches"`
	RetryCount        int           `json:"retryCount"`
	RowsAttempted     int64         `json:"rowsAttempted"`
	RowsSucceeded     int64         `json:"rowsSucceeded"`
	BatchLatencyAvg   time.Duration `json:"batchLatencyAvg"`
	BatchLatencyP50   time.Duration `json:"batchLatencyP50"`
	BatchLatencyP95   time.Duration `json:"batchLatencyP95"`
	BatchLatencyP99   time.Duration `json:"batchLatencyP99"`
	BatchLatencyMax   time.Duration `json:"batchLatencyMax"`
	ExecLatencyAvg    time.Duration `json:"execLatencyAvg"`
	ExecLatencyP50    time.Duration `json:"execLatencyP50"`
	ExecLatencyP95    time.Duration `json:"execLatencyP95"`
	ExecLatencyP99    time.Duration `json:"execLatencyP99"`
	ExecLatencyMax    time.Duration `json:"execLatencyMax"`
}

type MetricsCollector struct {
	mu             sync.Mutex
	batchLatencies []time.Duration
	execLatencies  []time.Duration
	successBatches int
	failedBatches  int
	retryCount     int
	rowsAttempted  int64
	rowsSucceeded  int64
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

func (m *MetricsCollector) RecordExec(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.execLatencies = append(m.execLatencies, duration)
}

func (m *MetricsCollector) RecordBatchSuccess(size int, duration time.Duration, retries int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.batchLatencies = append(m.batchLatencies, duration)
	m.successBatches++
	m.retryCount += retries
	m.rowsAttempted += int64(size)
	m.rowsSucceeded += int64(size)
}

func (m *MetricsCollector) RecordBatchFailure(size int, duration time.Duration, retries int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.batchLatencies = append(m.batchLatencies, duration)
	m.failedBatches++
	m.retryCount += retries
	m.rowsAttempted += int64(size)
}

func (m *MetricsCollector) Snapshot() MetricsSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	batchLatencies := slices.Clone(m.batchLatencies)
	execLatencies := slices.Clone(m.execLatencies)

	return MetricsSnapshot{
		SuccessfulBatches: m.successBatches,
		FailedBatches:     m.failedBatches,
		RetryCount:        m.retryCount,
		RowsAttempted:     m.rowsAttempted,
		RowsSucceeded:     m.rowsSucceeded,
		BatchLatencyAvg:   averageDuration(batchLatencies),
		BatchLatencyP50:   percentileDuration(batchLatencies, 0.50),
		BatchLatencyP95:   percentileDuration(batchLatencies, 0.95),
		BatchLatencyP99:   percentileDuration(batchLatencies, 0.99),
		BatchLatencyMax:   maxDuration(batchLatencies),
		ExecLatencyAvg:    averageDuration(execLatencies),
		ExecLatencyP50:    percentileDuration(execLatencies, 0.50),
		ExecLatencyP95:    percentileDuration(execLatencies, 0.95),
		ExecLatencyP99:    percentileDuration(execLatencies, 0.99),
		ExecLatencyMax:    maxDuration(execLatencies),
	}
}

func averageDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}

	var total time.Duration
	for _, value := range values {
		total += value
	}

	return total / time.Duration(len(values))
}

func maxDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}

	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}

	return max
}

func percentileDuration(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}

	slices.Sort(values)
	index := int(float64(len(values)-1) * percentile)
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}

	return values[index]
}
