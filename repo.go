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

// DoMergeOnePrPerDayIfNoActionToday - do merge one PR per day if no action today
func DoMergeOnePrPerDayIfNoActionToday(ctx context.Context, client *github.Client, userLogin string) {
	events, _, err := client.Activity.ListEventsPerformedByUser(ctx, userLogin, false, &github.ListOptions{
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		log.Panic(err)
	}
	event := funk.Find(events, func(event *github.Event) bool {
		return event.GetCreatedAt().Format("2006-01-02") == time.Now().Format("2006-01-02")
	})
	if event == nil {
		result, _, err := client.Search.Issues(ctx, fmt.Sprintf("is:open is:pr assignee:%s label:\"automated pr\"", userLogin), &github.SearchOptions{
			ListOptions: github.ListOptions{
				Page:    1,
				PerPage: 10},
		})
		if err != nil {
			log.Panic(err)
		}
		if len(result.Issues) == 0 {
			return
		}
		pr := result.Issues[0]
		parsedRepoLink := strings.Split(pr.GetPullRequestLinks().GetURL(), "/")
		owner := parsedRepoLink[4]
		repo := parsedRepoLink[5]
		_, _, err = client.PullRequests.CreateReview(ctx, owner, repo, pr.GetNumber(), &github.PullRequestReviewRequest{
			Event: github.String("APPROVE"),
		})
		if err != nil {
			log.Panic(err)
		}
		repository, _, err := client.Repositories.Get(ctx, owner, repo)
		if err != nil {
			log.Panic(err)
		}
		mergeMethod := "merge"
		if repository.GetAllowRebaseMerge() {
			mergeMethod = "rebase"
		} else if repository.GetAllowSquashMerge() {
			mergeMethod = "squash"
		}
		_, _, err = client.PullRequests.Merge(ctx, owner, repo, pr.GetNumber(), "", &github.PullRequestOptions{
			MergeMethod: mergeMethod,
		})
		if err != nil {
			log.Panic(err)
		}
	}
}
