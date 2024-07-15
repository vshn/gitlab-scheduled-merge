package client

import (
	"fmt"
	"strings"

	"github.com/xanzy/go-gitlab"
)

const MR_MERGE_STATUS_MERGEABLE = "mergeable"

type GitlabConfig struct {
	AccessToken string
	BaseURL     string
}

type GitlabClient interface {
	GetConfigFileForMR(mr *gitlab.MergeRequest, filePath string) (*[]byte, error)
	ListMrsWithLabel(label string) ([]*gitlab.MergeRequest, error)
	RefreshMr(mr *gitlab.MergeRequest) (*gitlab.MergeRequest, error)
	MergeMr(mr *gitlab.MergeRequest) error
	Comment(mr *gitlab.MergeRequest, title string, comment string) error
}

type gitlabClientImpl struct {
	client *gitlab.Client
	me     *gitlab.User
	config *GitlabConfig
}

func NewGitlabClient(config GitlabConfig) (GitlabClient, error) {
	git, err := gitlab.NewClient(config.AccessToken, gitlab.WithBaseURL(config.BaseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate to GitLab: %w", err)
	}
	me, _, err := git.Users.CurrentUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user information from GitLab: %w", err)
	}
	return &gitlabClientImpl{
		client: git,
		me:     me,
		config: &config,
	}, nil
}

func (g *gitlabClientImpl) GetConfigFileForMR(mr *gitlab.MergeRequest, filePath string) (*[]byte, error) {
	opts := &gitlab.GetRawFileOptions{Ref: &mr.SourceBranch}
	file, _, err := g.client.RepositoryFiles.GetRawFile(mr.ProjectID, filePath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config file: %w", err)
	}
	return &file, nil
}

func (g *gitlabClientImpl) ListMrsWithLabel(label string) ([]*gitlab.MergeRequest, error) {
	labels := gitlab.LabelOptions{label}
	opts := &gitlab.ListMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 20,
			Page:    1,
		},
		State:                  gitlab.Ptr("opened"),
		Labels:                 &labels,
		Scope:                  gitlab.Ptr("all"),
		WithMergeStatusRecheck: gitlab.Ptr(true),
	}
	var allMrs []*gitlab.MergeRequest

	for {
		mrs, resp, err := g.client.MergeRequests.ListMergeRequests(opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list MRs: %w", err)
		}
		allMrs = append(allMrs, mrs...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allMrs, nil
}

func (g *gitlabClientImpl) RefreshMr(mr *gitlab.MergeRequest) (*gitlab.MergeRequest, error) {
	opts := &gitlab.GetMergeRequestsOptions{}
	mr, _, err := g.client.MergeRequests.GetMergeRequest(mr.ProjectID, mr.IID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get MR: %w", err)
	}

	return mr, nil
}

func (g *gitlabClientImpl) MergeMr(mr *gitlab.MergeRequest) error {
	opts := &gitlab.AcceptMergeRequestOptions{ShouldRemoveSourceBranch: gitlab.Ptr(true)}
	_, _, err := g.client.MergeRequests.AcceptMergeRequest(mr.ProjectID, mr.IID, opts)
	if err != nil {
		return fmt.Errorf("failed to merge MR: %w", err)
	}
	return nil
}

func (g *gitlabClientImpl) Comment(mr *gitlab.MergeRequest, title string, comment string) error {
	full_comment := fmt.Sprintf("**%s**:  %s", title, comment)
	nopts := &gitlab.ListMergeRequestNotesOptions{}
	notes, _, err := g.client.Notes.ListMergeRequestNotes(mr.ProjectID, mr.IID, nopts)

	if err != nil {
		return fmt.Errorf("failed to get comments on MR: %w", err)
	}

	for _, n := range notes {
		if n.Author.ID == g.me.ID {
			if title == extractTitleFromComment(n.Body) {
				// Update old comment with the same title:
				opts := &gitlab.UpdateMergeRequestNoteOptions{
					Body: gitlab.Ptr(full_comment),
				}
				_, _, err = g.client.Notes.UpdateMergeRequestNote(mr.ProjectID, mr.IID, n.ID, opts)
				if err != nil {
					return fmt.Errorf("failed to update comment on MR: %w", err)
				}
				return nil
			}
			// Only check the newest own comment for title match.
			break
		}
	}

	// We don't have the possibility to update a previous comment of the same type - make a new one:
	opts := &gitlab.CreateMergeRequestNoteOptions{
		Body: gitlab.Ptr(full_comment),
	}
	_, _, err = g.client.Notes.CreateMergeRequestNote(mr.ProjectID, mr.IID, opts)
	if err != nil {
		return fmt.Errorf("failed to add comment to MR: %w", err)
	}
	return nil
}

func extractTitleFromComment(comment string) string {
	parts := strings.Split(comment, "**")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func IsMergeable(mr *gitlab.MergeRequest) bool {
	return mr.DetailedMergeStatus == MR_MERGE_STATUS_MERGEABLE
}
