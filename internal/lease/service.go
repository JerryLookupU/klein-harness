package lease

import (
	"errors"
	"fmt"
	"time"

	"klein-harness/internal/a2a"
	"klein-harness/internal/adapter"
	"klein-harness/internal/state"
)

var ErrLeaseNotFound = errors.New("lease not found")
var ErrLeaseNotActive = errors.New("lease is not active")
var ErrLeaseStale = errors.New("lease is stale for current execution")

type Record struct {
	LeaseID     string   `json:"leaseId"`
	TaskID      string   `json:"taskId"`
	DispatchID  string   `json:"dispatchId"`
	WorkerID    string   `json:"workerId"`
	Status      string   `json:"status"`
	ReasonCodes []string `json:"reasonCodes,omitempty"`
	CausationID string   `json:"causationId"`
	AcquiredAt  string   `json:"acquiredAt"`
	ExpiresAt   string   `json:"expiresAt"`
	RenewedAt   string   `json:"renewedAt,omitempty"`
	ReleasedAt  string   `json:"releasedAt,omitempty"`
}

type Summary struct {
	state.Metadata
	Leases     map[string]Record `json:"leases"`
	ByTask     map[string]string `json:"byTask"`
	ByDispatch map[string]string `json:"byDispatch"`
}

type AcquireRequest struct {
	Root        string
	TaskID      string
	DispatchID  string
	WorkerID    string
	LeaseID     string
	TTLSeconds  int
	CausationID string
	ReasonCodes []string
}

func Acquire(request AcquireRequest) (Record, error) {
	paths, err := adapter.Resolve(request.Root)
	if err != nil {
		return Record{}, err
	}
	summary, err := loadSummary(paths.LeaseSummaryPath)
	if err != nil {
		return Record{}, err
	}
	if request.LeaseID == "" {
		request.LeaseID = fmt.Sprintf("lease_%s_%s", request.TaskID, state.NowUTC())
	}
	if request.TTLSeconds <= 0 {
		request.TTLSeconds = 1800
	}
	if existingID := summary.ByTask[request.TaskID]; existingID != "" {
		existing := summary.Leases[existingID]
		if existing.Status == "active" && existing.WorkerID == request.WorkerID {
			return existing, nil
		}
		if existing.Status == "active" {
			return Record{}, fmt.Errorf("active lease exists for task %s", request.TaskID)
		}
	}
	now := state.NowUTC()
	record := Record{
		LeaseID:     request.LeaseID,
		TaskID:      request.TaskID,
		DispatchID:  request.DispatchID,
		WorkerID:    request.WorkerID,
		Status:      "active",
		ReasonCodes: request.ReasonCodes,
		CausationID: request.CausationID,
		AcquiredAt:  now,
		ExpiresAt:   expiresAt(request.TTLSeconds),
	}
	summary.Leases[record.LeaseID] = record
	summary.ByTask[record.TaskID] = record.LeaseID
	summary.ByDispatch[record.DispatchID] = record.LeaseID
	if _, err := state.WriteSnapshot(paths.LeaseSummaryPath, &summary, "kh-worker-supervisor", summary.Revision); err != nil {
		return Record{}, err
	}
	return record, nil
}

func Renew(root, leaseID, causationID string, ttlSeconds int, reasonCodes []string) (Record, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return Record{}, err
	}
	summary, err := loadSummary(paths.LeaseSummaryPath)
	if err != nil {
		return Record{}, err
	}
	record := summary.Leases[leaseID]
	record.RenewedAt = state.NowUTC()
	record.ExpiresAt = expiresAt(ttlSeconds)
	record.ReasonCodes = reasonCodes
	record.CausationID = causationID
	summary.Leases[leaseID] = record
	if _, err := state.WriteSnapshot(paths.LeaseSummaryPath, &summary, "kh-worker-supervisor", summary.Revision); err != nil {
		return Record{}, err
	}
	payload, err := a2a.NewPayload(map[string]any{
		"dispatchId": record.DispatchID,
		"workerId":   record.WorkerID,
		"leaseId":    record.LeaseID,
		"phase":      "running",
		"expiresAt":  record.ExpiresAt,
	})
	if err != nil {
		return Record{}, err
	}
	if _, err := a2a.AppendEvent(paths.EventLogPath, a2a.Envelope{
		Kind:           "worker.heartbeat",
		IdempotencyKey: fmt.Sprintf("heartbeat:%s:%s", record.LeaseID, record.RenewedAt),
		CausationID:    causationID,
		From:           "worker-supervisor-node",
		To:             "orchestrator-node",
		TaskID:         record.TaskID,
		WorkerID:       record.WorkerID,
		LeaseID:        record.LeaseID,
		ReasonCodes:    reasonCodes,
		Payload:        payload,
	}); err != nil {
		return Record{}, err
	}
	return record, nil
}

func Release(root, leaseID, causationID string, reasonCodes []string) (Record, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return Record{}, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		summary, err := loadSummary(paths.LeaseSummaryPath)
		if err != nil {
			return Record{}, err
		}
		record, ok := summary.Leases[leaseID]
		if !ok {
			return Record{}, ErrLeaseNotFound
		}
		if record.Status != "active" {
			return record, nil
		}
		record.Status = "released"
		record.ReleasedAt = state.NowUTC()
		record.CausationID = causationID
		record.ReasonCodes = reasonCodes
		summary.Leases[leaseID] = record
		delete(summary.ByTask, record.TaskID)
		delete(summary.ByDispatch, record.DispatchID)
		if _, err := state.WriteSnapshot(paths.LeaseSummaryPath, &summary, "kh-worker-supervisor", summary.Revision); err != nil {
			if errors.Is(err, state.ErrCASConflict) {
				continue
			}
			return Record{}, err
		}
		return record, nil
	}
	summary, err := loadSummary(paths.LeaseSummaryPath)
	if err != nil {
		return Record{}, err
	}
	record, ok := summary.Leases[leaseID]
	if !ok {
		return Record{}, ErrLeaseNotFound
	}
	if record.Status != "active" {
		return record, nil
	}
	return Record{}, fmt.Errorf("%w: could not release %s after concurrent updates", state.ErrCASConflict, leaseID)
}

func RecoverStale(root, causationID string) ([]Record, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return nil, err
	}
	summary, err := loadSummary(paths.LeaseSummaryPath)
	if err != nil {
		return nil, err
	}
	now := state.NowUTC()
	recovered := make([]Record, 0)
	for leaseID, record := range summary.Leases {
		if record.Status != "active" {
			continue
		}
		if record.ExpiresAt >= now {
			continue
		}
		record.Status = "stale_recovered"
		record.ReleasedAt = now
		record.CausationID = causationID
		record.ReasonCodes = []string{"stale_lease"}
		summary.Leases[leaseID] = record
		delete(summary.ByTask, record.TaskID)
		delete(summary.ByDispatch, record.DispatchID)
		recovered = append(recovered, record)
		payload, err := a2a.NewPayload(map[string]any{
			"status":  "stale_lease_recovered",
			"summary": "lease exceeded ttl and was recovered",
		})
		if err != nil {
			return nil, err
		}
		if _, err := a2a.AppendEvent(paths.EventLogPath, a2a.Envelope{
			Kind:           "worker.outcome",
			IdempotencyKey: fmt.Sprintf("stale:%s:%s", record.LeaseID, now),
			CausationID:    causationID,
			From:           "worker-supervisor-node",
			To:             "orchestrator-node",
			TaskID:         record.TaskID,
			WorkerID:       record.WorkerID,
			LeaseID:        record.LeaseID,
			ReasonCodes:    []string{"stale_lease"},
			Payload:        payload,
		}); err != nil {
			return nil, err
		}
	}
	if len(recovered) == 0 {
		return nil, nil
	}
	if _, err := state.WriteSnapshot(paths.LeaseSummaryPath, &summary, "kh-worker-supervisor", summary.Revision); err != nil {
		return nil, err
	}
	return recovered, nil
}

func ValidateCurrent(root, leaseID, taskID, dispatchID string) (Record, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return Record{}, err
	}
	summary, err := loadSummary(paths.LeaseSummaryPath)
	if err != nil {
		return Record{}, err
	}
	record, ok := summary.Leases[leaseID]
	if !ok {
		return Record{}, ErrLeaseNotFound
	}
	if taskID != "" && record.TaskID != taskID {
		return Record{}, fmt.Errorf("%w: task mismatch %s != %s", ErrLeaseStale, record.TaskID, taskID)
	}
	if dispatchID != "" && record.DispatchID != dispatchID {
		return Record{}, fmt.Errorf("%w: dispatch mismatch %s != %s", ErrLeaseStale, record.DispatchID, dispatchID)
	}
	if record.Status != "active" {
		return Record{}, fmt.Errorf("%w: %s", ErrLeaseNotActive, record.Status)
	}
	now := time.Now().UTC()
	expiresAt, err := time.Parse(time.RFC3339, record.ExpiresAt)
	if err == nil && expiresAt.Before(now) {
		return Record{}, fmt.Errorf("%w: expired at %s", ErrLeaseStale, record.ExpiresAt)
	}
	if current := summary.ByTask[record.TaskID]; current != "" && current != leaseID {
		return Record{}, fmt.Errorf("%w: task %s is now owned by %s", ErrLeaseStale, record.TaskID, current)
	}
	if current := summary.ByDispatch[record.DispatchID]; current != "" && current != leaseID {
		return Record{}, fmt.Errorf("%w: dispatch %s is now owned by %s", ErrLeaseStale, record.DispatchID, current)
	}
	return record, nil
}

func loadSummary(path string) (Summary, error) {
	summary := Summary{
		Leases:     map[string]Record{},
		ByTask:     map[string]string{},
		ByDispatch: map[string]string{},
	}
	if _, err := state.LoadJSONIfExists(path, &summary); err != nil {
		return Summary{}, err
	}
	if summary.Leases == nil {
		summary.Leases = map[string]Record{}
	}
	if summary.ByTask == nil {
		summary.ByTask = map[string]string{}
	}
	if summary.ByDispatch == nil {
		summary.ByDispatch = map[string]string{}
	}
	if len(summary.ByDispatch) == 0 {
		for leaseID, record := range summary.Leases {
			if record.DispatchID == "" {
				continue
			}
			if record.Status != "active" {
				continue
			}
			summary.ByDispatch[record.DispatchID] = leaseID
		}
	}
	return summary, nil
}

func expiresAt(ttlSeconds int) string {
	if ttlSeconds <= 0 {
		ttlSeconds = 1800
	}
	return time.Now().UTC().Add(time.Duration(ttlSeconds) * time.Second).Format(time.RFC3339)
}
