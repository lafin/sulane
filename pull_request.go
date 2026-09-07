package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v62/github"
)

// maxSearchPages - upper bound on the number of search result pages fetched per query
const maxSearchPages = 5

// maxReviewPages - upper bound on the number of review pages fetched per pull request
const maxReviewPages = 10

// ApprovePullRequest - approve pull request; when commitID is not empty the review is
// recorded against that commit. GitHub only rejects a commit id that is not part of the
// pull request, so pinning does not by itself invalidate the approval after a later push:
// whether a new head dismisses it depends on the branch protection setting
// "dismiss stale pull request approvals when new commits are pushed"
func ApprovePullRequest(ctx context.Context, client *github.Client, owner, repo string, prNumber int, commitID string) error {
	review := &github.PullRequestReviewRequest{
		Event: github.String("APPROVE"),
	}
	if commitID != "" {
		review.CommitID = github.String(commitID)
	}
	_, _, err := client.PullRequests.CreateReview(ctx, owner, repo, prNumber, review)
	return err
}

// getOwnerAndRepoFromURL - take owner and repo from the last two segments of a
// repository API URL; both are empty when the URL does not carry them
func getOwnerAndRepoFromURL(url string) (owner, repo string) {
	parts := strings.Split(strings.Trim(url, "/"), "/")
	if len(parts) < 2 {
		return "", ""
	}
	owner, repo = parts[len(parts)-2], parts[len(parts)-1]
	if owner == "" || repo == "" {
		return "", ""
	}
	return owner, repo
}

// searchPullRequestIssues - run a search query and collect up to maxSearchPages pages of results
func searchPullRequestIssues(ctx context.Context, client *github.Client, query string) ([]*github.Issue, error) {
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var issues []*github.Issue
	nextPage := 0
	for range maxSearchPages {
		result, resp, err := client.Search.Issues(ctx, query, opts)
		if err != nil {
			return nil, err
		}
		issues = append(issues, result.Issues...)
		nextPage = resp.NextPage
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	if nextPage != 0 {
		log.Printf("search %q truncated after %d pages", query, maxSearchPages)
	}
	return issues, nil
}

// listPullRequestReviews - collect every page of reviews of a pull request, oldest first;
// judging the last state changing review needs the complete list, so a run truncated by
// maxReviewPages fails closed with an error instead of returning a partial list
func listPullRequestReviews(ctx context.Context, client *github.Client, owner, repo string, prNumber int) ([]*github.PullRequestReview, error) {
	opts := &github.ListOptions{PerPage: 100}
	var reviews []*github.PullRequestReview
	for range maxReviewPages {
		pageReviews, resp, err := client.PullRequests.ListReviews(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, pageReviews...)
		if resp.NextPage == 0 {
			return reviews, nil
		}
		opts.Page = resp.NextPage
	}
	return nil, fmt.Errorf("review list longer than %d pages", maxReviewPages)
}

// hasCurrentHeadApprovalBy - report whether the last state changing review of login
// approves headSHA; COMMENTED reviews do not change state and are ignored, a later
// CHANGES_REQUESTED or DISMISSED review revokes an earlier approval
func hasCurrentHeadApprovalBy(reviews []*github.PullRequestReview, login, headSHA string) bool {
	if login == "" || headSHA == "" {
		return false
	}
	var last *github.PullRequestReview
	for _, review := range reviews {
		if review.GetUser().GetLogin() != login {
			continue
		}
		if strings.ToUpper(review.GetState()) == "COMMENTED" {
			continue
		}
		last = review
	}
	if last == nil {
		return false
	}
	return strings.ToUpper(last.GetState()) == "APPROVED" && last.GetCommitID() == headSHA
}

// AutoApprovePullRequests - auto approve pull requests
func AutoApprovePullRequests(ctx context.Context, client *github.Client) {
	verbose := GetBoolArgFromContext(ctx, keyVerbose)
	dry := GetBoolArgFromContext(ctx, keyDry)
	query := GetStringArgFromContext(ctx, keyQuery)
	shouldAutoApproveIfReviewedBy := GetStringArgFromContext(ctx, keyShouldAutoApproveIfReviewedBy)
	shouldAutoApproveIfCreatedBy := GetStringArgFromContext(ctx, keyShouldAutoApproveIfCreatedBy)

	if shouldAutoApproveIfCreatedBy == "" && shouldAutoApproveIfReviewedBy == "" {
		if verbose {
			log.Printf("skipping search %q: no auto approve criterion configured", query)
		}
		return
	}

	issues, err := searchPullRequestIssues(ctx, client, query)
	if err != nil {
		log.Printf("search %q: %v", query, err)
		return
	}

	for _, pr := range issues {
		if pr == nil || pr.GetDraft() || !pr.IsPullRequest() {
			continue
		}
		owner, repo := getOwnerAndRepoFromURL(pr.GetRepositoryURL())
		if owner == "" || repo == "" {
			if verbose {
				log.Printf("skipping %s: cannot read owner and repo from %q", pr.GetHTMLURL(), pr.GetRepositoryURL())
			}
			continue
		}

		pull, _, err := client.PullRequests.Get(ctx, owner, repo, pr.GetNumber())
		if err != nil {
			log.Printf("get pull request %s/%s#%d: %v", owner, repo, pr.GetNumber(), err)
			continue
		}
		headSHA := pull.GetHead().GetSHA()
		if headSHA == "" {
			if verbose {
				log.Printf("skipping %s: head SHA is unknown", pr.GetHTMLURL())
			}
			continue
		}

		approve := false
		switch {
		case shouldAutoApproveIfCreatedBy != "" && pr.GetUser().GetLogin() == shouldAutoApproveIfCreatedBy:
			approve = true
		case shouldAutoApproveIfReviewedBy != "":
			reviews, err := listPullRequestReviews(ctx, client, owner, repo, pr.GetNumber())
			if err != nil {
				log.Printf("list reviews of %s/%s#%d: %v", owner, repo, pr.GetNumber(), err)
				continue
			}
			approve = hasCurrentHeadApprovalBy(reviews, shouldAutoApproveIfReviewedBy, headSHA)
			if !approve && verbose {
				log.Printf("skipping %s: no approval of %s for head %s", pr.GetHTMLURL(), shouldAutoApproveIfReviewedBy, headSHA)
			}
		}
		if !approve {
			continue
		}

		if dry {
			log.Printf("[dry] would approve %s at %s", pr.GetHTMLURL(), headSHA)
			continue
		}
		if err := ApprovePullRequest(ctx, client, owner, repo, pr.GetNumber(), headSHA); err != nil {
			log.Printf("approve %s/%s#%d: %v", owner, repo, pr.GetNumber(), err)
			continue
		}
		if verbose {
			PrintPullRequestApproveStatus(pr)
		}
		SendTelegramMessage(ctx, fmt.Sprintf("Approved PR: %s\n%s", pr.GetHTMLURL(), pr.GetTitle()))
	}
}
