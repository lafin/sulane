package main

import (
	"context"
	"flag"
	"log"
	"sync"

	"github.com/google/go-github/v38/github"
	"github.com/thoas/go-funk"
)

// Task - struct of a task
type Task struct {
	owner                 string
	repo                  string
	shouldRestartedFailed bool
	verbose               bool
	last                  string
}

func getWorkflowRuns(ctx context.Context, client *github.Client, task Task) []*github.WorkflowRun {
	var runs []*github.WorkflowRun
	for _, event := range []string{"push", "schedule", "workflow_dispatch"} {
		workflowRuns, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, task.owner, task.repo, &github.ListWorkflowRunsOptions{
			Event: event,
		})
		if err != nil {
			log.Panic(err)
		}
		runs = append(runs, workflowRuns.WorkflowRuns...)
	}
	return runs
}

func worker(ctx context.Context, client *github.Client, task Task, wg *sync.WaitGroup) {
	defer wg.Done()
	filteredRuns := ProcessingWorkflowRuns(task, getWorkflowRuns(ctx, client, task))
	if len(filteredRuns) == 0 {
		return
	}
	if task.verbose {
		PrintStatus(task, filteredRuns)
	}
	if task.shouldRestartedFailed {
		for _, run := range filteredRuns {
			if run.GetConclusion() == "failure" {
				_, err := client.Actions.RerunWorkflowByID(ctx, task.owner, task.repo, run.GetID())
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
	skipArchive := GetBoolArgFromContext(ctx, "skipArchive")
	if skipArchive {
		repos = funk.Filter(repos, func(repo *github.Repository) bool {
			return !repo.GetArchived()
		}).([]*github.Repository)
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
	shouldRestartedFailedValue := flag.Bool("restart", false, "should restarted failed")
	verboseValue := flag.Bool("verbose", true, "verbose mode")
	lastValue := flag.String("last", "30d", "get the results of actions for the last days")
	skipArchiveValue := flag.Bool("skipArchive", true, "skip archived")
	flag.Parse()
	if *githubLoginValue == "" || *accessTokenValue == "" {
		log.Println("should specify a github login and a github token")
		return
	}

	ctx := context.Background()
	ctx = AddBoolArgToContext(ctx, "shouldRestartedFailed", *shouldRestartedFailedValue)
	ctx = AddBoolArgToContext(ctx, "verbose", *verboseValue)
	ctx = AddStringArgToContext(ctx, "last", *lastValue)
	ctx = AddBoolArgToContext(ctx, "skipArchive", *skipArchiveValue)
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
