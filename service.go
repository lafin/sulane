package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/google/go-github/v62/github"
	"github.com/thoas/go-funk"
)

var (
	daysNumberFlagPattern = regexp.MustCompile(`^(\d+)d$`)
	mainBranches          = []string{"master", "main"}
)

// parseDaysNumberFlag - parses the "last" flag, expecting a positive number of days like "30d"
func parseDaysNumberFlag(value string) (int, error) {
	match := daysNumberFlagPattern.FindStringSubmatch(value)
	if match == nil {
		return 0, fmt.Errorf(`the "last" flag %q must be a positive number of days followed by "d", for example "30d"`, value)
	}
	daysNumber, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf(`the "last" flag %q has an unparsable number of days: %w`, value, err)
	}
	if daysNumber <= 0 {
		return 0, fmt.Errorf(`the "last" flag %q must be greater than zero days`, value)
	}
	return daysNumber, nil
}

// ProcessingWorkflowRuns - filtering workflow runners from github by criteries
func ProcessingWorkflowRuns(task Task, runs []*github.WorkflowRun) ([]*github.WorkflowRun, error) {
	daysNumber, err := parseDaysNumberFlag(task.last)
	if err != nil {
		return nil, err
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].GetCreatedAt().Time.After(runs[j].GetCreatedAt().Time)
	})
	var foundRuns []string
	startFromDate := time.Now().Add(-(1 * time.Duration(daysNumber) * 24 * time.Hour))
	filteredRuns := funk.Filter(runs, func(run *github.WorkflowRun) bool {
		if startFromDate.After(run.GetCreatedAt().Time) {
			return false
		}
		if !funk.ContainsString(mainBranches, run.GetHeadBranch()) {
			return false
		}
		if funk.ContainsString(foundRuns, run.GetName()) {
			return false
		}
		foundRuns = append(foundRuns, run.GetName())
		return true
	}).([]*github.WorkflowRun)
	return filteredRuns, nil
}
