package main

import (
	"fmt"
	"github.com/wraient/godo/internal"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// Load config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	configPath := filepath.Join(homeDir, ".config", "godo", "godo.conf")
	config, err := internal.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}
	internal.SetGlobalConfig(&config)

	// Try to load existing tasks
	tasks, err := internal.LoadTasks()
	if err != nil {
		fmt.Printf("Error loading tasks: %v\n", err)
		os.Exit(1)
	}

	// If no tasks exist, create an intro task
	if len(tasks) == 0 {
		now := time.Now()
		tasks = []internal.Task{
			{
				ID:          "1",
				Title:       "Welcome to Godo!",
				Description: "This is your task management app.",
				Notes:       "",
				Completed:   false,
				CreatedAt:   now,
				DueDate:     now.AddDate(0, 0, 1),
				Tasks:       []internal.Task{},
			},
		}

		// Save the intro task
		if err := internal.SaveTasks(tasks); err != nil {
			fmt.Printf("Error saving tasks: %v\n", err)
			os.Exit(1)
		}
	}

	// Run the Bubble Tea Task UI
	internal.RunTaskUI(tasks)
}
