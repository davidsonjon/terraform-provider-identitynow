package source_actions_v1

import (
	"os"
	"testing"
	"time"

	"github.com/sailpoint-oss/golang-sdk/v3/task_management"
)

func strPtr(v string) *string { return &v }

func TestParseTimeout(t *testing.T) {
	t.Run("empty defaults to 30m", func(t *testing.T) {
		got, err := parseTimeout("")
		if err != nil {
			t.Fatalf("parseTimeout(\"\") returned error: %v", err)
		}
		if got != defaultInvokeTimeout {
			t.Fatalf("parseTimeout(\"\") = %v, want %v", got, defaultInvokeTimeout)
		}
	})

	t.Run("valid duration", func(t *testing.T) {
		got, err := parseTimeout("45m")
		if err != nil {
			t.Fatalf("parseTimeout(\"45m\") returned error: %v", err)
		}
		if got != 45*time.Minute {
			t.Fatalf("parseTimeout(\"45m\") = %v, want %v", got, 45*time.Minute)
		}
	})

	t.Run("invalid duration", func(t *testing.T) {
		if _, err := parseTimeout("not-a-duration"); err == nil {
			t.Fatal("parseTimeout(\"not-a-duration\") returned nil error, want non-nil")
		}
	})

	t.Run("non-positive duration", func(t *testing.T) {
		if _, err := parseTimeout("0s"); err == nil {
			t.Fatal("parseTimeout(\"0s\") returned nil error, want non-nil")
		}
		if _, err := parseTimeout("-5m"); err == nil {
			t.Fatal("parseTimeout(\"-5m\") returned nil error, want non-nil")
		}
	})
}

func TestNormalizedCompletionStatus(t *testing.T) {
	tests := []struct {
		name string
		in   task_management.NullableString
		want string
	}{
		{name: "unset", in: task_management.NullableString{}, want: ""},
		{name: "success", in: *task_management.NewNullableString(strPtr("success")), want: "SUCCESS"},
		{name: "warning with spaces", in: *task_management.NewNullableString(strPtr(" warning ")), want: "WARNING"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizedCompletionStatus(tt.in); got != tt.want {
				t.Fatalf("normalizedCompletionStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsSuccessfulCompletionStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: "SUCCESS", want: true},
		{status: "WARNING", want: true},
		{status: "ERROR", want: false},
		{status: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := isSuccessfulCompletionStatus(tt.status); got != tt.want {
				t.Fatalf("isSuccessfulCompletionStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestTaskCompletionResult(t *testing.T) {
	now := task_management.SailPointTime{Time: time.Now()}

	tests := []struct {
		name           string
		status         *task_management.TaskStatus
		wantFinished   bool
		wantCompletion string
	}{
		{name: "nil status", status: nil, wantFinished: false},
		{name: "not yet completed", status: &task_management.TaskStatus{}, wantFinished: false},
		{
			name: "completed and completionStatus both set",
			status: &task_management.TaskStatus{
				Completed:        *task_management.NewNullableTime(&now),
				CompletionStatus: *task_management.NewNullableString(strPtr("success")),
			},
			wantFinished:   true,
			wantCompletion: "SUCCESS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finished, completionStatus := taskCompletionResult(tt.status)
			if finished != tt.wantFinished {
				t.Fatalf("finished = %v, want %v", finished, tt.wantFinished)
			}
			if completionStatus != tt.wantCompletion {
				t.Fatalf("completionStatus = %q, want %q", completionStatus, tt.wantCompletion)
			}
		})
	}
}

func TestPollInterval(t *testing.T) {
	tests := []struct {
		attempt int
		want    string
	}{
		{attempt: 0, want: "2s"},
		{attempt: 1, want: "4s"},
		{attempt: 2, want: "8s"},
		{attempt: 3, want: "15s"},
		{attempt: 5, want: "15s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := pollInterval(tt.attempt).String(); got != tt.want {
				t.Fatalf("pollInterval(%d) = %q, want %q", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestEmptyMultipartFile(t *testing.T) {
	f, err := emptyMultipartFile("test-prefix")
	if err != nil {
		t.Fatalf("emptyMultipartFile() returned error: %v", err)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat() returned error: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("file size = %d, want 0 (empty file)", info.Size())
	}
}
