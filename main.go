package main

import (
	"context"
	"flag"
	"log"
)

func main() {
	githubLoginValue := flag.String("login", "", "github login")
	accessTokenValue := flag.String("token", "", "github token")
	shouldRestartedFailedValue := flag.Bool("restart", false, "should restarted failed")
	shouldReactivateSuspendedWorkflows := flag.Bool("reactivateSuspended", true, "should reactivate a suspended workflows")
	verboseValue := flag.Bool("verbose", true, "verbose mode")
	lastValue := flag.String("last", "30d", "get the results of actions for the last days")
	skipArchiveValue := flag.Bool("skipArchive", true, "skip archived")
	doMergeOnePrPerDayIfNoActionTodayValue := flag.Bool("doMergeOnePrPerDayIfNoActionToday", false, "do merge one PR per day if no action today")
	flag.Parse()
	userLogin := *githubLoginValue
	accessToken := *accessTokenValue
	if userLogin == "" || accessToken == "" {
		log.Println("should specify a github login and a github token")
		return
	}

	ctx := context.Background()
	ctx = AddBoolArgToContext(ctx, "shouldRestartedFailed", *shouldRestartedFailedValue)
	ctx = AddBoolArgToContext(ctx, "shouldReactivateSuspendedWorkflows", *shouldReactivateSuspendedWorkflows)
	ctx = AddBoolArgToContext(ctx, "verbose", *verboseValue)
	ctx = AddStringArgToContext(ctx, "last", *lastValue)
	ctx = AddBoolArgToContext(ctx, "skipArchive", *skipArchiveValue)
	client := GetClient(ctx, accessToken)

	if *doMergeOnePrPerDayIfNoActionTodayValue {
		DoMergeOnePrPerDayIfNoActionToday(ctx, client, userLogin)
	}
	GetWorkflowsStatus(ctx, client, userLogin)
}
