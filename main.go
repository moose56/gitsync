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

	// slice to store list of failed repos
	failed := make([]bitbucket.Repository, 0)

	// create a mutex, so we can guard access to the
	// failed slice from within goroutines
	var mu sync.Mutex

	// create a worker pool, so we can only pull up to 7
	// repositories at a time
	pool := pond.New(7, 1000)

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
				logger.Info("Copy\t\t", r.Full_name)

				if !config.IsDryRun {
					// create a path to copy the repository and a backup path
					repoDir := filepath.Join(config.OutputDir, r.Full_name)
					backupDir := repoDir + "-backup"

					// check if either of these paths exist
					repoExists, err := exists(repoDir)
					if err != nil {
						logger.Panic(err)
					}
					backupExists, err := exists(repoDir)
					if err != nil {
						logger.Panic(err)
					}

					// at this point we can be in one of the following states:
					// 1. No directory and no '-backup' directory - no special action required
					// 2. No directory and a '-backup' directory  - no special action required
					// 3. A directory and no '-backup' directory  - special action needed
					// 4. A directory and a '-backup' directory   - special action needed

					// 3. A directory and no '-backup' directory
					if repoExists && !backupExists {
						// create a backup
						if err := archive(repoDir, backupDir); err != nil {
							logger.Panic(err)
						}
					}

					// 4. A directory and a '-backup' directory
					if repoExists && backupExists {
						// 1. delete the old backup
						if err := os.RemoveAll(backupDir); err != nil {
							logger.Panic(err)
						}
						// 2. create a new backup
						if err := archive(repoDir, backupDir); err != nil {
							logger.Panic(err)
						}
					}

					// store the path we need to remove following the copy operation.
					// by default this will be the '-backup' version because we only want
					// to keep the latest version, but if the copy fails we want to clean
					// up the new version and keep the -backup one
					cleanupDir := backupDir

					// create a directory for the repository. This will create the
					// following directory structure: <WorkingDir>\<workspaceSlug>\<repositorySlug>
					if err := os.MkdirAll(repoDir, os.ModePerm); err != nil {
						logger.Panic(err)
					}

					// copy the repository. If the copy fails record it and
					// move onto the next one, so we pull as many as we can.
					if out, err := client.Copy(repoDir, &r); err != nil {
						logger.Warn("Copy failed\t", r.Full_name, " (", out, ")")

						// the copy failed so the new directory needs to be cleaned up
						cleanupDir = repoDir

						// add repository to failed in a concurrent safe way
						mu.Lock()
						failed = append(failed, r)
						mu.Unlock()
					}

					// cleanup the relevant path
					if err := os.RemoveAll(cleanupDir); err != nil {
						logger.Panic(err)
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

func archive(path, pathOld string) error {
	if err := os.Rename(path, pathOld); err != nil {
		return err
	}

	return nil
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
