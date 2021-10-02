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
}

func worker(ctx context.Context, client *github.Client, task Task, wg *sync.WaitGroup) {
	defer wg.Done()
	var err error
	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, task.owner, task.repo, &github.ListWorkflowRunsOptions{
		Event: "push",
	})
	if err != nil {
		log.Panic(err)
	}
	filteredRuns := ProcessingWorkflowRuns(runs.WorkflowRuns)
	if len(filteredRuns) == 0 {
		return
	}
	PrintStatus(task, filteredRuns)
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
	shouldRestartedFailed := GetShouldRestartedFailedArgFromContext(ctx)
	for _, repo := range repos {
		*tasks = append(*tasks, Task{
			owner:                 org,
			repo:                  repo.GetName(),
			shouldRestartedFailed: shouldRestartedFailed,
		})
	}
}

func main() {
	githubLogin := flag.String("login", "", "github login")
	accessToken := flag.String("token", "", "github token")
	shouldRestartedFailedValue := flag.Bool("restart", false, "should restarted failed")
	flag.Parse()
	if *githubLogin == "" || *accessToken == "" {
		log.Panic("should specify a github login and a github token")
	}

	ctx := context.Background()
	ctx = AddShouldRestartedFailedArgToContext(ctx, *shouldRestartedFailedValue)
	client := GetClient(ctx, *accessToken)

	tasks := []Task{}
	addTasksForLogin(ctx, client, &tasks, *githubLogin, "")
	orgs, _, err := client.Organizations.List(ctx, *githubLogin, nil)
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
