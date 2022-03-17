package main

import (
	"encoding/json"
	"github.com/ktrysmt/go-bitbucket"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitClient A simple interface to abstract away any 3rd party Bitbucket client used. Only
// the required functionality is exposed.
type GitClient interface {
	WorkspaceList() ([]bitbucket.Workspace, error)
	RepositoryList(workspace *bitbucket.Workspace) ([]bitbucket.Repository, error)
	Pull(path string, repository *bitbucket.Repository) (string, error)
	oAuthLink(repository *bitbucket.Repository) string
}

// bitBucketClient implementation of GitClient specifically to wrap the go-bitbucket module and make it
// easier to use in this application
type bitBucketClient struct {
	user   string
	key    string
	secret string
	token  string
	logger Logger
	client *bitbucket.Client
}

type link struct {
	Name string `json:"name"`
	Href string `json:"href"`
}

func NewBitbucketClient(bitbucketUser, bitbucketKey, bitbucketSecret string, logger Logger) (GitClient, error) {
	c := bitbucket.NewOAuthClientCredentials(bitbucketKey, bitbucketSecret)

	return &bitBucketClient{
		user:   bitbucketUser,
		key:    bitbucketKey,
		secret: bitbucketSecret,
		client: c,
		token:  c.GetOAuthToken().AccessToken,
		logger: logger,
	}, nil
}

func (c *bitBucketClient) WorkspaceList() ([]bitbucket.Workspace, error) {
	l, err := c.client.Workspaces.List()
	if err != nil {
		return nil, err
	}

	return l.Workspaces, nil
}

func (c *bitBucketClient) RepositoryList(workspace *bitbucket.Workspace) ([]bitbucket.Repository, error) {
	opts := &bitbucket.RepositoriesOptions{Owner: workspace.Slug, Role: "member"}

	l, err := c.client.Repositories.ListForAccount(opts)
	if err != nil {
		return nil, err
	}

	return l.Items, nil
}

// we need to replace the username in the
// http url with the auth token, so it will work without
// there being an ssh key present
func (c *bitBucketClient) oAuthLink(repository *bitbucket.Repository) string {
	link := c.cloneLink(repository)
	key := "x-token-auth:" + c.token
	return strings.Replace(link, c.user, key, 1)
}

func (c *bitBucketClient) Pull(path string, repository *bitbucket.Repository) (string, error) {
	// https://github.blog/2012-09-21-easier-builds-and-deployments-using-git-over-https-and-oauth/
	// Note: Tokens should be treated as passwords. Putting the token in the clone URL will result in Git
	// writing it to the .git/config file in plain text. Unfortunately, this happens for HTTP passwords,
	// too. We decided to use the token as the HTTP username to avoid colliding with credential helpers
	// available for OS X, Windows, and Linux.
	//
	// To avoid writing tokens to disk, don’t clone. Instead, just use the full git URL in your
	// push/pull operations.
	if shouldInit(path) {
		out, err := exec.Command("git", "init", path).CombinedOutput()
		if err != nil {
			return string(out), err
		}

		c.logger.Info("Init ", path)
	}

	url := c.oAuthLink(repository)
	out, err := exec.Command("git", "-C", path, "pull", url).CombinedOutput()
	if err != nil {
		return strings.Trim(string(out), " \n"), err
	}

	return strings.Trim(string(out), " \n"), nil
}

// clone links are stored in a map[string, interface{}] in the bitbucket.Repository
// type. This means we need to do some marshalling and unmarshalling to convert to
// a link type and access the url values within
func (c *bitBucketClient) cloneLink(repository *bitbucket.Repository) string {
	j, _ := json.Marshal(repository.Links["clone"])

	var links []link
	href := ""

	_ = json.Unmarshal(j, &links)

	for _, l := range links {
		if l.Name == "https" {
			href = l.Href
		}
	}

	return href
}

func shouldInit(path string) bool {
	gitPath := filepath.Join(path, ".git")
	if _, err := os.Stat(gitPath); os.IsNotExist(err) {
		return true
	}

	return false
}
