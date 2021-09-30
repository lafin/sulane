package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/go-github/v38/github"
	"github.com/joho/godotenv"
	"github.com/thoas/go-funk"
	"golang.org/x/oauth2"
)

// Task - struct of a task
type Task struct {
	owner                 string
	repo                  string
	shouldRestartedFailed bool
}

var (
	colorReset = "\033[0m"
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorWhite = "\033[37m"
	colorGray  = "\033[90m"
)

var (
	emptyCircle = "\u25EF"
	fillCircle  = "\u25CF"
	fillArrow   = "\u25B6"
)

func worker(ctx context.Context, client *github.Client, task Task, wg *sync.WaitGroup) {
	defer wg.Done()
	var err error
	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, task.owner, task.repo, &github.ListWorkflowRunsOptions{
		Event: "push",
	})
	if err != nil {
		log.Panic(err)
	}
	var foundRuns []string
	startFromDate := time.Now().Add(-(1 * 30 * 24 * time.Hour))
	filteredRuns := funk.Filter(runs.WorkflowRuns, func(run *github.WorkflowRun) bool {
		if startFromDate.After(run.GetCreatedAt().Time) {
			return false
		}
		if funk.ContainsString(foundRuns, run.GetName()) {
			return false
		}
		foundRuns = append(foundRuns, run.GetName())
		return funk.ContainsString([]string{"master", "main"}, run.GetHeadBranch())
	}).([]*github.WorkflowRun)
	if len(filteredRuns) == 0 {
		return
	}
	fmt.Printf("%s %s/%s\n", fillArrow, task.owner, task.repo)
	for _, run := range filteredRuns {
		isFailed := run.GetConclusion() == "failure"
		symbol := fillCircle
		if run.GetConclusion() == "success" {
			fmt.Print(colorGreen)
		} else if isFailed {
			fmt.Print(colorRed)
		} else if funk.ContainsString([]string{"in_progress", "queued"}, run.GetConclusion()) {
			symbol = emptyCircle
			fmt.Print(colorGray)
		} else {
			fmt.Print(colorWhite)
		}
		fmt.Printf("%s %s %s %s %s", symbol, run.GetName(), run.GetStatus(), run.GetConclusion(), run.GetCreatedAt())
		fmt.Println(colorReset)
		if isFailed && task.shouldRestartedFailed {
			fmt.Printf("restarted: %s/%s %s\n", task.owner, task.repo, run.GetName())
			_, err = client.Actions.RerunWorkflowByID(ctx, task.owner, task.repo, run.GetID())
			if err != nil {
				log.Panic(err)
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
	for _, repo := range repos {
		*tasks = append(*tasks, Task{
			owner:                 org,
			repo:                  repo.GetName(),
			shouldRestartedFailed: false,
		})
	}
}

func main() {
	_ = godotenv.Load()
	githubLogin := os.Getenv("GITHUB_LOGIN")
	accessToken := os.Getenv("ACCESS_TOKEN")
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	tasks := []Task{}
	addTasksForLogin(ctx, client, &tasks, githubLogin, "")
	orgs, _, err := client.Organizations.List(ctx, githubLogin, nil)
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
