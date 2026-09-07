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
	ShouldRestartFailed                *bool  `yaml:"should_restart_failed"`
	ShouldReactivateSuspendedWorkflows *bool  `yaml:"should_reactivate_suspended_workflows"`
	Last                               string `yaml:"last"`
	SkipArchive                        *bool  `yaml:"skip_archive"`
	DoMergeOnePrPerDayIfNoActionToday  *bool  `yaml:"do_merge_one_pr_per_day_if_no_action_today"`
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

// boolPtr returns a pointer to value, used to express config defaults.
func boolPtr(value bool) *bool {
	return &value
}

// boolValue dereferences an optional config flag; an unset flag is false.
func boolValue(value *bool) bool {
	return value != nil && *value
}

// MergeConfig merges the default config with the user config
func MergeConfig(config, defaultConfig *Config) *Config {
	if config.Workflows != nil && defaultConfig != nil && defaultConfig.Workflows != nil {
		if config.Workflows.ShouldRestartFailed == nil {
			config.Workflows.ShouldRestartFailed = defaultConfig.Workflows.ShouldRestartFailed
		}
		if config.Workflows.ShouldReactivateSuspendedWorkflows == nil {
			config.Workflows.ShouldReactivateSuspendedWorkflows = defaultConfig.Workflows.ShouldReactivateSuspendedWorkflows
		}
		if config.Workflows.Last == "" {
			config.Workflows.Last = defaultConfig.Workflows.Last
		}
		if config.Workflows.SkipArchive == nil {
			config.Workflows.SkipArchive = defaultConfig.Workflows.SkipArchive
		}
		if config.Workflows.DoMergeOnePrPerDayIfNoActionToday == nil {
			config.Workflows.DoMergeOnePrPerDayIfNoActionToday = defaultConfig.Workflows.DoMergeOnePrPerDayIfNoActionToday
		}
	}
	return config
}

// defaultConfig returns the config values applied to every key a user config omits.
func defaultConfig() *Config {
	return &Config{
		Workflows: &WorkflowsConfig{
			ShouldRestartFailed:                boolPtr(false),
			ShouldReactivateSuspendedWorkflows: boolPtr(true),
			Last:                               "30d",
			SkipArchive:                        boolPtr(true),
			DoMergeOnePrPerDayIfNoActionToday:  boolPtr(false),
		},
	}
}

// ReadConfig reads the YAML config file and unmarshals it into the Config struct
func ReadConfig(filename string) (*Config, error) {
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

	return MergeConfig(config, defaultConfig()), nil
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

	workflowsActive := config.Workflows != nil && config.Workflows.Active
	if workflowsActive {
		if _, err := parseDaysNumberFlag(config.Workflows.Last); err != nil {
			log.Fatalf("Error in the workflows config: %v", err)
		}
	}

	baseCtx := context.Background()
	client := GetClient(baseCtx, config.GithubToken)

	if config.TelegramToken != "" && config.TelegramChatID != "" {
		bot, err := tgbotapi.NewBotAPI(config.TelegramToken)
		if err != nil {
			// A Telegram misconfiguration must not abort the GitHub work.
			log.Printf("telegram: api: %s", redactTelegramError(err, config.TelegramToken))
		} else {
			baseCtx = AddAnyArgToContext(baseCtx, keyBot, bot)
			baseCtx = AddStringArgToContext(baseCtx, keyChatID, config.TelegramChatID)
		}
	}

	baseCtx = AddBoolArgToContext(baseCtx, keyVerbose, *verboseValue)
	baseCtx = AddBoolArgToContext(baseCtx, keyDry, *dryValue)

	if workflowsActive {
		workflowsCtx := AddBoolArgToContext(baseCtx, keyShouldRestartFailed, boolValue(config.Workflows.ShouldRestartFailed))
		workflowsCtx = AddBoolArgToContext(workflowsCtx, keyShouldReactivateSuspendedWorkflows, boolValue(config.Workflows.ShouldReactivateSuspendedWorkflows))
		workflowsCtx = AddStringArgToContext(workflowsCtx, keyLast, config.Workflows.Last)
		workflowsCtx = AddBoolArgToContext(workflowsCtx, keySkipArchive, boolValue(config.Workflows.SkipArchive))

		if boolValue(config.Workflows.DoMergeOnePrPerDayIfNoActionToday) {
			DoMergeOnePrPerDayIfNoActionToday(workflowsCtx, client, config.GithubLogin)
		}
		GetWorkflowsStatus(workflowsCtx, client, config.GithubLogin)
	}

	if config.Searches != nil {
		for _, search := range *config.Searches {
			if !search.Active {
				continue
			}
			searchCtx := AddStringArgToContext(baseCtx, keyQuery, search.Query)
			searchCtx = AddStringArgToContext(searchCtx, keyShouldAutoApproveIfReviewedBy, search.ShouldAutoApproveIfReviewedBy)
			searchCtx = AddStringArgToContext(searchCtx, keyShouldAutoApproveIfCreatedBy, search.ShouldAutoApproveIfCreatedBy)
			AutoApprovePullRequests(searchCtx, client)
		}
	}
}
