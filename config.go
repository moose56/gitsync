package main

import "os"

type Config struct {
	User      string
	Key       string
	Secret    string
	OutputDir string
	LogFile   string
}

func NewConfig() *Config {
	return &Config{
		User:      os.Getenv("BITBUCKET_USER"),
		Key:       os.Getenv("BITBUCKET_KEY"),
		Secret:    os.Getenv("BITBUCKET_SECRET"),
		OutputDir: os.Getenv("OUTPUT_DIR"),
		LogFile:   os.Getenv("LOG_FILE"),
	}
}
