package verify

import (
	"path/filepath"
	"testing"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
)

func TestRecordOuterLoopMemoryWritesFeedbackSummary(t *testing.T) {
	root := t.TempDir()
	event, err := RecordOuterLoopMemory(root, OuterLoopMemoryInput{
		Task: adapter.Task{
			TaskID:      "T-1",
			RoleHint:    "worker",
			WorkerMode:  "execution",
			OwnedPaths:  []string{"test/**"},
			Summary:     "Generate markdown output",
			Description: "demo",
		},
		Ticket: dispatch.Ticket{
			DispatchID: "dispatch_T-1_1_1",
			TaskID:     "T-1",
			PlanEpoch:  1,
			Attempt:    1,
		},
		SessionID:              "sess-1",
		BurstStatus:            "succeeded",
		VerifyStatus:           "blocked",
		VerifySummary:          "closeout hook blocked completion because required artifacts were missing: verify.json",
		FollowUp:               "analysis.required",
		VerificationResultPath: filepath.Join(root, ".harness", "artifacts", "T-1", "dispatch_T-1_1_1", "verify.json"),
		MissingArtifacts:       []string{"verify.json"},
		EvidenceRefs:           []string{"evidence-a"},
	})
	if err != nil {
		t.Fatalf("record outer-loop memory: %v", err)
	}
	if event.ID == "" || event.FeedbackType != "verification_failure" {
		t.Fatalf("unexpected feedback event: %+v", event)
	}

	summary, err := LoadFeedbackSummary(root)
	if err != nil {
		t.Fatalf("load feedback summary: %v", err)
	}
	if summary.FeedbackEventCount != 1 || summary.ErrorCount != 1 {
		t.Fatalf("unexpected feedback summary counts: %+v", summary)
	}
	taskSummary, ok := CurrentTaskFeedback(summary, "T-1")
	if !ok {
		t.Fatalf("expected task feedback summary: %+v", summary)
	}
	if taskSummary.LatestFeedbackType != "verification_failure" || len(taskSummary.RecentFailures) != 1 {
		t.Fatalf("unexpected task feedback summary: %+v", taskSummary)
	}
	if taskSummary.LatestThinkingSummary == "" || taskSummary.LatestNextAction == "" {
		t.Fatalf("expected thinking summary and next action: %+v", taskSummary)
	}
}
