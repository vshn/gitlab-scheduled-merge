package main

import (
	"fmt"
	"log"
	"os"

	"github.com/vshn/gitlab-scheduled-merge/client"
	"github.com/vshn/gitlab-scheduled-merge/task"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

var (
	version = "snapshot"
	commit  = "unknown"
	date    = "unknown"
)

type RepositoryConfig struct {
	MergeWindows []MergeWindow `yaml:"mergeWindows"`
}
type MergeWindow struct {
	Cron     string `yaml:"cron"`
	MaxDelay string `yaml:"maxDelay"`
}

func main() {
	cmd := &cobra.Command{
		Use: "gitlab-schedule-merge",
	}

	gitlabToken := cmd.Flags().StringP("gitlab-token", "t", "", "Token with which to authenticate with GitLab")
	gitlabBaseUrl := cmd.Flags().String("gitlab-base-url", "https://gitlab.com/api/v4", "Base URL of GitLab API to use")
	scheduledLabel := cmd.Flags().String("scheduled-label", "scheduled", "Name of the label which indicates a MR should be scheduled")
	configFilePath := cmd.Flags().String("config-file-path", ".merge-schedule.yml", "Path of the config file in the repo which is used to configure merge windows")
	taskSchedule := cmd.Flags().String("task-schedule", "@every 15m", "Cron schedule for how frequently to process merge requests")

	cmd.Run = func(*cobra.Command, []string) {
		gitlabConfig := client.GitlabConfig{
			AccessToken: *gitlabToken,
			BaseURL:     *gitlabBaseUrl,
		}
		gitlabClient, err := client.NewGitlabClient(gitlabConfig)
		if err != nil {
			log.Fatalf("GitLab client error: %s", err.Error())
		}

		task, err := setupCronTask(gitlabClient, *taskSchedule, *scheduledLabel, *configFilePath)
		if err != nil {
			log.Fatalf("Error setting up cron task: %s", err.Error())
		}

		log.Println("Starting task...")
		task.Run()

	}

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setupCronTask(
	client *client.GitlabClient,
	crontab string,
	scheduledLabel string,
	configFilePath string,
) (*cron.Cron, error) {
	config := task.TaskConfig{
		MergeRequestScheduledLabel: scheduledLabel,
		ConfigFilePath:             configFilePath,
	}
	periodicTask := task.NewTask(client, config)

	c := cron.New()
	_, err := c.AddFunc(crontab, func() {
		err := periodicTask.Run()
		if err == nil {
			return
		}
		log.Printf("error during periodic job: %s\n", err.Error())
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}
