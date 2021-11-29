package main

import (
	"context"
	"flag"
	"log"
	"sync"

	"github.com/google/go-github/v38/github"
)

// Task - struct of a task
type Task struct {
	owner                 string
	repo                  string
	shouldRestartedFailed bool
	verbose               bool
	last                  string
}

func worker(ctx context.Context, client *github.Client, task Task, wg *sync.WaitGroup) {
	defer wg.Done()
	var err error
	pushRuns, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, task.owner, task.repo, &github.ListWorkflowRunsOptions{
		Event: "push",
	})
	if err != nil {
		log.Panic(err)
	}
	scheduleRuns, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, task.owner, task.repo, &github.ListWorkflowRunsOptions{
		Event: "schedule",
	})
	if err != nil {
		log.Panic(err)
	}
	filteredRuns := ProcessingWorkflowRuns(task, append(pushRuns.WorkflowRuns, scheduleRuns.WorkflowRuns...))
	if len(filteredRuns) == 0 {
		return
	}
	if task.verbose {
		PrintStatus(task, filteredRuns)
	}
	if task.shouldRestartedFailed {
		for _, run := range filteredRuns {
			if run.GetConclusion() == "failure" {
				_, err = client.Actions.RerunWorkflowByID(ctx, task.owner, task.repo, run.GetID())
				if err != nil {
					log.Panic(err)
				}
			}
		}
	}
}

func addTasksForLogin(ctx context.Context, client *github.Client, tasks *[]Task, user, org string) {
	var repos []*github.Repository
	var err error
	if user == "" {
		repos, _, err = client.Repositories.ListByOrg(ctx, org, nil)
	} else {
		org = user
		repos, _, err = client.Repositories.List(ctx, user, nil)
	}
	if _, ok := err.(*github.RateLimitError); ok {
		log.Panic("hit rate limit")
	}
	if err != nil {
		log.Panic(err)
	}
	shouldRestartedFailed := GetBoolArgFromContext(ctx, "shouldRestartedFailed")
	verbose := GetBoolArgFromContext(ctx, "verbose")
	last := GetStringArgFromContext(ctx, "last")
	for _, repo := range repos {
		*tasks = append(*tasks, Task{
			owner:                 org,
			repo:                  repo.GetName(),
			shouldRestartedFailed: shouldRestartedFailed,
			verbose:               verbose,
			last:                  last,
		})
	}
}

func main() {
	githubLoginValue := flag.String("login", "", "github login")
	accessTokenValue := flag.String("token", "", "github token")
	shouldRestartedFailedValue := flag.Bool("restart", false, "should restarted failed (default: false)")
	verboseValue := flag.Bool("verbose", true, "verbose mode (default: true)")
	lastValue := flag.String("last", "30d", "get the results of actions for the last days (default: 30d)")
	flag.Parse()
	if *githubLoginValue == "" || *accessTokenValue == "" {
		log.Panic("should specify a github login and a github token")
	}

	ctx := context.Background()
	ctx = AddBoolArgToContext(ctx, "shouldRestartedFailed", *shouldRestartedFailedValue)
	ctx = AddBoolArgToContext(ctx, "verbose", *verboseValue)
	ctx = AddStringArgToContext(ctx, "last", *lastValue)
	client := GetClient(ctx, *accessTokenValue)

	tasks := []Task{}
	addTasksForLogin(ctx, client, &tasks, *githubLoginValue, "")
	orgs, _, err := client.Organizations.List(ctx, *githubLoginValue, nil)
	if err != nil {
		log.Panic(err)
	}
	for _, org := range orgs {
		addTasksForLogin(ctx, client, &tasks, "", org.GetLogin())
	}
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go worker(ctx, client, task, &wg)
	}
	wg.Wait()
}
