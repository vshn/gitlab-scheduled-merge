package task_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	mock_client "github.com/vshn/gitlab-scheduled-merge/client/mock"
	"github.com/vshn/gitlab-scheduled-merge/task"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/mock/gomock"
)

type testClock struct{}

func (testClock) Now() time.Time {
	time, _ := time.Parse(time.RFC3339, "2024-06-27T10:30:00+02:00")
	return time
}

type hasSubstr struct {
	values []string
}

func (m hasSubstr) Matches(arg interface{}) bool {
	sarg := arg.(string)
	for _, s := range m.values {
		if !strings.Contains(sarg, s) {
			return false
		}
	}
	return true
}
func (m hasSubstr) String() string {
	return strings.Join(m.values, ", ")
}

func Test_RunTask(t *testing.T) {
	mctrl := gomock.NewController(t)
	mock := mock_client.NewMockGitlabClient(mctrl)
	config := task.TaskConfig{
		MergeRequestScheduledLabel: "scheduled",
		ConfigFilePath:             ".config-file.yml",
	}

	subject := task.NewTaskWithClock(mock, config, testClock{})

	mrs := mrList()
	gomock.InOrder(
		mock.EXPECT().ListMrsWithLabel(gomock.Any()).Return(mrs, nil),
		mock.EXPECT().GetConfigFileForMR(mrs[0], ".config-file.yml").Return(activeMergeWindow(), nil),
		mock.EXPECT().RefreshMr(mrs[0]).Return(mrs[0], nil),
		mock.EXPECT().MergeMr(mrs[0]).Return(nil),
		mock.EXPECT().GetConfigFileForMR(mrs[1], ".config-file.yml").Return(inactiveMergeWindow(), nil),
		mock.EXPECT().Comment(mrs[1], gomock.Not(hasSubstr{[]string{"Failed"}}), gomock.Any()).Return(nil),
	)

	err := subject.Run()

	require.NoError(t, err)

}

func Test_RunTask_TimeZoneShenanigans(t *testing.T) {
	mctrl := gomock.NewController(t)
	mock := mock_client.NewMockGitlabClient(mctrl)
	config := task.TaskConfig{
		MergeRequestScheduledLabel: "scheduled",
		ConfigFilePath:             ".config-file.yml",
	}

	subject := task.NewTaskWithClock(mock, config, testClock{})

	mrs := mrList()
	gomock.InOrder(
		mock.EXPECT().ListMrsWithLabel(gomock.Any()).Return(mrs, nil),
		mock.EXPECT().GetConfigFileForMR(mrs[0], ".config-file.yml").Return(inactiveMergeWindowWithLocation(), nil),
		mock.EXPECT().Comment(mrs[0], gomock.Not(hasSubstr{[]string{"Failed"}}), gomock.Any()).Return(nil),
		mock.EXPECT().GetConfigFileForMR(mrs[1], ".config-file.yml").Return(inactiveMergeWindowWithWeek(), nil),
		mock.EXPECT().Comment(mrs[1], gomock.Not(hasSubstr{[]string{"Failed"}}), gomock.Any()).Return(nil),
	)

	err := subject.Run()

	require.NoError(t, err)

}

func Test_RunTask_WithError(t *testing.T) {
	mctrl := gomock.NewController(t)
	mock := mock_client.NewMockGitlabClient(mctrl)
	config := task.TaskConfig{
		MergeRequestScheduledLabel: "scheduled",
		ConfigFilePath:             ".config-file.yml",
	}

	subject := task.NewTaskWithClock(mock, config, testClock{})

	mrs := mrList()
	gomock.InOrder(
		mock.EXPECT().ListMrsWithLabel(gomock.Any()).Return(mrs, nil),
		mock.EXPECT().GetConfigFileForMR(mrs[0], ".config-file.yml").Return(nil, errors.New("ERROR FAIL HALP")),
		mock.EXPECT().Comment(mrs[0], hasSubstr{[]string{"Failed"}}, gomock.Any()).Return(nil),
		mock.EXPECT().GetConfigFileForMR(mrs[1], ".config-file.yml").Return(inactiveMergeWindowWithWeek(), nil),
		mock.EXPECT().Comment(mrs[1], gomock.Not(hasSubstr{[]string{"Failed"}}), gomock.Any()).Return(errors.New("COMMENT FAILED")),
	)

	err := subject.Run()

	require.Error(t, err)

}

func mrList() []*gitlab.MergeRequest {
	f := &gitlab.MergeRequest{
		IID:                 1,
		DetailedMergeStatus: "mergeable",
	}
	g := &gitlab.MergeRequest{
		IID: 2,
	}
	return []*gitlab.MergeRequest{f, g}
}

func activeMergeWindow() *[]byte {
	yaml := []byte(`
mergeWindows:
- schedule:
    cron: '0 10 * * *'
    location: 'Europe/Zurich'
  maxDelay: '1h'`)
	return &yaml
}

func inactiveMergeWindowWithLocation() *[]byte {
	yaml := []byte(`
mergeWindows:
- schedule:
    cron: '0 10 * * *'
    location: 'America/New_York'
  maxDelay: '1h'`)
	return &yaml
}

func inactiveMergeWindowWithWeek() *[]byte {
	yaml := []byte(`
mergeWindows:
- schedule:
    cron: '0 10 * * *'
    isoWeek: '@odd'
    location: 'Europe/Zurich'
  maxDelay: '1h'`)
	return &yaml
}

func inactiveMergeWindow() *[]byte {
	yaml := []byte(`
mergeWindows:
- schedule:
    cron: '0 20 * * *'
    location: 'Europe/Zurich'
  maxDelay: '1h'`)
	return &yaml
}
