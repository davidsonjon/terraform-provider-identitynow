package source_load_entitlement_wait_v1

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/sailpoint-oss/golang-sdk/v3/task_management"
)

func TestParseImportStateID(t *testing.T) {
	t.Run("multiple triggers", func(t *testing.T) {
		parsed, err := parseImportStateID("source-123,foo:bar/baz:qux,true")
		if err != nil {
			t.Fatalf("parseImportStateID returned error: %v", err)
		}
		if parsed.SourceID != "source-123" {
			t.Fatalf("SourceID = %q, want %q", parsed.SourceID, "source-123")
		}
		if !parsed.WaitForActiveJobs {
			t.Fatalf("WaitForActiveJobs = false, want true")
		}
		if parsed.Triggers.IsNull() {
			t.Fatal("Triggers is null, want populated map")
		}

		elements := parsed.Triggers.Elements()
		if len(elements) != 2 {
			t.Fatalf("len(Triggers.Elements()) = %d, want 2", len(elements))
		}
		if got := elements["foo"].(types.String).ValueString(); got != "bar" {
			t.Errorf("triggers[foo] = %q, want %q", got, "bar")
		}
		if got := elements["baz"].(types.String).ValueString(); got != "qux" {
			t.Errorf("triggers[baz] = %q, want %q", got, "qux")
		}
	})

	t.Run("empty triggers", func(t *testing.T) {
		parsed, err := parseImportStateID("source-123,,false")
		if err != nil {
			t.Fatalf("parseImportStateID returned error: %v", err)
		}
		if parsed.WaitForActiveJobs {
			t.Fatalf("WaitForActiveJobs = true, want false")
		}
		if !parsed.Triggers.IsNull() {
			t.Fatalf("Triggers.IsNull() = false, want true")
		}
	})

	t.Run("trigger allows empty value", func(t *testing.T) {
		parsed, err := parseImportStateID("source-123,foo:,false")
		if err != nil {
			t.Fatalf("parseImportStateID returned error: %v", err)
		}
		elements := parsed.Triggers.Elements()
		if got := elements["foo"].(types.String).ValueString(); got != "" {
			t.Errorf("triggers[foo] = %q, want empty string", got)
		}
	})
}

func TestParseImportStateID_Invalid(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "missing parts", id: "source-123,true"},
		{name: "empty source", id: ",foo:bar,true"},
		{name: "invalid bool", id: "source-123,foo:bar,maybe"},
		{name: "missing trigger colon", id: "source-123,foobar,true"},
		{name: "empty trigger key", id: "source-123,:bar,true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseImportStateID(tt.id); err == nil {
				t.Fatalf("parseImportStateID(%q) returned nil error, want non-nil", tt.id)
			}
		})
	}
}

func TestEntitlementImportTaskStatusListFilter(t *testing.T) {
	got := entitlementImportTaskStatusListFilter(`source-"abc"`)
	want := `sourceId eq "source-\"abc\"" and completionStatus isnull`
	if got != want {
		t.Fatalf("entitlementImportTaskStatusListFilter() = %q, want %q", got, want)
	}
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
		{status: "TEMPERROR", want: false},
		{status: "TERMINATED", want: false},
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
		{
			name:         "nil status",
			status:       nil,
			wantFinished: false,
		},
		{
			name:         "not yet completed",
			status:       &task_management.TaskStatus{},
			wantFinished: false,
		},
		{
			name: "completed set but completionStatus still empty (observed race condition)",
			status: &task_management.TaskStatus{
				Completed:        *task_management.NewNullableTime(&now),
				CompletionStatus: task_management.NullableString{},
			},
			wantFinished: false,
		},
		{
			name: "completionStatus set but completed not yet set",
			status: &task_management.TaskStatus{
				CompletionStatus: *task_management.NewNullableString(strPtr("SUCCESS")),
			},
			wantFinished: false,
		},
		{
			name: "completed and completionStatus both set",
			status: &task_management.TaskStatus{
				Completed:        *task_management.NewNullableTime(&now),
				CompletionStatus: *task_management.NewNullableString(strPtr("success")),
			},
			wantFinished:   true,
			wantCompletion: "SUCCESS",
		},
		{
			name: "completed and completionStatus both set with failure status",
			status: &task_management.TaskStatus{
				Completed:        *task_management.NewNullableTime(&now),
				CompletionStatus: *task_management.NewNullableString(strPtr("ERROR")),
			},
			wantFinished:   true,
			wantCompletion: "ERROR",
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

func TestEmptyImportEntitlementsFile(t *testing.T) {
	f, err := emptyImportEntitlementsFile()
	if err != nil {
		t.Fatalf("emptyImportEntitlementsFile() returned error: %v", err)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()

	if f.Name() == "" {
		t.Fatal("emptyImportEntitlementsFile() returned a file with an empty name")
	}

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat() returned error: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("file size = %d, want 0 (empty file)", info.Size())
	}
}

func strPtr(v string) *string { return &v }
