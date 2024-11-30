package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	v1 "google.golang.org/api/tasks/v1"
)

var (
	googleConfig *oauth2.Config
	taskService  *v1.Service
	UseGoogleTasks bool
	taskCache    *GoogleTasksCache
	currentModel *model // Reference to current UI model
	GoogleTasksClientVar *GoogleTasksClient
)

// GoogleTasksCache holds the cached tasks and handles background updates
type GoogleTasksCache struct {
	Tasks    []Task
	LastSync time.Time
	mu       sync.RWMutex
}

// GoogleTasksClient wraps Google Tasks service
type GoogleTasksClient struct {
	service *v1.Service
}

// NewGoogleTasksClient returns a new GoogleTasksClient instance
func NewGoogleTasksClient(service *v1.Service) *GoogleTasksClient {
	return &GoogleTasksClient{service: service}
}

// CreateTask creates a new task in the specified task list
func (c *GoogleTasksClient) CreateTask(task Task, listID string) (Task, error) {
	if listID == "" {
		// Fallback to first list if no list ID provided
		taskList, err := c.service.Tasklists.List().Do()
		if err != nil || len(taskList.Items) == 0 {
			return task, fmt.Errorf("no task lists found: %v", err)
		}
		listID = taskList.Items[0].Id
	}

	fmt.Printf("Debug: Creating task with Title: %s, Parent: %s in list: %s\n", task.Title, task.Parent, listID)

	// Create the task with required fields
	newTask := &v1.Task{
		Title:    task.Title,
		Notes:    task.Notes,
		Status:   task.Status,
		Parent:   task.Parent, // This is important for subtasks
		Position: task.Position,
	}

	// Only set due date if it's not zero
	if !task.DueDate.IsZero() {
		newTask.Due = task.DueDate.Format(time.RFC3339)
	}

	var err error
	var createdTask *v1.Task

	if task.Parent != "" {
		// If this is a subtask, use Insert with parent
		createdTask, err = c.service.Tasks.Insert(listID, newTask).Parent(task.Parent).Do()
	} else {
		// If this is a top-level task, use regular Insert
		createdTask, err = c.service.Tasks.Insert(listID, newTask).Do()
	}

	if err != nil {
		return task, fmt.Errorf("failed to create task: %v", err)
	}

	// Update the task with the response from Google Tasks
	task.Id = createdTask.Id
	task.Kind = createdTask.Kind
	task.SelfLink = createdTask.SelfLink
	task.Etag = createdTask.Etag
	task.Parent = createdTask.Parent // Make sure to capture the parent ID
	task.Position = createdTask.Position

	return task, nil
}

// UpdateTask updates an existing task in the first task list
func (c *GoogleTasksClient) UpdateTask(task Task) error {
	// Implement task update logic
	taskList, err := c.service.Tasklists.List().Do()
	if err != nil || len(taskList.Items) == 0 {
		return fmt.Errorf("no task lists found: %v", err)
	}

	updatedTask := &v1.Task{
		Id:          task.Id,
		Title:       task.Title,
		Notes:       task.Notes,
		Status:      task.Status,
		Due:         task.DueDate.Format(time.RFC3339),
		Parent:      task.Parent,
		Position:    task.Position,
	}

	_, err = c.service.Tasks.Update(taskList.Items[0].Id, task.Id, updatedTask).Do()
	return err
}

// DeleteTask deletes a task from the first task list
func (c *GoogleTasksClient) DeleteTask(taskID string) error {
	// Implement task deletion logic
	taskList, err := c.service.Tasklists.List().Do()
	if err != nil || len(taskList.Items) == 0 {
		return fmt.Errorf("no task lists found: %v", err)
	}

	return c.service.Tasks.Delete(taskList.Items[0].Id, taskID).Do()
}

// LoadTasks retrieves tasks from the first task list
func (c *GoogleTasksClient) LoadTasks() ([]Task, error) {
	// Fetch tasks using the existing fetchGoogleTasks function
	return fetchGoogleTasks()
}

// InitializeGoogleTasks sets up the Google Tasks API client and cache
func InitializeGoogleTasks() error {
	// Initialize OAuth2 config
	config := GetGlobalConfig()
	googleConfig = &oauth2.Config{
		ClientID:     config.GoogleClientID,
		ClientSecret: config.GoogleClientSecret,
		RedirectURL:  "http://localhost:8080/callback",
		Scopes: []string{
			"https://www.googleapis.com/auth/tasks",
		},
		Endpoint: google.Endpoint,
	}

	// Load or get new token
	token, err := loadToken()
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No token found. Starting OAuth flow...")
			token, err = getTokenFromWeb()
			if err != nil {
				return fmt.Errorf("error getting token from web: %v", err)
			}
			if err := saveToken(token); err != nil {
				return fmt.Errorf("error saving token: %v", err)
			}
			fmt.Println("Successfully authenticated with Google!")
		} else {
			return fmt.Errorf("error loading token: %v", err)
		}
	}

	// Create Tasks service
	service, err := v1.NewService(context.Background(), option.WithTokenSource(googleConfig.TokenSource(context.Background(), token)))
	if err != nil {
		return fmt.Errorf("error creating tasks service: %v", err)
	}
	taskService = service
	GoogleTasksClientVar = NewGoogleTasksClient(service)

	// Initialize cache
	taskCache = &GoogleTasksCache{
		Tasks:    make([]Task, 0),
		LastSync: time.Time{},
		mu:       sync.RWMutex{},
	}

	// Load cached tasks
	if err := loadCachedTasks(); err != nil {
		fmt.Printf("Error loading cache: %v\n", err)
	}

	// Start background sync
	startBackgroundSync()

	return nil
}

func startBackgroundSync() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for range ticker.C {
			tasks, err := fetchGoogleTasks()
			if err != nil {
				fmt.Printf("Error in background sync: %v\n", err)
				continue
			}

			taskCache.mu.Lock()
			if !tasksEqual(taskCache.Tasks, tasks) {
				fmt.Println("New tasks found in background sync, updating...")
				taskCache.Tasks = tasks
				taskCache.LastSync = time.Now()
				if err := saveCachedTasks(); err != nil {
					fmt.Printf("Error saving to cache: %v\n", err)
				}
				notifyUIOfChanges(tasks)
			}
			taskCache.mu.Unlock()
		}
	}()
}

func loadCachedTasks() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error getting home directory: %v", err)
	}

	cacheFile := filepath.Join(home, ".local", "share", "godo", "google_tasks_cache.json")
	
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Initialize empty cache if file doesn't exist
			taskCache.Tasks = make([]Task, 0)
			taskCache.LastSync = time.Time{}
			return nil
		}
		return fmt.Errorf("error reading cache file: %v", err)
	}

	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return fmt.Errorf("error unmarshaling cache: %v", err)
	}

	taskCache.Tasks = tasks
	fmt.Printf("Cache loaded from: %s\n", cacheFile) // Debug print
	return nil
}

func saveCachedTasks() error {
	// Ensure cache directory exists
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error getting home directory: %v", err)
	}

	cacheDir := filepath.Join(home, ".local", "share", "godo")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("error creating cache directory: %v", err)
	}

	cacheFile := filepath.Join(cacheDir, "google_tasks_cache.json")
	
	// Marshal tasks with indentation for readability
	data, err := json.MarshalIndent(taskCache.Tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling tasks: %v", err)
	}

	// Write to temporary file first
	tempFile := cacheFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("error writing temporary cache file: %v", err)
	}

	// Rename temporary file to actual cache file (atomic operation)
	if err := os.Rename(tempFile, cacheFile); err != nil {
		os.Remove(tempFile) // Clean up temp file if rename fails
		return fmt.Errorf("error renaming cache file: %v", err)
	}

	fmt.Printf("Cache saved to: %s\n", cacheFile) // Debug print
	return nil
}

func ImportTasks() ([]Task, error) {
	if UseGoogleTasks {
		// First try to load from cache
		if err := loadCachedTasks(); err != nil {
			fmt.Printf("Error loading cache: %v\n", err)
		}

		// Start background fetch from Google immediately
		go func() {
			tasks, err := fetchGoogleTasks()
			if err != nil {
				fmt.Printf("Error fetching from Google: %v\n", err)
				return
			}

			taskCache.mu.Lock()
			if !tasksEqual(taskCache.Tasks, tasks) {
				fmt.Println("New tasks found in Google, updating...")
				taskCache.Tasks = tasks
				taskCache.LastSync = time.Now()
				if err := saveCachedTasks(); err != nil {
					fmt.Printf("Error saving to cache: %v\n", err)
				}
				notifyUIOfChanges(tasks)
			}
			taskCache.mu.Unlock()
		}()

		// Return cached tasks immediately if available
		taskCache.mu.RLock()
		defer taskCache.mu.RUnlock()
		
		if len(taskCache.Tasks) > 0 {
			fmt.Println("Showing cached tasks while fetching from Google...")
			cachedTasks := make([]Task, len(taskCache.Tasks))
			copy(cachedTasks, taskCache.Tasks)
			return cachedTasks, nil
		}

		// If no cache, wait for Google fetch
		fmt.Println("No cached tasks available, fetching from Google...")
		return fetchGoogleTasks()
	}
	return ImportFromLocal()
}

func tasksEqual(a, b []Task) bool {
	if len(a) != len(b) {
		return false
	}
	
	// Compare tasks based on their content
	aJson, _ := json.Marshal(a)
	bJson, _ := json.Marshal(b)
	return bytes.Equal(aJson, bJson)
}

func loadToken() (*oauth2.Token, error) {
	config := GetGlobalConfig()
	tokenFile := os.ExpandEnv(config.GoogleTokenPath)
	
	f, err := os.Open(tokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	
	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

func saveToken(token *oauth2.Token) error {
	config := GetGlobalConfig()
	tokenFile := os.ExpandEnv(config.GoogleTokenPath)
	
	// Ensure directory exists
	dir := filepath.Dir(tokenFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating token directory: %v", err)
	}

	// Write to temp file first
	tempFile := tokenFile + ".tmp"
	f, err := os.OpenFile(tempFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("error creating token file: %v", err)
	}
	defer f.Close()

	// Save token
	if err := json.NewEncoder(f).Encode(token); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("error encoding token: %v", err)
	}

	// Ensure write is flushed to disk
	if err := f.Sync(); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("error syncing token file: %v", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, tokenFile); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("error saving token file: %v", err)
	}

	fmt.Printf("Token saved to: %s\n", tokenFile)
	return nil
}

func getTokenFromWeb() (*oauth2.Token, error) {
	// Generate OAuth URL
	authURL := googleConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	
	// Start local server to receive callback
	ch := make(chan string)
	server := &http.Server{Addr: ":8080"}
	
	// Handle callback
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		ch <- code
		fmt.Fprintf(w, "Authorization successful! You can close this window.")
		go func() {
			time.Sleep(time.Second)
			server.Shutdown(context.Background())
		}()
	})

	// Open browser
	fmt.Printf("Opening browser for authentication...\n")
	if err := open(authURL); err != nil {
		fmt.Printf("Failed to open browser automatically. Please open this URL in your browser:\n%v\n", authURL)
	}

	// Start server
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	// Wait for code
	code := <-ch

	// Exchange code for token
	token, err := googleConfig.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %v", err)
	}

	return token, nil
}

func open(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "linux":
		cmd = "xdg-open"
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	default:
		return fmt.Errorf("unsupported platform")
	}

	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func ImportFromLocal() ([]Task, error) {
	// Read from local storage
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %v", err)
	}

	tasksFile := filepath.Join(home, ".local", "share", "godo", "tasks.json")
	data, err := os.ReadFile(tasksFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []Task{}, nil
		}
		return nil, fmt.Errorf("failed to read tasks file: %v", err)
	}

	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse tasks file: %v", err)
	}

	return tasks, nil
}

func SaveGoogleTasks(tasks []Task) error {
	if UseGoogleTasks {
		return ExportToGoogle(tasks)
	}
	return SaveToLocal(tasks)
}

func SaveToLocal(tasks []Task) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}

	tasksDir := filepath.Join(home, ".local", "share", "godo")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		return fmt.Errorf("failed to create tasks directory: %v", err)
	}

	tasksFile := filepath.Join(tasksDir, "tasks.json")
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %v", err)
	}

	if err := os.WriteFile(tasksFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write tasks file: %v", err)
	}

	return nil
}

func ExportToGoogle(tasks []Task) error {
	if GoogleTasksClientVar == nil {
		return fmt.Errorf("Google Tasks client not initialized")
	}

	// For each task list
	for _, taskList := range tasks {
		// Skip if not a task list
		if taskList.Kind != "tasks#taskList" {
			continue
		}

		// Update or create task list
		googleTaskList := &v1.TaskList{
			Id:    taskList.Id,
			Title: taskList.Title,
			Kind:  taskList.Kind,
			Etag:  taskList.Etag,
		}

		var err error
		if taskList.Id != "" {
			_, err = GoogleTasksClientVar.service.Tasklists.Update(taskList.Id, googleTaskList).Do()
		} else {
			_, err = GoogleTasksClientVar.service.Tasklists.Insert(googleTaskList).Do()
		}
		if err != nil {
			return fmt.Errorf("failed to update/create task list: %v", err)
		}

		// Export tasks in this list
		if err := exportTasksInList(taskList.Id, taskList.Tasks); err != nil {
			return err
		}
	}

	return nil
}

func exportTasksInList(listID string, tasks []Task) error {
	for _, task := range tasks {
		googleTask := &v1.Task{
			Id:       task.Id,
			Title:    task.Title,
			Notes:    task.Notes,
			Status:   task.Status,
			Parent:   task.Parent,
			Position: task.Position,
			Kind:     task.Kind,
			Etag:     task.Etag,
		}

		if !task.DueDate.IsZero() {
			googleTask.Due = task.DueDate.Format(time.RFC3339)
		}

		if task.Completed {
			completedStr := task.CompletedDate.Format(time.RFC3339)
			googleTask.Completed = &completedStr
			googleTask.Status = "completed"
		} else {
			googleTask.Status = "needsAction"
		}

		var err error
		if task.Id != "" {
			err = GoogleTasksClientVar.UpdateTask(task)
		} else {
			_, err = GoogleTasksClientVar.CreateTask(task, listID)
		}
		if err != nil {
			return fmt.Errorf("failed to update/create task: %v", err)
		}

		// Recursively export child tasks
		if err := exportTasksInList(listID, task.Tasks); err != nil {
			return err
		}
	}

	return nil
}

func buildTaskHierarchy(tasks []*v1.Task, taskMap map[string]*Task) []Task {
	// First, sort tasks by position
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Position < tasks[j].Position
	})

	// Find all root tasks (tasks with no parent)
	var rootTasks []Task
	for _, googleTask := range tasks {
		task := taskMap[googleTask.Id]
		if task.Parent == "" {
			// Find all children of this task
			task.Tasks = findChildren(googleTask.Id, tasks, taskMap)
			rootTasks = append(rootTasks, *task)
		}
	}
	return rootTasks
}

func findChildren(parentID string, allTasks []*v1.Task, taskMap map[string]*Task) []Task {
	var children []Task
	for _, googleTask := range allTasks {
		if googleTask.Parent == parentID {
			task := taskMap[googleTask.Id]
			// Recursively find children of this child
			task.Tasks = findChildren(googleTask.Id, allTasks, taskMap)
			children = append(children, *task)
		}
	}
	// Sort children by position
	sort.Slice(children, func(i, j int) bool {
		return children[i].Position < children[j].Position
	})
	return children
}

func SetCurrentModel(m *model) {
	currentModel = m
}

func notifyUIOfChanges(tasks []Task) {
	if currentModel != nil {
		currentModel.UpdateTasks(tasks)
	}
}

func fetchGoogleTasks() ([]Task, error) {
	if GoogleTasksClientVar == nil {
		return nil, fmt.Errorf("Google Tasks client not initialized")
	}
	
	// Get all task lists
	taskLists, err := GoogleTasksClientVar.service.Tasklists.List().Do()
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve task lists: %v", err)
	}

	var allTasks []Task
	
	// For each task list
	for _, taskList := range taskLists.Items {
		// Create a task list container
		listTask := Task{
			Id:      taskList.Id,
			Title:   taskList.Title,
			Kind:    taskList.Kind,
			Etag:    taskList.Etag,
			Updated: time.Now(),
			Created: time.Now(),
			Tasks:   []Task{},
		}
		
		// Get all tasks in this list
		tasks, err := GoogleTasksClientVar.service.Tasks.List(taskList.Id).Do()
		if err != nil {
			fmt.Printf("Unable to retrieve tasks for list %s: %v\n", taskList.Title, err)
			continue
		}

		// First pass: create all tasks
		taskMap := make(map[string]*Task)
		for _, googleTask := range tasks.Items {
			task := Task{
				Id:          googleTask.Id,
				Title:       googleTask.Title,
				Notes:       googleTask.Notes,
				Status:      googleTask.Status,
				Completed:   googleTask.Status == "completed",
				Parent:      googleTask.Parent,
				Position:    googleTask.Position,
				Kind:        googleTask.Kind,
				SelfLink:    googleTask.SelfLink,
				Etag:        googleTask.Etag,
				Tasks:       []Task{},
			}

			// Parse due date if present
			if googleTask.Due != "" {
				if dueDate, err := time.Parse(time.RFC3339, googleTask.Due); err == nil {
					task.DueDate = dueDate
				}
			}

			// Parse completed date if present
			if googleTask.Completed != nil {
				if completedDate, err := time.Parse(time.RFC3339, *googleTask.Completed); err == nil {
					task.CompletedDate = completedDate
					task.Completed = true
				}
			}

			// Parse updated time if present
			if googleTask.Updated != "" {
				if updatedTime, err := time.Parse(time.RFC3339, googleTask.Updated); err == nil {
					task.Updated = updatedTime
				}
			}

			taskMap[task.Id] = &task
		}

		// Build task hierarchy recursively
		listTask.Tasks = buildTaskHierarchy(tasks.Items, taskMap)
		allTasks = append(allTasks, listTask)
	}
	
	return allTasks, nil
}