package main

import (
	"os"
	"strings"
)

type Config struct {
	User      string
	Key       string
	Secret    string
	OutputDir string
	LogFile   string
	IsDryRun  bool
}

func NewConfig() *Config {
	isDry := false
	for _, v := range os.Args[1:] {
		if v == strings.ToLower("dryrun") {
			isDry = true
		}
	}

	return &Config{
		User:      os.Getenv("BITBUCKET_USER"),
		Key:       os.Getenv("BITBUCKET_KEY"),
		Secret:    os.Getenv("BITBUCKET_SECRET"),
		OutputDir: os.Getenv("OUTPUT_DIR"),
		LogFile:   os.Getenv("LOG_FILE"),
		IsDryRun:  isDry,
	}
}
