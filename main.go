package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gopkg.in/yaml.v3"
)

// WorkflowsConfig represents the workflows section of the YAML file with default values
type WorkflowsConfig struct {
	Active                             bool   `yaml:"active"`
	ShouldRestartFailed                bool   `yaml:"should_restart_failed"`
	ShouldReactivateSuspendedWorkflows bool   `yaml:"should_reactivate_suspended_workflows"`
	Last                               string `yaml:"last"`
	SkipArchive                        bool   `yaml:"skip_archive"`
	DoMergeOnePrPerDayIfNoActionToday  bool   `yaml:"do_merge_one_pr_per_day_if_no_action_today"`
}

// SearchConfig represents the searches section of the YAML file with default values
type SearchConfig struct {
	Active                        bool   `yaml:"active"`
	Query                         string `yaml:"query"`
	ShouldAutoApproveIfReviewedBy string `yaml:"should_auto_approve_if_reviewed_by"`
	ShouldAutoApproveIfCreatedBy  string `yaml:"should_auto_approve_if_created_by"`
}

// Config represents the structure of your YAML file with nested workflows
type Config struct {
	GithubLogin    string           `yaml:"github_login"`
	GithubToken    string           `yaml:"github_token"`
	TelegramToken  string           `yaml:"telegram_token"`
	TelegramChatID string           `yaml:"telegram_chat_id"`
	Workflows      *WorkflowsConfig `yaml:"workflows"`
	Searches       *[]SearchConfig  `yaml:"searches"`
}

// MergeConfig merges the default config with the user config
func MergeConfig(config, defaultConfig *Config) *Config {
	if config.Workflows != nil && config.Workflows.Active {
		if !config.Workflows.ShouldRestartFailed {
			config.Workflows.ShouldRestartFailed = defaultConfig.Workflows.ShouldRestartFailed
		}
		if !config.Workflows.ShouldReactivateSuspendedWorkflows {
			config.Workflows.ShouldReactivateSuspendedWorkflows = defaultConfig.Workflows.ShouldReactivateSuspendedWorkflows
		}
		if config.Workflows.Last == "" {
			config.Workflows.Last = defaultConfig.Workflows.Last
		}
		if !config.Workflows.SkipArchive {
			config.Workflows.SkipArchive = defaultConfig.Workflows.SkipArchive
		}
		if !config.Workflows.DoMergeOnePrPerDayIfNoActionToday {
			config.Workflows.DoMergeOnePrPerDayIfNoActionToday = defaultConfig.Workflows.DoMergeOnePrPerDayIfNoActionToday
		}
	}
	return config
}

// ReadConfig reads the YAML config file and unmarshals it into the Config struct
func ReadConfig(filename string) (*Config, error) {
	// Set default values
	defaultConfig := &Config{
		Workflows: &WorkflowsConfig{
			ShouldRestartFailed:                false,
			ShouldReactivateSuspendedWorkflows: true,
			Last:                               "30d",
			SkipArchive:                        true,
			DoMergeOnePrPerDayIfNoActionToday:  false,
		},
	}

	file, err := os.Open(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	config := &Config{}
	if err := decoder.Decode(config); err != nil {
		return nil, err
	}

	return MergeConfig(config, defaultConfig), nil
}

func main() {
	verboseValue := flag.Bool("verbose", true, "verbose mode")
	dryValue := flag.Bool("dry", false, "dry run mode")
	configFile := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	config, err := ReadConfig(*configFile)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	if config.GithubLogin == "" || config.GithubToken == "" {
		log.Println("You must specify a GitHub login and a GitHub token in the config file.")
		return
	}

	ctx := context.Background()
	client := GetClient(ctx, config.GithubToken)

	if config.TelegramToken != "" && config.TelegramChatID != "" {
		bot, err := tgbotapi.NewBotAPI(config.TelegramToken)
		if err != nil {
			log.Panic("telegram api", err)
		}
		bot.Debug = true
		ctx = AddAnyArgToContext(ctx, "bot", bot)
		ctx = AddStringArgToContext(ctx, "chat_id", config.TelegramChatID)
	}

	if config.Workflows != nil {
		if !config.Workflows.Active {
			return
		}
		ctx = AddBoolArgToContext(ctx, "verbose", *verboseValue)
		ctx = AddBoolArgToContext(ctx, "dry", *dryValue)
		ctx = AddBoolArgToContext(ctx, "shouldRestartFailed", config.Workflows.ShouldRestartFailed)
		ctx = AddBoolArgToContext(ctx, "shouldReactivateSuspendedWorkflows", config.Workflows.ShouldReactivateSuspendedWorkflows)
		ctx = AddStringArgToContext(ctx, "last", config.Workflows.Last)
		ctx = AddBoolArgToContext(ctx, "skipArchive", config.Workflows.SkipArchive)

		if config.Workflows.DoMergeOnePrPerDayIfNoActionToday {
			DoMergeOnePrPerDayIfNoActionToday(ctx, client, config.GithubLogin)
		}
		GetWorkflowsStatus(ctx, client, config.GithubLogin)
	}

	if config.Searches != nil {
		for _, search := range *config.Searches {
			if !search.Active {
				return
			}
			ctx = AddBoolArgToContext(ctx, "verbose", *verboseValue)
			ctx = AddBoolArgToContext(ctx, "dry", *dryValue)
			ctx = AddStringArgToContext(ctx, "query", search.Query)
			ctx = AddStringArgToContext(ctx, "shouldAutoApproveIfReviewedBy", search.ShouldAutoApproveIfReviewedBy)
			ctx = AddStringArgToContext(ctx, "shouldAutoApproveIfCreatedBy", search.ShouldAutoApproveIfCreatedBy)
			AutoApprovePullRequests(ctx, client)
		}
	}
}
