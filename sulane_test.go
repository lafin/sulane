package main

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v62/github"
)

// workflowRun builds a workflow run carrying only the fields the filtering reads.
func workflowRun(name, branch, conclusion string, createdAt time.Time) *github.WorkflowRun {
	return &github.WorkflowRun{
		Name:       github.String(name),
		HeadBranch: github.String(branch),
		CreatedAt:  &github.Timestamp{Time: createdAt},
		Conclusion: github.String(conclusion),
	}
}

// runKeys renders the identifying fields of every run so a filtered result can be
// compared as an ordered list.
func runKeys(runs []*github.WorkflowRun) []string {
	keys := make([]string, 0, len(runs))
	for _, run := range runs {
		keys = append(keys, fmt.Sprintf("%s@%s@%s", run.GetName(), run.GetHeadBranch(), run.GetConclusion()))
	}
	return keys
}

// review builds a pull request review carrying only the fields the approval check reads.
func review(login, state, commitID string) *github.PullRequestReview {
	return &github.PullRequestReview{
		User:     &github.User{Login: github.String(login)},
		State:    github.String(state),
		CommitID: github.String(commitID),
	}
}

func TestParseDaysNumberFlag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "thirty days", input: "30d", want: 30},
		{name: "seven days", input: "7d", want: 7},
		{name: "missing unit", input: "30", wantErr: true},
		{name: "week unit", input: "2w", wantErr: true},
		{name: "unit without a number", input: "d", wantErr: true},
		{name: "zero days", input: "0d", wantErr: true},
		{name: "empty value", input: "", wantErr: true},
		{name: "leading garbage", input: "xx30d", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDaysNumberFlag(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseDaysNumberFlag(%q) = (%d, nil), want an error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDaysNumberFlag(%q) returned an unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseDaysNumberFlag(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestProcessingWorkflowRuns(t *testing.T) {
	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)
	threeHoursAgo := now.Add(-3 * time.Hour)
	longAgo := now.Add(-40 * 24 * time.Hour)

	tests := []struct {
		name string
		last string
		runs []*github.WorkflowRun
		want []string
	}{
		{
			name: "a newer run on a side branch does not hide the main branch run",
			last: "30d",
			runs: []*github.WorkflowRun{
				workflowRun("ci", "feature", "success", hourAgo),
				workflowRun("ci", "main", "failure", twoHoursAgo),
			},
			want: []string{"ci@main@failure"},
		},
		{
			name: "runs older than the window are excluded",
			last: "30d",
			runs: []*github.WorkflowRun{
				workflowRun("ci", "main", "success", hourAgo),
				workflowRun("release", "main", "success", longAgo),
			},
			want: []string{"ci@main@success"},
		},
		{
			name: "only the newest main branch run of each workflow survives",
			last: "30d",
			runs: []*github.WorkflowRun{
				workflowRun("ci", "main", "success", hourAgo),
				workflowRun("build", "master", "success", twoHoursAgo),
				workflowRun("ci", "main", "failure", threeHoursAgo),
			},
			want: []string{"ci@main@success", "build@master@success"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProcessingWorkflowRuns(Task{owner: "lafin", repo: "sulane", last: tt.last}, tt.runs)
			if err != nil {
				t.Fatalf("ProcessingWorkflowRuns returned an unexpected error: %v", err)
			}
			if keys := runKeys(got); !slices.Equal(keys, tt.want) {
				t.Errorf("ProcessingWorkflowRuns = %v, want %v", keys, tt.want)
			}
		})
	}
}

func TestProcessingWorkflowRunsInvalidWindow(t *testing.T) {
	runs := []*github.WorkflowRun{workflowRun("ci", "main", "success", time.Now().Add(-1*time.Hour))}

	got, err := ProcessingWorkflowRuns(Task{owner: "lafin", repo: "sulane", last: "2w"}, runs)
	if err == nil {
		t.Fatalf("ProcessingWorkflowRuns with last %q returned no error", "2w")
	}
	if len(got) != 0 {
		t.Errorf("ProcessingWorkflowRuns with an invalid window returned %d runs, want none", len(got))
	}
}

func TestMergeConfig(t *testing.T) {
	tests := []struct {
		name string
		user *WorkflowsConfig
		want *WorkflowsConfig
	}{
		{
			name: "explicitly disabled flags survive the enabled defaults",
			user: &WorkflowsConfig{
				Active:                             true,
				ShouldReactivateSuspendedWorkflows: boolPtr(false),
				SkipArchive:                        boolPtr(false),
				Last:                               "7d",
			},
			want: &WorkflowsConfig{
				ShouldRestartFailed:                boolPtr(false),
				ShouldReactivateSuspendedWorkflows: boolPtr(false),
				SkipArchive:                        boolPtr(false),
				DoMergeOnePrPerDayIfNoActionToday:  boolPtr(false),
				Last:                               "7d",
			},
		},
		{
			name: "omitted keys receive the defaults",
			user: &WorkflowsConfig{Active: true},
			want: &WorkflowsConfig{
				ShouldRestartFailed:                boolPtr(false),
				ShouldReactivateSuspendedWorkflows: boolPtr(true),
				SkipArchive:                        boolPtr(true),
				DoMergeOnePrPerDayIfNoActionToday:  boolPtr(false),
				Last:                               "30d",
			},
		},
		{
			name: "explicitly enabled flags stay enabled",
			user: &WorkflowsConfig{
				Active:                             true,
				ShouldRestartFailed:                boolPtr(true),
				ShouldReactivateSuspendedWorkflows: boolPtr(true),
				SkipArchive:                        boolPtr(true),
				DoMergeOnePrPerDayIfNoActionToday:  boolPtr(true),
				Last:                               "1d",
			},
			want: &WorkflowsConfig{
				ShouldRestartFailed:                boolPtr(true),
				ShouldReactivateSuspendedWorkflows: boolPtr(true),
				SkipArchive:                        boolPtr(true),
				DoMergeOnePrPerDayIfNoActionToday:  boolPtr(true),
				Last:                               "1d",
			},
		},
	}

	checkFlag := func(t *testing.T, field string, got, want *bool) {
		t.Helper()
		if got == nil {
			t.Errorf("%s = nil, want %t", field, *want)
			return
		}
		if *got != *want {
			t.Errorf("%s = %t, want %t", field, *got, *want)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := MergeConfig(&Config{Workflows: tt.user}, defaultConfig())
			if merged.Workflows == nil {
				t.Fatal("MergeConfig dropped the workflows section")
			}
			checkFlag(t, "should_restart_failed", merged.Workflows.ShouldRestartFailed, tt.want.ShouldRestartFailed)
			checkFlag(t, "should_reactivate_suspended_workflows", merged.Workflows.ShouldReactivateSuspendedWorkflows, tt.want.ShouldReactivateSuspendedWorkflows)
			checkFlag(t, "skip_archive", merged.Workflows.SkipArchive, tt.want.SkipArchive)
			checkFlag(t, "do_merge_one_pr_per_day_if_no_action_today", merged.Workflows.DoMergeOnePrPerDayIfNoActionToday, tt.want.DoMergeOnePrPerDayIfNoActionToday)
			if merged.Workflows.Last != tt.want.Last {
				t.Errorf("last = %q, want %q", merged.Workflows.Last, tt.want.Last)
			}
		})
	}
}

func TestRedactTelegramError(t *testing.T) {
	const (
		token       = "123456:AAH-secret"
		requestURL  = "https://api.telegram.org/bot" + token + "/sendMessage"
		redactedURL = "https://api.telegram.org/bot***/sendMessage"
	)
	// The telegram library returns transport failures as *url.Error, whose message
	// carries the request URL and therefore the bot token.
	urlError := &url.Error{Op: "Post", URL: requestURL, Err: errors.New("dial tcp: i/o timeout")}

	tests := []struct {
		name        string
		err         error
		token       string
		want        string
		wantRedacts bool
	}{
		{name: "no error", err: nil, token: token, want: ""},
		{name: "no token configured", err: errors.New("dial tcp: i/o timeout"), token: "", want: "dial tcp: i/o timeout"},
		{
			name:        "token in the request url",
			err:         urlError,
			token:       token,
			want:        `Post "` + redactedURL + `": dial tcp: i/o timeout`,
			wantRedacts: true,
		},
		{
			name:        "token repeated",
			err:         fmt.Errorf("send to %s failed, retry with %s", token, token),
			token:       token,
			want:        "send to *** failed, retry with ***",
			wantRedacts: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactTelegramError(tt.err, tt.token)
			if got != tt.want {
				t.Errorf("redactTelegramError = %q, want %q", got, tt.want)
			}
			if !tt.wantRedacts {
				return
			}
			if strings.Contains(got, tt.token) {
				t.Errorf("redactTelegramError = %q, it still carries the bot token", got)
			}
			if !strings.Contains(got, "***") {
				t.Errorf("redactTelegramError = %q, want every token occurrence replaced by %q", got, "***")
			}
		})
	}
}

func TestGetOwnerAndRepoFromURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
	}{
		{name: "repository api url", url: "https://api.github.com/repos/lafin/sulane", wantOwner: "lafin", wantRepo: "sulane"},
		{name: "trailing slash", url: "https://api.github.com/repos/lafin/sulane/", wantOwner: "lafin", wantRepo: "sulane"},
		{name: "empty url", url: ""},
		{name: "host only", url: "https://api.github.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo := getOwnerAndRepoFromURL(tt.url)
			if owner != tt.wantOwner || repo != tt.wantRepo {
				t.Errorf("getOwnerAndRepoFromURL(%q) = (%q, %q), want (%q, %q)", tt.url, owner, repo, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}

func TestParseOwnerAndRepoFromPullRequestAPIURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{name: "pull request api url", url: "https://api.github.com/repos/lafin/sulane/pulls/42", wantOwner: "lafin", wantRepo: "sulane", wantOK: true},
		{name: "empty url", url: ""},
		{name: "too few segments", url: "https://api.github.com/repos/lafin"},
		{name: "blank owner segment", url: "https://api.github.com/repos//sulane/pulls/1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, ok := parseOwnerAndRepoFromPullRequestAPIURL(tt.url)
			if ok != tt.wantOK || owner != tt.wantOwner || repo != tt.wantRepo {
				t.Errorf("parseOwnerAndRepoFromPullRequestAPIURL(%q) = (%q, %q, %t), want (%q, %q, %t)",
					tt.url, owner, repo, ok, tt.wantOwner, tt.wantRepo, tt.wantOK)
			}
		})
	}
}

func TestHasCurrentHeadApprovalBy(t *testing.T) {
	const (
		head  = "3f1a9c0d2b4e6f8a1c3d5e7f9a0b2c4d6e8f0a1b"
		stale = "0b1a2c3d4e5f60718293a4b5c6d7e8f9a0b1c2d3"
	)

	tests := []struct {
		name string
		revs []*github.PullRequestReview
		user string
		head string
		want bool
	}{
		{
			name: "approval of the trusted login on the current head",
			revs: []*github.PullRequestReview{review("trusted", "APPROVED", head)},
			user: "trusted",
			head: head,
			want: true,
		},
		{
			name: "approval pinned to a stale commit",
			revs: []*github.PullRequestReview{review("trusted", "APPROVED", stale)},
			user: "trusted",
			head: head,
		},
		{
			name: "changes requested after the approval",
			revs: []*github.PullRequestReview{
				review("trusted", "APPROVED", head),
				review("trusted", "CHANGES_REQUESTED", head),
			},
			user: "trusted",
			head: head,
		},
		{
			name: "comment after the approval does not revoke it",
			revs: []*github.PullRequestReview{
				review("trusted", "APPROVED", head),
				review("trusted", "COMMENTED", head),
			},
			user: "trusted",
			head: head,
			want: true,
		},
		{
			name: "approval of another login",
			revs: []*github.PullRequestReview{review("stranger", "APPROVED", head)},
			user: "trusted",
			head: head,
		},
		{
			name: "no trusted login configured",
			revs: []*github.PullRequestReview{review("trusted", "APPROVED", head)},
			user: "",
			head: head,
		},
		{
			name: "unknown head sha",
			revs: []*github.PullRequestReview{review("trusted", "APPROVED", "")},
			user: "trusted",
			head: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasCurrentHeadApprovalBy(tt.revs, tt.user, tt.head); got != tt.want {
				t.Errorf("hasCurrentHeadApprovalBy(%q, %q) = %t, want %t", tt.user, tt.head, got, tt.want)
			}
		})
	}
}
