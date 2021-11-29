package main

import (
	"log"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/google/go-github/v38/github"
	"github.com/thoas/go-funk"
)

func parseDaysNumberFlag(value string) int {
	re := regexp.MustCompile(`(^\d+)?d$`)
	match := re.FindStringSubmatch(value)
	daysNumber, err := strconv.Atoi(match[1])
	if err != nil {
		log.Panicf(`An error parsing the "last" flag: %s`, err)
	}
	return daysNumber
}

// ProcessingWorkflowRuns - filtering workflow runners from github by criteries
func ProcessingWorkflowRuns(task Task, runs []*github.WorkflowRun) []*github.WorkflowRun {
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].GetCreatedAt().Time.After(runs[j].GetCreatedAt().Time)
	})
	var foundRuns []string
	daysNumber := parseDaysNumberFlag(task.last)
	startFromDate := time.Now().Add(-(1 * time.Duration(daysNumber) * 24 * time.Hour))
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
