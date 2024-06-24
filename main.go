package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the structure of your YAML file with default values
type Config struct {
	GithubLogin                        string `yaml:"github_login"`
	AccessToken                        string `yaml:"access_token"`
	ShouldRestartFailed                bool   `yaml:"should_restart_failed"`
	ShouldReactivateSuspendedWorkflows bool   `yaml:"should_reactivate_suspended_workflows"`
	Last                               string `yaml:"last"`
	SkipArchive                        bool   `yaml:"skip_archive"`
	DoMergeOnePrPerDayIfNoActionToday  bool   `yaml:"do_merge_one_pr_per_day_if_no_action_today"`
}

// ReadConfig reads the YAML config file and unmarshals it into the Config struct
func ReadConfig(filename string) (*Config, error) {

	// Set default values
	config := &Config{
		ShouldRestartFailed:                false,
		ShouldReactivateSuspendedWorkflows: true,
		Last:                               "30d",
		SkipArchive:                        true,
		DoMergeOnePrPerDayIfNoActionToday:  false,
	}

	file, err := os.Open(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, err
	}

	return config, nil
}

func main() {
	verboseValue := flag.Bool("verbose", true, "verbose mode")
	configFile := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	config, err := ReadConfig(*configFile)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	if config.GithubLogin == "" || config.AccessToken == "" {
		log.Println("You must specify a GitHub login and a GitHub token in the config file.")
		return
	}

	ctx := context.Background()
	ctx = AddBoolArgToContext(ctx, "shouldRestartFailed", config.ShouldRestartFailed)
	ctx = AddBoolArgToContext(ctx, "shouldReactivateSuspendedWorkflows", config.ShouldReactivateSuspendedWorkflows)
	ctx = AddBoolArgToContext(ctx, "verbose", *verboseValue)
	ctx = AddStringArgToContext(ctx, "last", config.Last)
	ctx = AddBoolArgToContext(ctx, "skipArchive", config.SkipArchive)
	client := GetClient(ctx, config.AccessToken)

	if config.DoMergeOnePrPerDayIfNoActionToday {
		DoMergeOnePrPerDayIfNoActionToday(ctx, client, config.GithubLogin)
	}
	GetWorkflowsStatus(ctx, client, config.GithubLogin)
}
