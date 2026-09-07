package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/go-github/v62/github"
	"github.com/thoas/go-funk"
)

const (
	// listPerPage is the maximum page size the GitHub REST API accepts.
	listPerPage = 100
	// maxWorkflowRunPages bounds the number of workflow run pages fetched per event.
	maxWorkflowRunPages = 5
	// maxConcurrentRepos bounds parallel repository workers to stay below GitHub secondary rate limits.
	maxConcurrentRepos = 8
)

// Task - struct of a task
type Task struct {
	owner                              string
	repo                               string
	dry                                bool
	shouldRestartFailed                bool
	shouldReactivateSuspendedWorkflows bool
	verbose                            bool
	last                               string
}

func reactivateSuspendedWorkflows(ctx context.Context, client *github.Client, task Task) error {
	opt := &github.ListOptions{PerPage: listPerPage}
	for {
		workflows, resp, err := client.Actions.ListWorkflows(ctx, task.owner, task.repo, opt)
		if err != nil {
			return err
		}
		for _, workflow := range workflows.Workflows {
			if workflow.GetState() != "disabled_inactivity" {
				continue
			}
			if task.dry {
				if task.verbose {
					log.Printf("[dry] %s/%s - %q would be reactivated", task.owner, task.repo, workflow.GetName())
				}
				continue
			}
			if _, err := client.Actions.EnableWorkflowByID(ctx, task.owner, task.repo, workflow.GetID()); err != nil {
				log.Println(err.Error())
				continue
			}
			if task.verbose {
				PrintReactivateWorkflowsStatus(task, workflow)
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return nil
}

func getWorkflowRuns(ctx context.Context, client *github.Client, task Task) ([]*github.WorkflowRun, error) {
	daysNumber, err := parseDaysNumberFlag(task.last)
	if err != nil {
		return nil, err
	}
	created := ">=" + time.Now().AddDate(0, 0, -daysNumber).Format("2006-01-02")
	var runs []*github.WorkflowRun
	for _, event := range []string{"push", "schedule", "workflow_dispatch"} {
		opt := &github.ListWorkflowRunsOptions{
			Event:       event,
			Created:     created,
			ListOptions: github.ListOptions{PerPage: listPerPage},
		}
		for range maxWorkflowRunPages {
			workflowRuns, resp, err := client.Actions.ListRepositoryWorkflowRuns(ctx, task.owner, task.repo, opt)
			if err != nil {
				return nil, err
			}
			runs = append(runs, workflowRuns.WorkflowRuns...)
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
	}
	return runs, nil
}

func restartFailedRuns(ctx context.Context, client *github.Client, task Task, runs []*github.WorkflowRun) {
	for _, run := range runs {
		if run.GetConclusion() != "failure" {
			continue
		}
		if task.dry {
			log.Printf("[dry] %s/%s - run %q (%d) would be restarted", task.owner, task.repo, run.GetName(), run.GetID())
			continue
		}
		if _, err := client.Actions.RerunWorkflowByID(ctx, task.owner, task.repo, run.GetID()); err != nil {
			log.Printf("%s/%s: %v", task.owner, task.repo, err)
		}
	}
}

func worker(ctx context.Context, client *github.Client, task Task, wg *sync.WaitGroup, semaphore chan struct{}) {
	defer wg.Done()
	semaphore <- struct{}{}
	defer func() { <-semaphore }()

	if task.shouldReactivateSuspendedWorkflows {
		if err := reactivateSuspendedWorkflows(ctx, client, task); err != nil {
			log.Printf("%s/%s: reactivating workflows: %v", task.owner, task.repo, err)
		}
	}
	runs, err := getWorkflowRuns(ctx, client, task)
	if err != nil {
		log.Printf("%s/%s: %v", task.owner, task.repo, err)
		return
	}
	filteredRuns, err := ProcessingWorkflowRuns(task, runs)
	if err != nil {
		log.Printf("%s/%s: %v", task.owner, task.repo, err)
		return
	}
	if len(filteredRuns) == 0 {
		return
	}
	if task.verbose {
		PrintRunnersStatus(task, filteredRuns)
	}
	if task.shouldRestartFailed {
		restartFailedRuns(ctx, client, task, filteredRuns)
	}
}

func listOrgRepositories(ctx context.Context, client *github.Client, org string) ([]*github.Repository, error) {
	var repos []*github.Repository
	opt := &github.RepositoryListByOrgOptions{ListOptions: github.ListOptions{PerPage: listPerPage}}
	for {
		pageRepos, resp, err := client.Repositories.ListByOrg(ctx, org, opt)
		if err != nil {
			return nil, err
		}
		repos = append(repos, pageRepos...)
		if resp.NextPage == 0 {
			return repos, nil
		}
		opt.Page = resp.NextPage
	}
}

func listOwnRepositories(ctx context.Context, client *github.Client) ([]*github.Repository, error) {
	var repos []*github.Repository
	opt := &github.RepositoryListByAuthenticatedUserOptions{
		Affiliation: "owner",
		ListOptions: github.ListOptions{PerPage: listPerPage},
	}
	for {
		pageRepos, resp, err := client.Repositories.ListByAuthenticatedUser(ctx, opt)
		if err != nil {
			return nil, err
		}
		repos = append(repos, pageRepos...)
		if resp.NextPage == 0 {
			return repos, nil
		}
		opt.Page = resp.NextPage
	}
}

func listOrganizations(ctx context.Context, client *github.Client) ([]*github.Organization, error) {
	var orgs []*github.Organization
	opt := &github.ListOptions{PerPage: listPerPage}
	for {
		pageOrgs, resp, err := client.Organizations.List(ctx, "", opt)
		if err != nil {
			return nil, err
		}
		orgs = append(orgs, pageOrgs...)
		if resp.NextPage == 0 {
			return orgs, nil
		}
		opt.Page = resp.NextPage
	}
}

func addTasksForLogin(ctx context.Context, client *github.Client, tasks *[]Task, userLogin, org string) error {
	var repos []*github.Repository
	var err error
	fallbackOwner := org
	if userLogin == "" {
		repos, err = listOrgRepositories(ctx, client, org)
	} else {
		fallbackOwner = userLogin
		repos, err = listOwnRepositories(ctx, client)
	}
	if err != nil {
		return err
	}
	if GetBoolArgFromContext(ctx, keySkipArchive) {
		repos = funk.Filter(repos, func(repo *github.Repository) bool {
			return !repo.GetArchived()
		}).([]*github.Repository)
	}
	dry := GetBoolArgFromContext(ctx, keyDry)
	shouldRestartFailed := GetBoolArgFromContext(ctx, keyShouldRestartFailed)
	shouldReactivateSuspendedWorkflows := GetBoolArgFromContext(ctx, keyShouldReactivateSuspendedWorkflows)
	verbose := GetBoolArgFromContext(ctx, keyVerbose)
	last := GetStringArgFromContext(ctx, keyLast)
	for _, repo := range repos {
		owner := repo.GetOwner().GetLogin()
		if owner == "" {
			owner = fallbackOwner
		}
		*tasks = append(*tasks, Task{
			owner:                              owner,
			repo:                               repo.GetName(),
			dry:                                dry,
			shouldRestartFailed:                shouldRestartFailed,
			shouldReactivateSuspendedWorkflows: shouldReactivateSuspendedWorkflows,
			verbose:                            verbose,
			last:                               last,
		})
	}
	return nil
}

// GetWorkflowsStatus - get workflows status
func GetWorkflowsStatus(ctx context.Context, client *github.Client, userLogin string) {
	tasks := []Task{}
	if err := addTasksForLogin(ctx, client, &tasks, userLogin, ""); err != nil {
		log.Printf("%s: %v", userLogin, err)
	}
	orgs, err := listOrganizations(ctx, client)
	if err != nil {
		log.Printf("listing organizations: %v", err)
	}
	for _, org := range orgs {
		if err := addTasksForLogin(ctx, client, &tasks, "", org.GetLogin()); err != nil {
			log.Printf("%s: %v", org.GetLogin(), err)
		}
	}
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrentRepos)
	for _, task := range tasks {
		wg.Add(1)
		go worker(ctx, client, task, &wg, semaphore)
	}
	wg.Wait()
}
