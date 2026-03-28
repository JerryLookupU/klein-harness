package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeriveVerificationRejectsEmptyObject(t *testing.T) {
	artifactDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(artifactDir, "verify.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write verify.json: %v", err)
	}

	status, summary, verifyPath := DeriveVerification(artifactDir, "completed", "worker finished")
	if status != "failed" {
		t.Fatalf("expected empty verify object to fail, got status=%s summary=%s", status, summary)
	}
	if verifyPath == "" {
		t.Fatalf("expected verify path to be returned for empty verify artifact")
	}
}

func TestDeriveVerificationAcceptsScorecardBackedArtifact(t *testing.T) {
	artifactDir := t.TempDir()
	payload := `{
  "overallStatus": "passed",
  "overallSummary": "verification completed",
  "scorecard": [{"id":"scopeCompletion","status":"pass","score":3,"threshold":3}],
  "evidenceLedger": [{"kind":"command","summary":"go test ./... passed"}]
}`
	if err := os.WriteFile(filepath.Join(artifactDir, "verify.json"), []byte(payload), 0o644); err != nil {
		t.Fatalf("write verify.json: %v", err)
	}

	status, summary, verifyPath := DeriveVerification(artifactDir, "completed", "worker finished")
	if status != "passed" || summary != "verification completed" || verifyPath == "" {
		t.Fatalf("expected scorecard-backed verify to pass, got status=%s summary=%s path=%s", status, summary, verifyPath)
	}
}
