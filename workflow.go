package main

import (
	"context"
	"log"
	"sync"

	"github.com/google/go-github/v62/github"
	"github.com/thoas/go-funk"
)

// Task - struct of a task
type Task struct {
	owner                              string
	repo                               string
	shouldRestartedFailed              bool
	shouldReactivateSuspendedWorkflows bool
	verbose                            bool
	last                               string
}

func reactivateSuspendedWorkflows(ctx context.Context, client *github.Client, task Task) {
	workflows, _, err := client.Actions.ListWorkflows(ctx, task.owner, task.repo, &github.ListOptions{
		Page:    1,
		PerPage: 100,
	})
	if err != nil {
		log.Panic(err)
	}
	for _, workflow := range workflows.Workflows {
		if *workflow.State == "disabled_inactivity" {
			_, err := client.Actions.EnableWorkflowByID(ctx, task.owner, task.repo, *workflow.ID)
			if err != nil {
				log.Println(err.Error())
			}
			if task.verbose {
				PrintReactivateWorkflowsStatus(task, workflow)
			}
		}
	}
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
	if task.shouldReactivateSuspendedWorkflows {
		reactivateSuspendedWorkflows(ctx, client, task)
	}
	filteredRuns := ProcessingWorkflowRuns(task, getWorkflowRuns(ctx, client, task))
	if len(filteredRuns) == 0 {
		return
	}
	if task.verbose {
		PrintRunnersStatus(task, filteredRuns)
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

func addTasksForLogin(ctx context.Context, client *github.Client, tasks *[]Task, userLogin, org string) {
	var repos []*github.Repository
	var err error
	if userLogin == "" {
		repos, _, err = client.Repositories.ListByOrg(ctx, org, nil)
	} else {
		org = userLogin
		repos, _, err = client.Repositories.List(ctx, userLogin, nil)
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
	shouldReactivateSuspendedWorkflows := GetBoolArgFromContext(ctx, "shouldReactivateSuspendedWorkflows")
	verbose := GetBoolArgFromContext(ctx, "verbose")
	last := GetStringArgFromContext(ctx, "last")
	for _, repo := range repos {
		*tasks = append(*tasks, Task{
			owner:                              org,
			repo:                               repo.GetName(),
			shouldRestartedFailed:              shouldRestartedFailed,
			shouldReactivateSuspendedWorkflows: shouldReactivateSuspendedWorkflows,
			verbose:                            verbose,
			last:                               last,
		})
	}
}

// GetWorkflowsStatus - get workflows status
func GetWorkflowsStatus(ctx context.Context, client *github.Client, userLogin string) {
	tasks := []Task{}
	addTasksForLogin(ctx, client, &tasks, userLogin, "")
	orgs, _, err := client.Organizations.List(ctx, userLogin, nil)
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
