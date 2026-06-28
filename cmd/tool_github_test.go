package cmd

import (
	"reflect"
	"testing"
)

func TestCollectGitHubPaginatedObjects(t *testing.T) {
	t.Parallel()

	var counts []int
	got, err := collectGitHubPaginatedObjects(func(handle githubPageHandler) error {
		for _, raw := range [][]byte{
			[]byte(`[{"id":1}]`),
			[]byte(`[{"id":2},{"id":3}]`),
			[]byte(`   `),
		} {
			count, err := handle(raw)
			if err != nil {
				return err
			}
			counts = append(counts, count)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("collectGitHubPaginatedObjects returned error: %v", err)
	}

	if !reflect.DeepEqual(counts, []int{1, 2, 0}) {
		t.Fatalf("page counts = %v, want [1 2 0]", counts)
	}
	if len(got) != 3 {
		t.Fatalf("collected %d objects, want 3", len(got))
	}
	for i, want := range []int{1, 2, 3} {
		if id := jsonNumberToInt(got[i]["id"]); id != want {
			t.Fatalf("object %d id = %d, want %d", i, id, want)
		}
	}
}

func TestCollectGitHubPaginatedCheckRunsPreservesFirstTotal(t *testing.T) {
	t.Parallel()

	total, runs, err := collectGitHubPaginatedCheckRuns(func(handle githubPageHandler) error {
		for _, raw := range [][]byte{
			[]byte(`{"total_count":3,"check_runs":[{"id":11}]}`),
			[]byte(`{"total_count":99,"check_runs":[{"id":12},{"id":13}]}`),
		} {
			if _, err := handle(raw); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("collectGitHubPaginatedCheckRuns returned error: %v", err)
	}

	if total != 3 {
		t.Fatalf("total = %d, want first page total 3", total)
	}
	if len(runs) != 3 {
		t.Fatalf("collected %d check runs, want 3", len(runs))
	}
	for i, want := range []int{11, 12, 13} {
		if id := jsonNumberToInt(runs[i]["id"]); id != want {
			t.Fatalf("check run %d id = %d, want %d", i, id, want)
		}
	}
}

func TestExtractGitHubCombinedStatusesSkipsNonObjects(t *testing.T) {
	t.Parallel()

	got := extractGitHubCombinedStatuses(map[string]any{
		"statuses": []any{
			map[string]any{"context": "ci"},
			"not-an-object",
			map[string]any{"context": "lint"},
			nil,
		},
	})

	if len(got) != 2 {
		t.Fatalf("statuses length = %d, want 2", len(got))
	}
	if got[0]["context"] != "ci" || got[1]["context"] != "lint" {
		t.Fatalf("statuses = %v, want ci and lint contexts", got)
	}
}

func TestAddGitHubPaginationParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		page int
		want string
	}{
		{
			name: "first page omits explicit page",
			path: "/repos/owner/repo/issues",
			page: 0,
			want: "/repos/owner/repo/issues?per_page=100",
		},
		{
			name: "numbered page preserves existing query",
			path: "/repos/owner/repo/issues?state=open",
			page: 2,
			want: "/repos/owner/repo/issues?state=open&per_page=100&page=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := addGitHubPaginationParams(tt.path, tt.page); got != tt.want {
				t.Fatalf("addGitHubPaginationParams(%q, %d) = %q, want %q", tt.path, tt.page, got, tt.want)
			}
		})
	}
}
