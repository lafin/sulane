package main

import (
	"fmt"

	"github.com/google/go-github/v38/github"
	"github.com/thoas/go-funk"
)

var (
	colorReset = "\033[0m"
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorWhite = "\033[37m"
	colorGray  = "\033[90m"
)

var (
	emptyCircle = "\u25ef"
	fillCircle  = "\u25cf"
)

// PrintStatus - print status of runners
func PrintStatus(task Task, runs []*github.WorkflowRun) {
	fmt.Printf("%s/%s\n", task.owner, task.repo)
	for _, run := range runs {
		symbol := fillCircle
		if run.GetConclusion() == "success" {
			fmt.Print(colorGreen)
		} else if run.GetConclusion() == "failure" {
			fmt.Print(colorRed)
		} else if funk.ContainsString([]string{"in_progress", "queued"}, run.GetStatus()) {
			symbol = emptyCircle
			fmt.Print(colorGray)
		} else {
			fmt.Print(colorWhite)
		}
		fmt.Printf("  %s %s %s %s %s", symbol, run.GetName(), run.GetStatus(), run.GetConclusion(), run.GetCreatedAt())
		fmt.Println(colorReset)
	}
}
