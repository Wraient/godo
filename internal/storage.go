package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"fmt"
)

// SaveTasks saves the tasks to a JSON file in the configured storage path
func SaveTasks(tasks []Task) error {
	config := GetGlobalConfig()
	if config == nil {
		return fmt.Errorf("global config not initialized")
	}

	storagePath := os.ExpandEnv(config.StoragePath)
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %v", err)
	}

	tasksFile := filepath.Join(storagePath, "tasks.json")
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %v", err)
	}

	if err := os.WriteFile(tasksFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write tasks file: %v", err)
	}

	return nil
}

// LoadTasks loads tasks from the JSON file in the configured storage path
func LoadTasks() ([]Task, error) {
	config := GetGlobalConfig()
	if config == nil {
		return nil, fmt.Errorf("global config not initialized")
	}

	storagePath := os.ExpandEnv(config.StoragePath)
	tasksFile := filepath.Join(storagePath, "tasks.json")

	if _, err := os.Stat(tasksFile); os.IsNotExist(err) {
		return []Task{}, nil
	}

	data, err := os.ReadFile(tasksFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file: %v", err)
	}

	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tasks: %v", err)
	}

	return tasks, nil
}
