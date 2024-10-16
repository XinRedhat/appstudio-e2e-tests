package git

import (
	"fmt"
	"strings"

	"github.com/konflux-ci/e2e-tests/pkg/clients/github"
)

type GitHubClient struct {
	*github.Github
}

func NewGitHubClient(gh *github.Github) *GitHubClient {
	return &GitHubClient{gh}
}

func (g *GitHubClient) CreateBranch(repository, baseBranchName, revision, branchName string) error {
	return g.CreateRef(repository, baseBranchName, revision, branchName)
}

func (g *GitHubClient) DeleteBranch(repository, branchName string) error {
	return g.DeleteRef(repository, branchName)
}

func (g *GitHubClient) BranchExists(repository, branchName string) (bool, error) {
	return g.ExistsRef(repository, branchName)
}

func (g *GitHubClient) ListPullRequests(repository string) ([]*PullRequest, error) {
	prs, err := g.Github.ListPullRequests(repository)
	if err != nil {
		return nil, err
	}
	var pullRequests []*PullRequest
	for _, pr := range prs {
		pullRequests = append(pullRequests, &PullRequest{
			Number:       pr.GetNumber(),
			SourceBranch: pr.Head.GetRef(),
			TargetBranch: pr.Base.GetRef(),
			HeadSHA:      pr.Head.GetSHA(),
		})
	}
	return pullRequests, nil
}

func (g *GitHubClient) CreateFile(repository, pathToFile, content, branchName string) error {
	_, err := g.Github.CreateFile(repository, pathToFile, content, branchName)
	return err
}

func (g *GitHubClient) GetFileContent(repository, pathToFile, branchName string) (string, error) {
	contents, err := g.Github.GetFile(repository, pathToFile, branchName)
	if err != nil {
		return "", err
	}
	content, err := contents.GetContent()
	if err != nil {
		return "", err
	}
	return content, nil
}

func (g *GitHubClient) MergePullRequest(repository string, prNumber int) (*PullRequest, error) {
	mergeResult, err := g.Github.MergePullRequest(repository, prNumber)
	if err != nil {
		return nil, err
	}
	return &PullRequest{
		Number:         prNumber,
		MergeCommitSHA: mergeResult.GetSHA(),
	}, nil
}

func (g *GitHubClient) CreatePullRequest(repository, title, body, head, base string) (*PullRequest, error) {
	pr, err := g.Github.CreatePullRequest(repository, title, body, head, base)
	if err != nil {
		return nil, err
	}
	return &PullRequest{
		Number:       pr.GetNumber(),
		SourceBranch: pr.Head.GetRef(),
		TargetBranch: pr.Base.GetRef(),
		HeadSHA:      pr.Head.GetSHA(),
	}, nil
}

func (g *GitHubClient) CleanupWebhooks(repository, clusterAppDomain string) error {
	hooks, err := g.Github.ListRepoWebhooks(repository)
	if err != nil {
		return err
	}
	for _, h := range hooks {
		hookUrl := h.Config["url"].(string)
		if strings.Contains(hookUrl, clusterAppDomain) {
			fmt.Printf("removing webhook URL: %s\n", hookUrl)
			err = g.Github.DeleteWebhook(repository, h.GetID())
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}
