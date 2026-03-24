package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"klein-harness/internal/checkpoint"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/lease"
	"klein-harness/internal/tmux"
	"klein-harness/internal/worker"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "claim":
		err = runClaim(os.Args[2:])
	case "burst":
		err = runBurst(os.Args[2:])
	case "renew-lease":
		err = runRenewLease(os.Args[2:])
	case "release-lease":
		err = runReleaseLease(os.Args[2:])
	case "recover-stale":
		err = runRecoverStale(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: kh-worker-supervisor <claim|burst|renew-lease|release-lease|recover-stale> [args...]")
}

func runClaim(args []string) error {
	fs := flag.NewFlagSet("claim", flag.ContinueOnError)
	root := fs.String("root", ".", "project root")
	dispatchID := fs.String("dispatch-id", "", "dispatch id")
	taskID := fs.String("task-id", "", "task id")
	workerID := fs.String("worker-id", "", "worker id")
	leaseID := fs.String("lease-id", "", "lease id")
	causationID := fs.String("causation-id", "", "causation id")
	ttl := fs.Int("lease-ttl-sec", 1800, "lease ttl")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *workerID == "" {
		return fmt.Errorf("missing --worker-id")
	}
	ticket, err := resolveTicket(*root, *dispatchID, *taskID)
	if err != nil {
		return err
	}
	leaseRecord, err := lease.Acquire(lease.AcquireRequest{
		Root:        *root,
		TaskID:      ticket.TaskID,
		DispatchID:  ticket.DispatchID,
		WorkerID:    *workerID,
		LeaseID:     *leaseID,
		TTLSeconds:  *ttl,
		CausationID: *causationID,
	})
	if err != nil {
		return err
	}
	ticket, err = dispatch.Claim(dispatch.ClaimRequest{
		Root:        *root,
		DispatchID:  ticket.DispatchID,
		WorkerID:    *workerID,
		LeaseID:     leaseRecord.LeaseID,
		CausationID: *causationID,
	})
	if err != nil {
		return err
	}
	return writeStdout(map[string]any{
		"dispatch": ticket,
		"lease":    leaseRecord,
	})
}

func runBurst(args []string) error {
	fs := flag.NewFlagSet("burst", flag.ContinueOnError)
	root := fs.String("root", ".", "project root")
	dispatchID := fs.String("dispatch-id", "", "dispatch id")
	taskID := fs.String("task-id", "", "task id")
	workerID := fs.String("worker-id", "", "worker id")
	causationID := fs.String("causation-id", "", "causation id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *workerID == "" {
		return fmt.Errorf("missing --worker-id")
	}
	ticket, err := resolveTicket(*root, *dispatchID, *taskID)
	if err != nil {
		return err
	}
	var leaseRecord lease.Record
	if ticket.LeaseID != "" {
		leaseRecord, err = lease.Renew(*root, ticket.LeaseID, *causationID, ticket.LeaseTTLSec, []string{"burst_start"})
		if err != nil {
			return err
		}
	} else {
		leaseRecord, err = lease.Acquire(lease.AcquireRequest{
			Root:        *root,
			TaskID:      ticket.TaskID,
			DispatchID:  ticket.DispatchID,
			WorkerID:    *workerID,
			TTLSeconds:  ticket.LeaseTTLSec,
			CausationID: *causationID,
			ReasonCodes: []string{"burst_start"},
		})
		if err != nil {
			return err
		}
		ticket, err = dispatch.Claim(dispatch.ClaimRequest{
			Root:        *root,
			DispatchID:  ticket.DispatchID,
			WorkerID:    *workerID,
			LeaseID:     leaseRecord.LeaseID,
			CausationID: *causationID,
			ReasonCodes: []string{"burst_start"},
		})
		if err != nil {
			return err
		}
	}
	checkpointPath := dispatch.DefaultCheckpointPath(*root, ticket.TaskID, ticket.Attempt)
	outcomePath := filepath.Join(filepath.Dir(checkpointPath), "outcome.json")
	bundle, err := worker.Prepare(*root, ticket, leaseRecord.LeaseID)
	if err != nil {
		return err
	}
	result, err := tmux.RunBoundedBurst(tmux.BurstRequest{
		TaskID:     ticket.TaskID,
		DispatchID: ticket.DispatchID,
		WorkerID:   *workerID,
		Cwd:        ticket.Cwd,
		Command: resolveWorkerCommand(ticket.Command, map[string]string{
			"SESSION_ID":        ticket.ResumeSessionID,
			"LAST_MESSAGE_PATH": filepath.Join(bundle.ArtifactDir, "last-message.txt"),
		}),
		PromptPath:     bundle.PromptPath,
		Budget:         ticket.Budget,
		CheckpointPath: checkpointPath,
		OutcomePath:    outcomePath,
		Artifacts: []string{
			bundle.ManifestPath,
			bundle.PromptPath,
			filepath.Join(bundle.ArtifactDir, "worker-result.json"),
			filepath.Join(bundle.ArtifactDir, "verify.json"),
			filepath.Join(bundle.ArtifactDir, "handoff.md"),
		},
	})
	if err != nil {
		return err
	}
	if _, err := checkpoint.IngestCheckpoint(checkpoint.IngestCheckpointRequest{
		Root:          *root,
		TaskID:        ticket.TaskID,
		DispatchID:    ticket.DispatchID,
		PlanEpoch:     ticket.PlanEpoch,
		Attempt:       ticket.Attempt,
		CausationID:   *causationID,
		ReasonCodes:   []string{"burst_checkpoint"},
		CheckpointRef: checkpointPath,
		Status:        "checkpointed",
		Summary:       "bounded burst checkpoint persisted",
	}); err != nil {
		return err
	}
	nextKind := ""
	switch result.Status {
	case "failed", "timed_out":
		nextKind = "replan"
	}
	if _, err := checkpoint.IngestOutcome(checkpoint.IngestOutcomeRequest{
		Root:          *root,
		TaskID:        ticket.TaskID,
		DispatchID:    ticket.DispatchID,
		PlanEpoch:     ticket.PlanEpoch,
		Attempt:       ticket.Attempt,
		CausationID:   *causationID,
		WorkerID:      *workerID,
		LeaseID:       leaseRecord.LeaseID,
		ReasonCodes:   []string{"bounded_burst_finished"},
		Status:        result.Status,
		Summary:       result.Summary,
		CheckpointRef: checkpointPath,
		DiffStats: checkpoint.DiffStats{
			FilesChanged: result.DiffStats["filesChanged"],
			Insertions:   result.DiffStats["insertions"],
			Deletions:    result.DiffStats["deletions"],
		},
		Artifacts:         result.Artifacts,
		NextSuggestedKind: nextKind,
	}); err != nil {
		return err
	}
	if _, err := dispatch.UpdateStatus(*root, ticket.DispatchID, result.Status, "kh-worker-supervisor"); err != nil {
		return err
	}
	if _, err := lease.Release(*root, leaseRecord.LeaseID, *causationID, []string{"burst_finished"}); err != nil {
		return err
	}
	return writeStdout(result)
}

func runRenewLease(args []string) error {
	fs := flag.NewFlagSet("renew-lease", flag.ContinueOnError)
	root := fs.String("root", ".", "project root")
	leaseID := fs.String("lease-id", "", "lease id")
	causationID := fs.String("causation-id", "", "causation id")
	ttl := fs.Int("lease-ttl-sec", 1800, "lease ttl")
	if err := fs.Parse(args); err != nil {
		return err
	}
	record, err := lease.Renew(*root, *leaseID, *causationID, *ttl, []string{"lease_renewed"})
	if err != nil {
		return err
	}
	return writeStdout(record)
}

func runReleaseLease(args []string) error {
	fs := flag.NewFlagSet("release-lease", flag.ContinueOnError)
	root := fs.String("root", ".", "project root")
	leaseID := fs.String("lease-id", "", "lease id")
	causationID := fs.String("causation-id", "", "causation id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	record, err := lease.Release(*root, *leaseID, *causationID, []string{"lease_released"})
	if err != nil {
		return err
	}
	return writeStdout(record)
}

func runRecoverStale(args []string) error {
	fs := flag.NewFlagSet("recover-stale", flag.ContinueOnError)
	root := fs.String("root", ".", "project root")
	causationID := fs.String("causation-id", "lease_recovery", "causation id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	records, err := lease.RecoverStale(*root, *causationID)
	if err != nil {
		return err
	}
	return writeStdout(records)
}

func resolveTicket(root, dispatchID, taskID string) (dispatch.Ticket, error) {
	if dispatchID != "" {
		return dispatch.Get(root, dispatchID)
	}
	return dispatch.FindClaimableForTask(root, taskID)
}

func writeStdout(value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(payload, '\n'))
	return err
}

func resolveWorkerCommand(command string, replacements map[string]string) string {
	resolved := command
	for key, value := range replacements {
		resolved = strings.ReplaceAll(resolved, "<"+key+">", value)
	}
	return resolved
}
