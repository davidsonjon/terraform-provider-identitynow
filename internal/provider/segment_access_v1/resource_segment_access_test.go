package segment_access_v1

import (
	"reflect"
	"testing"
)

func TestDiffSegmentAccessAssignments(t *testing.T) {
	tests := []struct {
		name         string
		current      []segmentAccessAssignment
		desired      []segmentAccessAssignment
		wantAssign   []segmentAccessAssignment
		wantRemovals []segmentAccessAssignment
	}{
		{
			name:         "no change",
			current:      nil,
			desired:      nil,
			wantAssign:   nil,
			wantRemovals: nil,
		},
		{
			name:       "pure addition",
			current:    nil,
			desired:    []segmentAccessAssignment{{Type: segmentAccessTypeRole, ID: "role-1"}},
			wantAssign: []segmentAccessAssignment{{Type: segmentAccessTypeRole, ID: "role-1"}},
		},
		{
			name:         "pure removal",
			current:      []segmentAccessAssignment{{Type: segmentAccessTypeAccessProfile, ID: "ap-1"}},
			desired:      nil,
			wantRemovals: []segmentAccessAssignment{{Type: segmentAccessTypeAccessProfile, ID: "ap-1"}},
		},
		{
			name: "mixed add and remove",
			current: []segmentAccessAssignment{
				{Type: segmentAccessTypeRole, ID: "role-1"},
				{Type: segmentAccessTypeAccessProfile, ID: "ap-1"},
			},
			desired: []segmentAccessAssignment{
				{Type: segmentAccessTypeAccessProfile, ID: "ap-1"},
				{Type: segmentAccessTypeRole, ID: "role-2"},
			},
			wantAssign:   []segmentAccessAssignment{{Type: segmentAccessTypeRole, ID: "role-2"}},
			wantRemovals: []segmentAccessAssignment{{Type: segmentAccessTypeRole, ID: "role-1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAssign, gotRemovals := diffSegmentAccessAssignments(tt.current, tt.desired)
			if !reflect.DeepEqual(gotAssign, tt.wantAssign) {
				t.Fatalf("toAssign = %#v, want %#v", gotAssign, tt.wantAssign)
			}
			if !reflect.DeepEqual(gotRemovals, tt.wantRemovals) {
				t.Fatalf("toRemove = %#v, want %#v", gotRemovals, tt.wantRemovals)
			}
		})
	}
}

func TestSegmentAccessDesiredSegments(t *testing.T) {
	tests := []struct {
		name          string
		current       []string
		segmentID     string
		ensurePresent bool
		want          []string
		wantChanged   bool
	}{
		{
			name:          "add preserves existing and appends when absent",
			current:       []string{"seg-a", "seg-b"},
			segmentID:     "seg-c",
			ensurePresent: true,
			want:          []string{"seg-a", "seg-b", "seg-c"},
			wantChanged:   true,
		},
		{
			name:          "add is no-op when already present",
			current:       []string{"seg-a", "seg-b"},
			segmentID:     "seg-b",
			ensurePresent: true,
			want:          []string{"seg-a", "seg-b"},
			wantChanged:   false,
		},
		{
			name:          "remove preserves unrelated segments",
			current:       []string{"seg-a", "seg-b", "seg-c"},
			segmentID:     "seg-b",
			ensurePresent: false,
			want:          []string{"seg-a", "seg-c"},
			wantChanged:   true,
		},
		{
			name:          "remove is no-op when absent",
			current:       []string{"seg-a", "seg-c"},
			segmentID:     "seg-b",
			ensurePresent: false,
			want:          []string{"seg-a", "seg-c"},
			wantChanged:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotChanged := segmentAccessDesiredSegments(tt.current, tt.segmentID, tt.ensurePresent)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("segmentAccessDesiredSegments() = %#v, want %#v", got, tt.want)
			}
			if gotChanged != tt.wantChanged {
				t.Fatalf("changed = %v, want %v", gotChanged, tt.wantChanged)
			}
		})
	}
}

func TestSegmentAccessSegmentsPatch(t *testing.T) {
	t.Run("role", func(t *testing.T) {
		ops := segmentAccessRoleSegmentsPatch([]string{"seg-a", "seg-b"})
		if len(ops) != 1 {
			t.Fatalf("len(ops) = %d, want 1", len(ops))
		}
		if ops[0].Op != "replace" {
			t.Fatalf("Op = %q, want %q", ops[0].Op, "replace")
		}
		if ops[0].Path != "/segments" {
			t.Fatalf("Path = %q, want %q", ops[0].Path, "/segments")
		}
		if ops[0].Value == nil || ops[0].Value.ArrayOfArrayInner == nil {
			t.Fatal("Value.ArrayOfArrayInner is nil, want populated")
		}
		arr := *ops[0].Value.ArrayOfArrayInner
		if len(arr) != 2 {
			t.Fatalf("len(arr) = %d, want 2", len(arr))
		}
		for i, want := range []string{"seg-a", "seg-b"} {
			if arr[i].String == nil || *arr[i].String != want {
				t.Fatalf("arr[%d] = %v, want %q", i, arr[i].String, want)
			}
		}
	})

	t.Run("access_profile", func(t *testing.T) {
		ops := segmentAccessAccessProfileSegmentsPatch([]string{"seg-a", "seg-b"})
		if len(ops) != 1 {
			t.Fatalf("len(ops) = %d, want 1", len(ops))
		}
		if ops[0].Op != "replace" {
			t.Fatalf("Op = %q, want %q", ops[0].Op, "replace")
		}
		if ops[0].Path != "/segments" {
			t.Fatalf("Path = %q, want %q", ops[0].Path, "/segments")
		}
		if ops[0].Value == nil || ops[0].Value.ArrayOfArrayInner == nil {
			t.Fatal("Value.ArrayOfArrayInner is nil, want populated")
		}
		arr := *ops[0].Value.ArrayOfArrayInner
		if len(arr) != 2 {
			t.Fatalf("len(arr) = %d, want 2", len(arr))
		}
		for i, want := range []string{"seg-a", "seg-b"} {
			if arr[i].String == nil || *arr[i].String != want {
				t.Fatalf("arr[%d] = %v, want %q", i, arr[i].String, want)
			}
		}
	})
}
