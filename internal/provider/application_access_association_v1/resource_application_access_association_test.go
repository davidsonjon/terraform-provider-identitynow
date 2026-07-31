package application_access_association_v1

import (
	"reflect"
	"testing"
)

func TestMergeAccessProfileIDsForCreate(t *testing.T) {
	tests := []struct {
		name    string
		current []string
		desired []string
		want    []string
	}{
		{
			name:    "unions current and desired preserving first-seen order",
			current: []string{"ap-1", "ap-2"},
			desired: []string{"ap-2", "ap-3", "ap-1", "ap-4"},
			want:    []string{"ap-1", "ap-2", "ap-3", "ap-4"},
		},
		{
			name:    "skips empty ids",
			current: []string{"ap-1", ""},
			desired: []string{"", "ap-2"},
			want:    []string{"ap-1", "ap-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeAccessProfileIDsForCreate(tt.current, tt.desired); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("mergeAccessProfileIDsForCreate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeAccessProfileIDsForUpdate(t *testing.T) {
	tests := []struct {
		name       string
		current    []string
		oldTracked []string
		newTracked []string
		want       []string
	}{
		{
			name:       "removes only old tracked ids before adding new tracked ids",
			current:    []string{"shared-1", "old-1", "other-managed", "old-2"},
			oldTracked: []string{"old-1", "old-2"},
			newTracked: []string{"old-2", "new-1"},
			want:       []string{"shared-1", "other-managed", "old-2", "new-1"},
		},
		{
			name:       "preserves ids added by other resources between applies",
			current:    []string{"other-1", "mine-1", "other-2"},
			oldTracked: []string{"mine-1"},
			newTracked: []string{"mine-2"},
			want:       []string{"other-1", "other-2", "mine-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeAccessProfileIDsForUpdate(tt.current, tt.oldTracked, tt.newTracked); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("mergeAccessProfileIDsForUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveAccessProfileIDs(t *testing.T) {
	tests := []struct {
		name     string
		current  []string
		toRemove []string
		want     []string
	}{
		{
			name:     "removes only tracked ids and preserves everything else",
			current:  []string{"keep-1", "remove-1", "keep-2", "remove-2"},
			toRemove: []string{"remove-1", "remove-2"},
			want:     []string{"keep-1", "keep-2"},
		},
		{
			name:     "deduplicates preserved ids",
			current:  []string{"keep-1", "keep-1", "remove-1"},
			toRemove: []string{"remove-1"},
			want:     []string{"keep-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeAccessProfileIDs(tt.current, tt.toRemove); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("removeAccessProfileIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetainManagedAccessProfileIDs(t *testing.T) {
	tests := []struct {
		name    string
		current []string
		tracked []string
		want    []string
	}{
		{
			name:    "keeps only tracked ids still present live",
			current: []string{"ap-1", "ap-3"},
			tracked: []string{"ap-1", "ap-2", "ap-3"},
			want:    []string{"ap-1", "ap-3"},
		},
		{
			name:    "deduplicates tracked ids",
			current: []string{"ap-1"},
			tracked: []string{"ap-1", "ap-1"},
			want:    []string{"ap-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retainManagedAccessProfileIDs(tt.current, tt.tracked); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("retainManagedAccessProfileIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseImportStateID(t *testing.T) {
	t.Run("valid import id", func(t *testing.T) {
		parsed, err := parseImportStateID("app-123,ap-1/ap-2/ap-1")
		if err != nil {
			t.Fatalf("parseImportStateID returned error: %v", err)
		}
		if parsed.ApplicationID != "app-123" {
			t.Fatalf("ApplicationID = %q, want %q", parsed.ApplicationID, "app-123")
		}
		wantIDs := []string{"ap-1", "ap-2"}
		if !reflect.DeepEqual(parsed.AccessProfileIDs, wantIDs) {
			t.Fatalf("AccessProfileIDs = %v, want %v", parsed.AccessProfileIDs, wantIDs)
		}
	})

	t.Run("trims surrounding whitespace", func(t *testing.T) {
		parsed, err := parseImportStateID(" app-123 , ap-1 / ap-2 ")
		if err != nil {
			t.Fatalf("parseImportStateID returned error: %v", err)
		}
		wantIDs := []string{"ap-1", "ap-2"}
		if parsed.ApplicationID != "app-123" {
			t.Fatalf("ApplicationID = %q, want %q", parsed.ApplicationID, "app-123")
		}
		if !reflect.DeepEqual(parsed.AccessProfileIDs, wantIDs) {
			t.Fatalf("AccessProfileIDs = %v, want %v", parsed.AccessProfileIDs, wantIDs)
		}
	})
}

func TestParseImportStateID_Invalid(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "missing comma", id: "app-123"},
		{name: "empty application id", id: ",ap-1/ap-2"},
		{name: "empty access profile component", id: "app-123,"},
		{name: "empty access profile id", id: "app-123,ap-1//ap-2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseImportStateID(tt.id); err == nil {
				t.Fatalf("parseImportStateID(%q) returned nil error, want non-nil", tt.id)
			}
		})
	}
}
