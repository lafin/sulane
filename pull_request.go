package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v62/github"
	"github.com/thoas/go-funk"
)

// ApprovePullRequest - approve pull request
func ApprovePullRequest(ctx context.Context, client *github.Client, owner, repo string, prNumber int) {
	_, _, err := client.PullRequests.CreateReview(ctx, owner, repo, prNumber, &github.PullRequestReviewRequest{
		Event: github.String("APPROVE"),
	})
	if err != nil {
		log.Panic(err)
	}
}

func getOwnerAndRepoFromURL(url string) (owner, repo string) {
	url = url[29:]
	parts := strings.Split(url, "/")
	return parts[0], parts[1]
}

// AutoApprovePullRequests - auto approve pull requests
func AutoApprovePullRequests(ctx context.Context, client *github.Client) {
	verbose := GetBoolArgFromContext(ctx, "verbose")
	dry := GetBoolArgFromContext(ctx, "dry")
	query := GetStringArgFromContext(ctx, "query")
	shouldAutoApproveIfReviewedBy := GetStringArgFromContext(ctx, "shouldAutoApproveIfReviewedBy")
	shouldAutoApproveIfCreatedBy := GetStringArgFromContext(ctx, "shouldAutoApproveIfCreatedBy")

	result, _, err := client.Search.Issues(ctx, query, nil)
	if err != nil {
		log.Panic(err)
	}
	for _, issue := range result.Issues {
		if !(!issue.GetDraft() && issue.IsPullRequest()) {
			continue
		}
		pr := issue
		owner, repo := getOwnerAndRepoFromURL(pr.GetRepositoryURL())
		if pr.GetUser().GetLogin() == shouldAutoApproveIfCreatedBy {
			if verbose {
				PrintPullRequestApproveStatus(pr)
			}
			if !dry {
				ApprovePullRequest(ctx, client, owner, repo, pr.GetNumber())
				SendTelegramMessage(ctx, fmt.Sprintf("Approved PR: %s\n%s", pr.GetHTMLURL(), pr.GetTitle()))
			}
		} else {
			reviews, _, err := client.PullRequests.ListReviews(ctx, owner, repo, pr.GetNumber(), &github.ListOptions{
				Page:    1,
				PerPage: 100,
			})
			if err != nil {
				log.Panic(err)
			}
			if funk.Some(reviews, func(review *github.PullRequestReview) bool {
				return review.GetState() == "APPROVED" && review.GetUser().GetLogin() == shouldAutoApproveIfReviewedBy
			}) {
				if verbose {
					PrintPullRequestApproveStatus(pr)
				}
				if !dry {
					ApprovePullRequest(ctx, client, owner, repo, pr.GetNumber())
					SendTelegramMessage(ctx, fmt.Sprintf("Approved PR: %s\n%s", pr.GetHTMLURL(), pr.GetTitle()))
				}
			}
		}
	}
}
