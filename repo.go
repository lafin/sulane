package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/go-github/v62/github"
	"github.com/thoas/go-funk"
)

// parseOwnerAndRepoFromPullRequestAPIURL - parse owner and repo from a pull request API URL
// (https://api.github.com/repos/<owner>/<repo>/pulls/<number>). Reports false when the URL
// does not carry both segments.
func parseOwnerAndRepoFromPullRequestAPIURL(url string) (owner, repo string, ok bool) {
	parts := strings.Split(url, "/")
	if len(parts) < 6 {
		return "", "", false
	}
	owner, repo = parts[4], parts[5]
	if owner == "" || repo == "" {
		return "", "", false
	}
	return owner, repo, true
}

// mergeableStateRetries - how many times a pull request is re-fetched while GitHub is still
// computing its mergeable state.
const mergeableStateRetries = 3

// mergeableStateRetryDelay - pause between mergeable state re-fetches.
const mergeableStateRetryDelay = 2 * time.Second

// mergeableStateKnown - accept a pull request whose mergeable state GitHub has finished computing.
func mergeableStateKnown(pull *github.PullRequest) bool {
	return pull.Mergeable != nil && pull.GetMergeableState() != "unknown"
}

// mergeableStateMergeable - accept a pull request GitHub reports as ready to merge. A pull request
// that only lacked this tool's approval leaves "blocked" for "clean" once GitHub recomputes the
// state; "unstable" and "has_hooks" are mergeable too, they only carry a check that is not
// required. A pull request whose required status checks fail stays "blocked" no matter who
// approved it, so it never matches.
func mergeableStateMergeable(pull *github.PullRequest) bool {
	switch pull.GetMergeableState() {
	case "clean", "unstable", "has_hooks":
		return true
	}
	return false
}

// fetchPullRequestUntil - fetch a pull request, re-fetching until accept reports true. GitHub
// computes the mergeable state asynchronously, so a cold fetch usually reports "unknown" and a
// fetch right after an approval still reports the pre-approval state. Gives up after
// mergeableStateRetries re-fetches and then returns the last pull request together with an error,
// so the caller can report the state it settled on; the returned pull request is only safe to act
// on when the error is nil.
func fetchPullRequestUntil(ctx context.Context, client *github.Client, owner, repo string, prNumber int, accept func(*github.PullRequest) bool) (*github.PullRequest, error) {
	for attempt := 0; ; attempt++ {
		pull, _, err := client.PullRequests.Get(ctx, owner, repo, prNumber)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch pull request: %w", err)
		}
		if pull == nil {
			return nil, fmt.Errorf("pull request %s/%s#%d: empty response", owner, repo, prNumber)
		}
		if accept(pull) {
			return pull, nil
		}
		if attempt >= mergeableStateRetries {
			return pull, fmt.Errorf("mergeable state is %q after %d fetches", pull.GetMergeableState(), attempt+1)
		}
		select {
		case <-ctx.Done():
			return pull, fmt.Errorf("waiting for the mergeable state: %w", ctx.Err())
		case <-time.After(mergeableStateRetryDelay):
		}
	}
}

// DoMergeOnePrPerDayIfNoActionToday - do merge one PR per day if no action today
func DoMergeOnePrPerDayIfNoActionToday(ctx context.Context, client *github.Client, userLogin string) {
	dry := GetBoolArgFromContext(ctx, keyDry)
	verbose := GetBoolArgFromContext(ctx, keyVerbose)

	events, _, err := client.Activity.ListEventsPerformedByUser(ctx, userLogin, false, &github.ListOptions{
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		log.Printf("Cannot list events performed by %s: %v", userLogin, err)
		return
	}
	event := funk.Find(events, func(event *github.Event) bool {
		return event.GetCreatedAt().Format("2006-01-02") == time.Now().Format("2006-01-02")
	})
	if event != nil {
		if verbose {
			log.Printf("Skipping merge: %s already has activity today", userLogin)
		}
		return
	}
	result, _, err := client.Search.Issues(ctx, fmt.Sprintf("is:open is:pr assignee:%s user:%s label:\"automated pr\"", userLogin, userLogin), &github.SearchOptions{
		Sort:  "created",
		Order: "asc",
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: 10},
	})
	if err != nil {
		log.Printf("Cannot search for automated pull requests of %s: %v", userLogin, err)
		return
	}
	for _, issue := range result.Issues {
		link := issue.GetPullRequestLinks().GetURL()
		owner, repo, ok := parseOwnerAndRepoFromPullRequestAPIURL(link)
		if !ok {
			log.Printf("Skipping %s: cannot parse owner and repo from pull request URL %q", issue.GetHTMLURL(), link)
			continue
		}
		if owner != userLogin {
			log.Printf("Skipping %s: owner %s is not %s", issue.GetHTMLURL(), owner, userLogin)
			continue
		}
		pull, err := fetchPullRequestUntil(ctx, client, owner, repo, issue.GetNumber(), mergeableStateKnown)
		if err != nil {
			log.Printf("Skipping %s: %v", issue.GetHTMLURL(), err)
			continue
		}
		// Only automation accounts (peter-evans/create-pull-request commits as github-actions[bot])
		// and the operator may be self-approved: this function writes the only approval the pull
		// request has, so a third party must never reach the merge below.
		author := pull.GetUser()
		if author.GetType() != "Bot" && author.GetLogin() != userLogin {
			log.Printf("Skipping %s: author %q (type %q) is neither an automation account nor %s", pull.GetHTMLURL(), author.GetLogin(), author.GetType(), userLogin)
			continue
		}
		// States this flow can never fix; "blocked" is deliberately absent because the approval
		// below can clear it.
		if state := pull.GetMergeableState(); state == "dirty" || state == "draft" || state == "behind" {
			log.Printf("Skipping %s: mergeable state is %q", pull.GetHTMLURL(), state)
			continue
		}
		headSHA := pull.GetHead().GetSHA()
		if headSHA == "" {
			log.Printf("Skipping %s: head SHA is unknown", pull.GetHTMLURL())
			continue
		}
		if dry {
			log.Printf("Dry run: would approve and merge %s (%s) at %s", pull.GetHTMLURL(), pull.GetTitle(), headSHA)
			return
		}
		if err := ApprovePullRequest(ctx, client, owner, repo, pull.GetNumber(), headSHA); err != nil {
			log.Printf("Not merging %s: cannot approve: %v", pull.GetHTMLURL(), err)
			return
		}
		// Re-read the state until GitHub reports the pull request as mergeable: a branch protected
		// by a required review reports "blocked" until the approval above lands, and that
		// recomputation is asynchronous, so the first fetch usually still reports "blocked".
		// Failing required status checks keep a pull request "blocked" no matter who approved it,
		// so the merge below is only reached once GitHub itself considers it mergeable.
		if _, err := fetchPullRequestUntil(ctx, client, owner, repo, pull.GetNumber(), mergeableStateMergeable); err != nil {
			// The approval is already written, so stop instead of approving a second pull request.
			log.Printf("Not merging %s: %v after approving; leaving it for a later run", pull.GetHTMLURL(), err)
			return
		}
		repository, _, err := client.Repositories.Get(ctx, owner, repo)
		if err != nil {
			log.Printf("Not merging %s: cannot fetch repository: %v", pull.GetHTMLURL(), err)
			return
		}
		mergeMethod := "merge"
		if repository.GetAllowRebaseMerge() {
			mergeMethod = "rebase"
		} else if repository.GetAllowSquashMerge() {
			mergeMethod = "squash"
		}
		_, _, err = client.PullRequests.Merge(ctx, owner, repo, pull.GetNumber(), "", &github.PullRequestOptions{
			MergeMethod: mergeMethod,
			SHA:         headSHA,
		})
		if err != nil {
			log.Printf("Cannot merge %s: %v", pull.GetHTMLURL(), err)
			return
		}
		if verbose {
			log.Printf("Merged %s (%s) at %s", pull.GetHTMLURL(), pull.GetTitle(), headSHA)
		}
		return
	}
}
