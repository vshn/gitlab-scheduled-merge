package task

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/vshn/gitlab-scheduled-merge/client"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/multierr"
	"gopkg.in/yaml.v3"
)

type TaskConfig struct {
	MergeRequestScheduledLabel string
	ConfigFilePath             string
}

type Task struct {
	config TaskConfig
	client client.GitlabClient
	clock  Clock
}

type RepositoryConfig struct {
	MergeWindows []MergeWindow `yaml:"mergeWindows"`
}
type MergeWindow struct {
	Schedule MergeSchedule `yaml:"schedule"`
	MaxDelay time.Duration `yaml:"maxDelay"`
}
type MergeSchedule struct {
	Cron     string `yaml:"cron"`
	IsoWeek  string `yaml:"isoWeek"`
	Location string `yaml:"location"`
}

type Clock interface {
	Now() time.Time
}

type realClock struct{}

const (
	COMMENT_MERGE_FAILED            = "Failed to merge"
	COMMENT_MERGE_SKIPPED           = "Not merging automatically"
	COMMENT_MERGE_SCHEDULING_FAILED = "Failed to schedule merge"
	COMMENT_MERGE_SCHEDULED         = "Merge scheduled"
)

func (realClock) Now() time.Time {
	return time.Now()
}

func NewTask(client client.GitlabClient, config TaskConfig) Task {
	return Task{
		config: config,
		client: client,
		clock:  realClock{},
	}
}

func NewTaskWithClock(client client.GitlabClient, config TaskConfig, clock Clock) Task {
	return Task{
		config: config,
		client: client,
		clock:  clock,
	}
}

func (t Task) Run() error {
	log.Println("Running task...")
	mrs, err := t.client.ListMrsWithLabel(t.config.MergeRequestScheduledLabel)
	if err != nil {
		return fmt.Errorf("failed to list MRs: %w", err)
	}

	log.Printf("Processing %d MRs with label...\n", len(mrs))
	errs := make([]error, 0)
	for _, mr := range mrs {
		err := t.processMR(mr)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return multierr.Combine(errs...)
}

func (t Task) processMR(mr *gitlab.MergeRequest) error {
	file, err := t.client.GetConfigFileForMR(mr, t.config.ConfigFilePath)

	if err != nil {
		return t.client.Comment(mr, COMMENT_MERGE_SCHEDULING_FAILED, "Missing config file.")
	}

	config := RepositoryConfig{}
	err = yaml.Unmarshal(*file, &config)

	if err != nil {
		return t.client.Comment(mr, COMMENT_MERGE_SCHEDULING_FAILED, fmt.Sprintf("Error while parsing config file.\n\n%s", err.Error()))
	}

	now := t.clock.Now()
	var earliestMergeWindow *MergeWindow = nil
	earliestMergeWindowTime := now.Add(1000000 * time.Hour)
	for _, w := range config.MergeWindows {
		nextActiveStartTime, err := w.getNextActiveWindowStartTime(now)
		if err != nil {
			return t.client.Comment(mr, COMMENT_MERGE_SCHEDULING_FAILED, fmt.Sprintf("Error while parsing merge windows.\n\n%s", err.Error()))
		}
		if nextActiveStartTime.Before(now) {
			return t.mergeMR(mr)
		}
		if nextActiveStartTime.Before(earliestMergeWindowTime) {
			earliestMergeWindow = &w
			earliestMergeWindowTime = nextActiveStartTime
		}
	}
	nextActiveEndTime := earliestMergeWindowTime.Add(earliestMergeWindow.MaxDelay)

	msg := fmt.Sprintf(
		"This MR will be merged between %s and %s.",
		earliestMergeWindowTime.Format(time.UnixDate),
		nextActiveEndTime.Format(time.UnixDate),
	)

	if !client.IsMergeable(mr) {
		msg = fmt.Sprintf(
			"%s\n\nWarning: This merge request is currently not mergeable. Current status: %s",
			msg,
			mr.DetailedMergeStatus,
		)

	}
	return t.client.Comment(mr, COMMENT_MERGE_SCHEDULED, msg)
}

func (t Task) mergeMR(mr *gitlab.MergeRequest) error {
	// We need to recheck MRs - we might in the interim have merged other things that led to conflicts
	rmr, err := t.client.RefreshMr(mr)
	if err != nil {
		return t.client.Comment(mr, COMMENT_MERGE_FAILED, fmt.Sprintf("Error while refreshing merge request data.\n\n%s", err.Error()))
	}

	if !client.IsMergeable(rmr) {
		return t.client.Comment(mr, COMMENT_MERGE_SKIPPED, fmt.Sprintf("MR is not mergeable. Current status: %s", rmr.DetailedMergeStatus))
	}

	err = t.client.MergeMr(rmr)
	if err != nil {
		return t.client.Comment(mr, COMMENT_MERGE_FAILED, fmt.Sprintf("Error while merging.\n\n%s", err.Error()))
	}

	return nil
}

// getNextActiveWindowStartTime returns the start time of the next active merge window
// including potential windows that have already started but are still active at timestamp `t`
func (w MergeWindow) getNextActiveWindowStartTime(t time.Time) (time.Time, error) {
	location := time.Local
	if w.Schedule.Location != "" {
		l, err := time.LoadLocation(w.Schedule.Location)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to load location for merge window: %w", err)
		}
		location = l
	}
	now := t.In(location)
	earliestTime := now.Add(-w.MaxDelay)

	sched, err := cron.ParseStandard(w.Schedule.Cron)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse cron schedule: %w", err)
	}

	nextRun := sched.Next(earliestTime)
	for i := 0; i < 1000; i++ {
		isoWeekOK, err := w.checkIsoWeek(nextRun)
		if err != nil {
			return time.Time{}, err
		}
		if isoWeekOK {
			return nextRun, nil
		}
		nextRun = sched.Next(nextRun)
	}
	return time.Time{}, fmt.Errorf("could not find next run, max time: %s", nextRun)
}

// checkIsoWeek checks if the given time is in the given iso week.
// The iso week can be one of the following:
// - "": every iso week
// - "@even": every even iso week
// - "@odd": every odd iso week
// - "<N>": every iso week N
func (w MergeWindow) checkIsoWeek(t time.Time) (bool, error) {
	_, iw := t.ISOWeek()
	switch w.Schedule.IsoWeek {
	case "":
		return true, nil
	case "@even":
		return iw%2 == 0, nil
	case "@odd":
		return iw%2 == 1, nil
	}
	nw, err := strconv.ParseInt(w.Schedule.IsoWeek, 10, 64)
	if err == nil {
		return nw == int64(iw), nil
	}

	return false, fmt.Errorf("unknown iso week: %s", w.Schedule.IsoWeek)
}
