package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wraient/godo/internal"

)

func main() {
	// Parse command line flags
	useGoogle := flag.Bool("google", false, "Use Google Tasks for storage")
	flag.Parse()

	// Set the global flag for Google Tasks mode
	internal.UseGoogleTasks = *useGoogle

	var tasks []internal.Task
	var err error

	// Load configuration first, regardless of mode
	config, err := internal.LoadConfig(filepath.Join(os.Getenv("HOME"), ".config", "godo", "config"))
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}
	internal.SetGlobalConfig(&config)

	if internal.UseGoogleTasks {
		err = internal.InitializeGoogleTasks()
		if err != nil {
			fmt.Printf("Error initializing Google Tasks: %v\n", err)
			os.Exit(1)
		}

		tasks, err = internal.GoogleTasksClientVar.LoadTasks()
		if err != nil {
			fmt.Printf("Error loading tasks from Google: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Load tasks based on storage mode
		tasks, err = internal.ImportTasks()
		if err != nil {
			fmt.Printf("Error loading tasks: %v\n", err)
			os.Exit(1)
		}
	}

	// If no tasks exist, create an intro task
	if len(tasks) == 0 {
		now := time.Now()
		tasks = []internal.Task{
			{
				Id:      "1",
				Title:   "Welcome to Godo!",
				Notes:   "This is your first task. Press 'n' to create a new task, 'e' to edit this task, or 'd' to delete it.",
				Created: now,
				Updated: now,
			},
		}
		// Save the intro task
		if err := internal.SaveTasks(tasks); err != nil {
			fmt.Printf("Error saving intro task: %v\n", err)
		}
	}

	internal.RunTaskUI(tasks, internal.GoogleTasksClientVar)
}
