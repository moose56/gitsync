package main

import (
	"encoding/json"
	"github.com/ktrysmt/go-bitbucket"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitSyncClient A simple interface to abstract away any 3rd party Bitbucket client used. Only
// the required functionality is exposed.
type GitSyncClient interface {
	WorkspaceList() ([]Workspace, error)
	RepositoryList(workspace *Workspace) ([]Repository, error)
	Copy(path string, repository *Repository) (string, error)
	oAuthLink(repository *Repository) string
}

type Workspace struct {
	Slug string
}

type Repository struct {
	Full_name string
	Links     map[string]interface{}
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

func NewBitbucketClient(bitbucketUser, bitbucketKey, bitbucketSecret string, logger Logger) (GitSyncClient, error) {
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

func (c *bitBucketClient) WorkspaceList() ([]Workspace, error) {
	l, err := c.client.Workspaces.List()
	if err != nil {
		return nil, err
	}

	var results []Workspace

	for _, w := range l.Workspaces {
		results = append(results, Workspace{Slug: w.Slug})
	}

	return results, nil
}

func (c *bitBucketClient) RepositoryList(workspace *Workspace) ([]Repository, error) {
	opts := &bitbucket.RepositoriesOptions{Owner: workspace.Slug, Role: "member"}

	l, err := c.client.Repositories.ListForAccount(opts)
	if err != nil {
		return nil, err
	}

	var results []Repository
	for _, r := range l.Items {
		results = append(results, Repository{Full_name: r.Full_name, Links: r.Links})
	}

	return results, nil
}

// we need to replace the username in the
// http url with the auth token, so it will work without
// there being an ssh key present
func (c *bitBucketClient) oAuthLink(repository *Repository) string {
	link := c.cloneLink(repository)
	key := "x-token-auth:" + c.token
	return strings.Replace(link, c.user, key, 1)
}

func (c *bitBucketClient) Copy(path string, repository *Repository) (string, error) {
	// https://github.blog/2012-09-21-easier-builds-and-deployments-using-git-over-https-and-oauth/
	// Note: Tokens should be treated as passwords. Putting the token in the clone URL will result in Git
	// writing it to the .git/config file in plain text. Unfortunately, this happens for HTTP passwords,
	// too. We decided to use the token as the HTTP username to avoid colliding with credential helpers
	// available for OS X, Windows, and Linux.

	cloneUrl := c.cloneLink(repository)
	tokenUrl := c.oAuthLink(repository)
	gitDir := filepath.Join(path, ".git")

	// https://stackoverflow.com/questions/67699/how-to-clone-all-remote-branches-in-git/7216269#7216269
	//git clone --mirror path/to/original path/to/dest/.git
	//cd path/to/dest
	//git config --bool core.bare false
	//git checkout

	// clone
	out, err := exec.Command("git", "clone", "--mirror", tokenUrl, gitDir).CombinedOutput()
	if err != nil {
		return strings.Trim(string(out), " \n"), err
	}

	// convert to ordinary repo
	out, err = exec.Command("git", "-C", path, "config", "--bool", "core.bare", "false").CombinedOutput()
	if err != nil {
		return strings.Trim(string(out), " \n"), err
	}

	// update remote url, so it does not include the auth token. tokens should be treated
	// as passwords. Putting the token in the clone URL will result in Git writing it to
	// the .git/config file in plain text. we reset it to the standard https clone url
	out, err = exec.Command("git", "-C", path, "remote", "set-url", "origin", cloneUrl).CombinedOutput()
	if err != nil {
		return strings.Trim(string(out), " \n"), err
	}

	// checkout default branch
	out, err = exec.Command("git", "-C", path, "checkout").CombinedOutput()
	if err != nil {
		return strings.Trim(string(out), " \n"), err
	}

	return strings.Trim(string(out), " \n"), nil
}

// clone links are stored in a map[string, interface{}] in the Repository
// type. This means we need to do some marshalling and unmarshalling to convert to
// a link type and access the url values within
func (c *bitBucketClient) cloneLink(repository *Repository) string {
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
