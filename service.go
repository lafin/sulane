package main

import (
	"time"

	"github.com/google/go-github/v38/github"
	"github.com/thoas/go-funk"
)

// ProcessingWorkflowRuns - filtering workflow runners from github by criteries
func ProcessingWorkflowRuns(runs []*github.WorkflowRun) []*github.WorkflowRun {
	var foundRuns []string
	startFromDate := time.Now().Add(-(1 * 30 * 24 * time.Hour))
	filteredRuns := funk.Filter(runs, func(run *github.WorkflowRun) bool {
		if startFromDate.After(run.GetCreatedAt().Time) {
			return false
		}
		if funk.ContainsString(foundRuns, run.GetName()) {
			return false
		}
		foundRuns = append(foundRuns, run.GetName())
		return funk.ContainsString([]string{"master", "main"}, run.GetHeadBranch())
	}).([]*github.WorkflowRun)
	return filteredRuns
}
