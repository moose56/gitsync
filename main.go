package main

import (
	"github.com/alitto/pond"
	"github.com/ktrysmt/go-bitbucket"
	"os"
	"path/filepath"
	"sync"
)

func main() {
	pullRepositories()
}

func pullRepositories() {
	// load all the config options
	config := NewConfig()

	// create a logger to use throughout the application
	logger := NewLogger(config.LogFile)
	defer logger.Sync()

	logger.Info("Starting gitsync")
	if config.IsDryRun {
		logger.Info("Dry run")
	}

	// create a bitbucket client to interact with the bitbucket api
	client, err := NewBitbucketClient(config.User, config.Key, config.Secret, logger)
	if err != nil {
		logger.Panic(err)
	}

	workspaces, err := client.WorkspaceList()
	if err != nil {
		logger.Panic(err)
	}

	// create a worker pool, so we can only pull up to 7
	// repositories at a time
	pool := pond.New(7, 1000)

	// slice to store list of failed repos
	failed := make([]bitbucket.Repository, 0)

	// create a mutex, so we can guard access to the
	// failed slice from within goroutines
	var mu sync.Mutex

	// loop through all the workspaces available to our account.
	// we are going to pull every repo in each workspace, so we are
	// going to be looking through all of them one by one.
	for _, workspace := range workspaces {
		repositories, err := client.RepositoryList(&workspace)
		if err != nil {
			logger.Panic(err)
		}

		// loop through each repository in the workspace and pull it
		for _, repository := range repositories {
			r := repository

			// add goroutine to the pool
			pool.Submit(func() {
				logger.Info("Pull\t\t", r.Full_name)

				if !config.IsDryRun {
					// create a directory for the repository. This will create the
					// following directory structure: <WorkingDir>\<workspaceSlug>\<repositorySlug>
					repoPath := filepath.Join(config.OutputDir, r.Full_name)
					if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
						logger.Panic(err)
					}

					// pull the repository. If the pull fails record it and
					// move onto the next one, so we pull as many as we can.
					//
					// We pull each repository rather than clone it as the url used
					// to get the repository includes an OAuth token. If we clone,
					// the token will be stored in plain text in the .git/config
					// file. If we pull, even if we have not previously cloned, it will
					// not include any remote url in the /git/config file.
					//
					// always pulling also means we don't need to differentiate between
					// new repositories and repositories we have pulled in the past. The pull
					// operation works for both scenarios
					if out, err := client.Pull(repoPath, &r); err != nil {
						logger.Warn("Pull failed\t", r.Full_name, " (", out, ")")

						// add repository to failed in a concurrent safe way
						mu.Lock()
						failed = append(failed, r)
						mu.Unlock()
					}
				}
			})
		}
	}
	// wait for all goroutines to complete
	pool.StopAndWait()

	// if there are any failed pulls list the names of
	// the repos that failed
	logger.Info("Failed to pull: ", len(failed))
	for _, repository := range failed {
		logger.Info(">> ", repository.Full_name)
	}

	if config.IsDryRun {
		logger.Info("Dry run")
	}

	logger.Info("Completed gitsync")
}
