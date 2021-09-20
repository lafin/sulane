package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/google/go-github/v38/github"
	"github.com/joho/godotenv"
	"github.com/thoas/go-funk"
	"golang.org/x/oauth2"
)

type task struct {
	org  string
	repo string
}

var (
	colorReset = "\033[0m"
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorWhite = "\033[37m"
)

func worker(client *github.Client, t task, wg *sync.WaitGroup) {
	defer wg.Done()
	runs, _, _ := client.Actions.ListRepositoryWorkflowRuns(context.Background(), t.org, t.repo, &github.ListWorkflowRunsOptions{
		Event: "push",
	})
	var foundRuns []string
	filteredRuns := funk.Filter(runs.WorkflowRuns, func(run *github.WorkflowRun) bool {
		if funk.ContainsString(foundRuns, run.GetName()) {
			return false
		}
		foundRuns = append(foundRuns, run.GetName())
		return funk.ContainsString([]string{"master", "main"}, run.GetHeadBranch())
	}).([]*github.WorkflowRun)
	if len(filteredRuns) == 0 {
		return
	}
	fmt.Printf("\u2b58 %s/%s\n", t.org, t.repo)
	for _, run := range filteredRuns {
		if run.GetConclusion() == "success" {
			fmt.Print(colorGreen)
		} else if run.GetConclusion() == "failure" {
			fmt.Print(colorRed)
		} else {
			fmt.Print(colorWhite)
		}
		fmt.Printf("\u2b51 %s %s %s %s", run.GetName(), run.GetStatus(), run.GetConclusion(), run.GetCreatedAt())
		fmt.Println(colorReset)
	}
}

func addTasksForLogin(ctx context.Context, client *github.Client, tasks *[]task, user, org string) {
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
		*tasks = append(*tasks, task{
			org:  org,
			repo: repo.GetName(),
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
	tasks := []task{}
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
		go worker(client, task, &wg)
	}
	wg.Wait()
}
