package internal

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Task represents a task or subtask
type Task struct {
	Id            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Notes         string    `json:"notes"`
	Status        string    `json:"status"`
	Completed     bool      `json:"completed"`
	CreatedAt     time.Time `json:"createdAt"`
	DueDate       time.Time `json:"dueDate"`
	CompletedDate time.Time `json:"completedDate"`
	Parent        string    `json:"parent"`
	Position      string    `json:"position"`
	Kind          string    `json:"kind"`
	SelfLink      string    `json:"selfLink"`
	Etag          string    `json:"etag"`
	Updated       time.Time `json:"updated"`
	Created       time.Time `json:"created"`
	Deleted       bool      `json:"deleted"`
	Tasks         []Task    `json:"tasks"`
	Links         []struct {
		Type string `json:"type"`
		Desc string `json:"description"`
		Link string `json:"link"`
	} `json:"links"`
}

// Model represents the state of our Bubble Tea program
type model struct {
	tasks          []Task
	completedTasks []Task
	cursor         int
	currentPath    []Task // Tracks the current task hierarchy path
	input          textinput.Model
	inputActive    bool
	inputAction    string
	deletedTaskID  string
	editingField   string // Field currently being edited: "title", "description", "notes", "due_date"
	width          int     // Terminal width
	height         int     // Terminal height
	updateChan     chan []Task
	refreshChan    chan struct{} // Channel for UI refresh signals
	googleTasks    *GoogleTasksClient // Add Google Tasks client
	currentListID  string            // Current Google Tasks list ID
}

// NewModel initializes the Bubble Tea model with tasks
func NewModel(tasks []Task, client *GoogleTasksClient) model {
	ti := textinput.New()
	ti.Placeholder = "Enter task title..."
	ti.Focus()

	// Get the first task list ID
	taskLists, err := client.service.Tasklists.List().Do()
	var currentListID string
	if err == nil && len(taskLists.Items) > 0 {
		currentListID = taskLists.Items[0].Id
	}

	// Initialize channels
	updateChan := make(chan []Task, 10)
	refreshChan := make(chan struct{}, 1)

	// Split initial tasks
	active, completed := splitTasks(tasks)

	m := model{
		tasks:          active,
		completedTasks: completed,
		input:         ti,
		updateChan:    updateChan,
		refreshChan:   refreshChan,
		googleTasks:   client,
		currentListID: currentListID,
	}

	// Start update handler
	go m.handleUpdates()

	return m
}

// getCurrentTasks returns the current level's tasks based on currentPath
func (m *model) getCurrentTasks() ([]Task, []Task) {
	if len(m.currentPath) == 0 {
		return m.tasks, m.completedTasks
	}

	parentTask := &m.currentPath[len(m.currentPath)-1]
	active := make([]Task, 0)
	completed := make([]Task, 0)

	for _, task := range parentTask.Tasks {
		if task.Completed {
			completed = append(completed, task)
		} else {
			active = append(active, task)
		}
	}

	return active, completed
}

func (m *model) updateTerminalSize() {
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))
	m.width = width
	m.height = height
}

// Init starts the program
func (m model) Init() tea.Cmd {
	return m.waitForRefresh
}

// waitForRefresh waits for refresh signals
func (m model) waitForRefresh() tea.Msg {
	return <-m.refreshChan
}

// handleUpdates processes task updates in the background
func (m *model) handleUpdates() {
	for tasks := range m.updateChan {
		m.tasks = tasks
		active, completed := splitTasks(tasks)
		m.tasks = active
		m.completedTasks = completed
		tea.Println("Tasks updated from Google")
		
		// Send refresh signal
		select {
		case m.refreshChan <- struct{}{}:
		default:
			// Channel is full, skip refresh
		}
	}
}

// Update handles keypresses and updates the state of the UI
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case struct{}: // Refresh message
		return m, m.waitForRefresh
	
	case tea.KeyMsg:
		// If input is active, handle all text input
		if m.inputActive {
			switch msg.String() {
			case "esc":
				m.inputActive = false
				m.input.Blur()
				return m, nil
			case "enter":
				// Save the input based on action type
				active, completed := m.getCurrentTasks()
				switch m.inputAction {
				case "description", "notes":
					if len(m.currentPath) == 0 {
						if m.cursor < len(active) {
							if m.inputAction == "description" {
								active[m.cursor].Description = m.input.Value()
								active[m.cursor].Updated = time.Now()
								m.syncToGoogle(active[m.cursor])
							} else {
								active[m.cursor].Notes = m.input.Value()
								active[m.cursor].Updated = time.Now()
								m.syncToGoogle(active[m.cursor])
							}
						} else {
							completedIdx := m.cursor - len(active)
							if m.inputAction == "description" {
								completed[completedIdx].Description = m.input.Value()
								completed[completedIdx].Updated = time.Now()
								m.syncToGoogle(completed[completedIdx])
							} else {
								completed[completedIdx].Notes = m.input.Value()
								completed[completedIdx].Updated = time.Now()
								m.syncToGoogle(completed[completedIdx])
							}
						}
					} else {
						parentTask := &m.currentPath[len(m.currentPath)-1]
						if m.cursor < len(parentTask.Tasks) {
							if m.inputAction == "description" {
								parentTask.Tasks[m.cursor].Description = m.input.Value()
								parentTask.Tasks[m.cursor].Updated = time.Now()
								m.syncToGoogle(parentTask.Tasks[m.cursor])
							} else {
								parentTask.Tasks[m.cursor].Notes = m.input.Value()
								parentTask.Tasks[m.cursor].Updated = time.Now()
								m.syncToGoogle(parentTask.Tasks[m.cursor])
							}
						}
					}
					if err := SaveTasks(m.tasks); err != nil {
						fmt.Printf("Error saving tasks: %v\n", err)
					}
				case "rename":
					if len(m.currentPath) == 0 {
						if m.cursor < len(active) {
							active[m.cursor].Title = m.input.Value()
							active[m.cursor].Updated = time.Now()
							if active[m.cursor].Status == "" {
								active[m.cursor].Status = "needsAction"
							}
							m.syncToGoogle(active[m.cursor])
						} else {
							completedIdx := m.cursor - len(active)
							completed[completedIdx].Title = m.input.Value()
							completed[completedIdx].Updated = time.Now()
							if completed[completedIdx].Status == "" {
								completed[completedIdx].Status = "completed"
							}
							m.syncToGoogle(completed[completedIdx])
						}
					} else {
						parentTask := &m.currentPath[len(m.currentPath)-1]
						if m.cursor < len(parentTask.Tasks) {
							parentTask.Tasks[m.cursor].Title = m.input.Value()
							parentTask.Tasks[m.cursor].Updated = time.Now()
							if parentTask.Tasks[m.cursor].Status == "" {
								parentTask.Tasks[m.cursor].Status = "needsAction"
							}
							m.syncToGoogle(parentTask.Tasks[m.cursor])
						}
					}
					if err := SaveTasks(m.tasks); err != nil {
						fmt.Printf("Error saving tasks: %v\n", err)
					}
				case "due_date":
					dateStr := m.input.Value()
					if dateStr == "" {
						m.inputActive = false
						m.input.Blur()
						return m, nil
					}

					// Try parsing with different formats
					var dueDate time.Time
					var err error
					formats := []string{
						"2006-01-02 15:04",
						"2006-01-02",
						"01/02/2006",
						"02-01-2006",
					}

					for _, format := range formats {
						dueDate, err = time.Parse(format, dateStr)
						if err == nil {
							break
						}
					}

					if err != nil {
						tea.Printf("Invalid date format. Please use one of:\nYYYY-MM-DD HH:mm\nYYYY-MM-DD\nMM/DD/YYYY\nDD-MM-YYYY")
						return m, nil
					}

					var task *Task
					if len(m.currentPath) == 0 {
						active, completed := m.getCurrentTasks()
						if m.cursor < len(active) {
							task = &active[m.cursor]
						} else {
							completedIdx := m.cursor - len(active)
							task = &completed[completedIdx]
						}
					} else {
						parentTask := &m.currentPath[len(m.currentPath)-1]
						if m.cursor < len(parentTask.Tasks) {
							task = &parentTask.Tasks[m.cursor]
						}
					}

					if task != nil {
						task.DueDate = dueDate
						task.Updated = time.Now()
						if err := SaveTasks(m.tasks); err != nil {
							fmt.Printf("Error saving tasks: %v\n", err)
						}
						m.syncToGoogle(*task)
					}
				case "new_task":
					now := time.Now()
					newTask := Task{
						Title:     m.input.Value(),
						CreatedAt: now,
						Created:   now,
						Updated:   now,
						Status:    "needsAction",
						Kind:      "tasks#task",
						Notes:     "",
					}

					// Create task in Google Tasks first
					listID := m.currentListID
					if listID == "" {
						// If currentListID is empty, try to get it again
						taskLists, err := m.googleTasks.service.Tasklists.List().Do()
						if err != nil {
							fmt.Printf("Error getting task lists: %v\n", err)
							return m, nil
						}
						if len(taskLists.Items) > 0 {
							listID = taskLists.Items[0].Id
							m.currentListID = listID
						} else {
							fmt.Printf("Error: No task lists found\n")
							return m, nil
						}
					}

					// Set parent ID if we're in a sublist
					if len(m.currentPath) > 0 {
						currentTask := m.currentPath[len(m.currentPath)-1]
						// Only set parent if we're not at the root
						if currentTask.Kind != "tasks#taskList" {
							newTask.Parent = currentTask.Id
						}
					}

					fmt.Printf("Debug: Creating task in list %s with parent %s\n", listID, newTask.Parent)
					createdTask, err := m.googleTasks.CreateTask(newTask, listID)
					if err != nil {
						fmt.Printf("Error creating task in Google Tasks: %v\n", err)
						return m, nil
					}

					// Just add the task to wherever we currently are
					if len(m.currentPath) == 0 {
						m.tasks = append(m.tasks, createdTask)
						m.cursor = len(m.tasks) - 1
					} else {
						// Add to current view
						parentTask := m.currentPath[len(m.currentPath)-1]
						parentTask.Tasks = append(parentTask.Tasks, createdTask)
						m.cursor = len(parentTask.Tasks) - 1
						m.currentPath[len(m.currentPath)-1] = parentTask

						// Also update the task in the main task tree
						for i := range m.tasks {
							if m.tasks[i].Id == parentTask.Id {
								m.tasks[i] = parentTask
								break
							}
						}
					}

					if err := SaveTasks(m.tasks); err != nil {
						fmt.Printf("Error saving tasks: %v\n", err)
					}

					m.inputActive = false
					m.input.Blur()
					return m, nil
				case "delete":
					if m.input.Value() == "yes" {
						active, completed := m.getCurrentTasks()
						// Only allow deletion if there are tasks to delete
						if len(active) == 0 && len(completed) == 0 {
							return m, nil
						}

						if len(m.currentPath) == 0 {
							// Delete from main task list
							if m.cursor < len(active) {
								// Delete active task
								task := active[m.cursor]
								task.Status = "deleted"
								m.syncToGoogle(task)
								m.tasks = removeTask(m.tasks, task)
								if m.cursor >= len(active)-1 {
									m.cursor = len(active) - 2
									if m.cursor < 0 {
										m.cursor = 0
									}
								}
							} else if m.cursor < len(active)+len(completed) {
								// Delete completed task
								completedIdx := m.cursor - len(active)
								task := completed[completedIdx]
								task.Status = "deleted"
								m.syncToGoogle(task)
								m.completedTasks = removeTask(m.completedTasks, task)
								if m.cursor >= len(active)+len(completed)-1 {
									m.cursor = len(active) + len(completed) - 2
									if m.cursor < 0 {
										m.cursor = 0
									}
								}
							} else {
								// Cursor is out of bounds, don't delete anything
								return m, nil
							}
						} else {
							// Delete from subtask list
							currentTask := &m.tasks
							var taskPtr *Task
							for i, pathTask := range m.currentPath {
								for j := range *currentTask {
									if (*currentTask)[j].Id == pathTask.Id {
										if i == len(m.currentPath)-1 {
											taskPtr = &(*currentTask)[j]
										} else {
											currentTask = &(*currentTask)[j].Tasks
										}
										break
									}
								}
							}

							if taskPtr != nil {
								if m.cursor < len(active) {
									// Delete active subtask
									task := active[m.cursor]
									task.Status = "deleted"
									m.syncToGoogle(task)
									taskPtr.Tasks = removeTask(taskPtr.Tasks, task)
									if m.cursor >= len(active)-1 {
										m.cursor = len(active) - 2
										if m.cursor < 0 {
											m.cursor = 0
										}
									}
								} else {
									// Delete completed subtask
									completedIdx := m.cursor - len(active)
									task := completed[completedIdx]
									task.Status = "deleted"
									m.syncToGoogle(task)
									taskPtr.Tasks = removeTask(taskPtr.Tasks, task)
									if m.cursor >= len(active)+len(completed)-1 {
										m.cursor = len(active) + len(completed) - 2
										if m.cursor < 0 {
											m.cursor = 0
										}
									}
								}
								// Update the current path with the modified parent
								m.currentPath[len(m.currentPath)-1] = *taskPtr
							}
						}
						// Save tasks after deletion
						if err := SaveTasks(m.tasks); err != nil {
							fmt.Printf("Error saving tasks: %v\n", err)
						}
					}
					m.inputActive = false
					m.input.Blur()
					return m, nil
				}
				m.inputActive = false
				m.input.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
		}

		// Handle navigation and shortcuts when input is not active
		switch msg.String() {
		case "down", "j":
			active, completed := m.getCurrentTasks()
			if m.cursor < len(active)+len(completed)-1 {
				m.cursor++
				return m, tea.ClearScreen
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				return m, tea.ClearScreen
			}

		case "right", "l", "enter":
			active, _ := m.getCurrentTasks()
			if m.cursor < len(active) {
				// Always allow entering a task to potentially create subtasks
				if len(m.currentPath) == 0 {
					// If entering a top-level task list, update the currentListID
					m.currentListID = active[m.cursor].Id
				}
				m.currentPath = append(m.currentPath, active[m.cursor])
				m.cursor = 0
			}
			return m, nil

		case "left", "h":
			if len(m.currentPath) > 0 {
				m.currentPath = m.currentPath[:len(m.currentPath)-1]
				m.cursor = 0
				if len(m.currentPath) == 0 {
					// If returning to top level, reset currentListID to first list
					taskLists, err := m.googleTasks.service.Tasklists.List().Do()
					if err == nil && len(taskLists.Items) > 0 {
						m.currentListID = taskLists.Items[0].Id
					}
				} else {
					// If still in a nested list, update currentListID to parent list
					m.currentListID = m.currentPath[0].Id // Always use the top-level list ID
				}
			}
			return m, nil

		case "n":
			m.inputActive = true
			m.inputAction = "new_task"
			m.input.Placeholder = "Enter task title..."
			m.input.SetValue("")
			m.input.Focus()
			return m, nil

		case "r":
			active, completed := m.getCurrentTasks()
			if (m.cursor < len(active) && len(active) > 0) || 
			   (m.cursor >= len(active) && m.cursor-len(active) < len(completed)) {
				m.inputActive = true
				m.inputAction = "rename"
				m.input.Placeholder = ""  // Clear any previous placeholder
				if m.cursor < len(active) {
					m.input.SetValue(active[m.cursor].Title)
				} else {
					completedIdx := m.cursor - len(active)
					m.input.SetValue(completed[completedIdx].Title)
				}
				m.input.Focus()
			}

		case "i":
			active, completed := m.getCurrentTasks()
			if (m.cursor < len(active) && len(active) > 0) || 
			   (m.cursor >= len(active) && m.cursor-len(active) < len(completed)) {
				m.inputActive = true
				m.inputAction = "description"
				m.input.Placeholder = ""  // Clear any previous placeholder
				if m.cursor < len(active) {
					m.input.SetValue(active[m.cursor].Description)
				} else {
					completedIdx := m.cursor - len(active)
					m.input.SetValue(completed[completedIdx].Description)
				}
				m.input.Focus()
				m.input.CursorEnd()
			}

		case "o":
			active, completed := m.getCurrentTasks()
			if (m.cursor < len(active) && len(active) > 0) || 
			   (m.cursor >= len(active) && m.cursor-len(active) < len(completed)) {
				m.inputActive = true
				m.inputAction = "notes"
				m.input.Placeholder = ""  // Clear any previous placeholder
				if m.cursor < len(active) {
					m.input.SetValue(active[m.cursor].Notes)
				} else {
					completedIdx := m.cursor - len(active)
					m.input.SetValue(completed[completedIdx].Notes)
				}
				m.input.Focus()
				m.input.CursorEnd()
			}

		case "t":
			active, completed := m.getCurrentTasks()
			var currentTask *Task
			if m.cursor < len(active) {
				currentTask = &active[m.cursor]
			} else if m.cursor-len(active) < len(completed) {
				currentTask = &completed[m.cursor-len(active)]
			}

			if currentTask != nil {
				m.inputActive = true
				m.inputAction = "due_date"
				m.input.Placeholder = "Format: YYYY-MM-DD HH:mm, YYYY-MM-DD, MM/DD/YYYY, or DD-MM-YYYY"
				
				// Show current due date if it exists
				if !currentTask.DueDate.IsZero() {
					m.input.SetValue(currentTask.DueDate.Format("2006-01-02 15:04"))
					tea.Printf("Current due date: %s", currentTask.DueDate.Format("2006-01-02 15:04"))
				} else {
					m.input.SetValue("")
					tea.Printf("No current due date. Enter in format: YYYY-MM-DD HH:mm, YYYY-MM-DD, MM/DD/YYYY, or DD-MM-YYYY")
				}
				m.input.Focus()
			}

		case "d":
			active, completed := m.getCurrentTasks()
			// Only allow deletion if there are tasks to delete
			if len(active) == 0 && len(completed) == 0 {
				return m, nil
			}

			// Confirm deletion
			m.inputActive = true
			m.inputAction = "delete"
			m.input.Placeholder = "Type 'yes' to confirm deletion"
			m.input.SetValue("")
			m.input.Focus()
			return m, nil

		case " ":
			active, completed := m.getCurrentTasks()
			if len(m.currentPath) == 0 {
				if m.cursor < len(active) {
					// Mark task as completed
					task := active[m.cursor]
					task.Completed = true
					task.Status = "completed"
					m.syncToGoogle(task)
					m.completedTasks = append(m.completedTasks, task)
					m.tasks = removeTask(m.tasks, task)
				} else {
					// Move task back to active
					completedIdx := m.cursor - len(active)
					task := completed[completedIdx]
					task.Completed = false
					task.Status = "needsAction"
					m.syncToGoogle(task)
					m.tasks = append(m.tasks, task)
					m.completedTasks = removeTask(m.completedTasks, task)
				}
				if err := SaveTasks(m.tasks); err != nil {
					fmt.Printf("Error saving tasks: %v\n", err)
				}
			} else {
				// Find and update the actual task in the main task list
				currentTask := &m.tasks
				var taskPtr *Task
				for i, pathTask := range m.currentPath {
					for j := range *currentTask {
						if (*currentTask)[j].Id == pathTask.Id {
							if i == len(m.currentPath)-1 {
								taskPtr = &(*currentTask)[j]
							} else {
								currentTask = &(*currentTask)[j].Tasks
							}
							break
						}
					}
				}
				
				if taskPtr != nil {
					if m.cursor < len(active) {
						// Mark subtask as completed
						task := active[m.cursor]
						task.Completed = true
						task.Status = "completed"
						m.syncToGoogle(task)
						taskPtr.Tasks = removeTask(taskPtr.Tasks, task)
						taskPtr.Tasks = append(taskPtr.Tasks, task)
					} else {
						// Move subtask back to active
						completedIdx := m.cursor - len(active)
						task := completed[completedIdx]
						task.Completed = false
						task.Status = "needsAction"
						m.syncToGoogle(task)
						taskPtr.Tasks = removeTask(taskPtr.Tasks, task)
						taskPtr.Tasks = append(taskPtr.Tasks, task)
					}
					
					// Update current path with latest task data
					m.currentPath[len(m.currentPath)-1] = *taskPtr
					if err := SaveTasks(m.tasks); err != nil {
						fmt.Printf("Error saving tasks: %v\n", err)
					}
				}
			}
			return m, nil

		case "q":
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the UI
func (m model) View() string {
	var s strings.Builder

	// Calculate panel widths based on terminal size
	minMainWidth := 30  // Minimum width for main panel
	minDetailsWidth := 30  // Minimum width for details panel
	padding := 3  // Space between panels

	// Adjust panel widths based on terminal size
	mainPanelWidth := m.width * 2 / 3
	detailsPanelWidth := m.width - mainPanelWidth - padding

	// If terminal is too narrow, switch to full width for main panel
	if m.width < minMainWidth+minDetailsWidth+padding {
		mainPanelWidth = m.width
		detailsPanelWidth = 0
	} else if detailsPanelWidth < minDetailsWidth {
		// Ensure details panel has minimum width if shown
		detailsPanelWidth = minDetailsWidth
		mainPanelWidth = m.width - minDetailsWidth - padding
	}

	// Build main task list panel
	var mainPanel strings.Builder
	if len(m.currentPath) > 0 {
		// Show breadcrumb
		path := "Main"
		for _, task := range m.currentPath {
			path += " > " + task.Title
		}
		mainPanel.WriteString(path + "\n\n")
	}

	active, completed := m.getCurrentTasks()
	if len(active) == 0 && len(completed) == 0 && !m.inputActive {
		// Show hint message when no tasks exist
		hint := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("No tasks yet! Press 'n' to create a new task")
		mainPanel.WriteString("\n" + hint + "\n")
	}

	if m.inputActive {
		if m.inputAction == "due_date" {
			var oldDate string
			if m.cursor >= 0 && m.cursor < len(active) {
				oldDate = active[m.cursor].DueDate.Format("2006-01-02 15:04")
			}
			if oldDate != "" {
				mainPanel.WriteString("Current due date: " + oldDate + "\n")
			}
			mainPanel.WriteString("Enter due date (YYYY-MM-DD HH:mm, YYYY-MM-DD, MM/DD/YYYY, or DD-MM-YYYY): \n" + m.input.View() + "\n\n")
		} else {
			mainPanel.WriteString("Enter " + m.inputAction + ": " + m.input.View() + "\n\n")
		}
	} else {
		// Calculate available height for tasks
		headerHeight := len(strings.Split(mainPanel.String(), "\n"))
		footerHeight := 2 // For potential scroll indicators
		availableHeight := m.height - headerHeight - footerHeight

		// Calculate total tasks
		totalTasks := len(active) + len(completed)

		// Calculate visible window
		startIdx := 0
		if m.cursor >= availableHeight {
			startIdx = m.cursor - (availableHeight / 2)
		}
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + availableHeight
		if endIdx > totalTasks {
			endIdx = totalTasks
			// Adjust startIdx to show maximum possible tasks
			if totalTasks > availableHeight {
				startIdx = totalTasks - availableHeight
			}
		}

		// Show active tasks
		mainPanel.WriteString("Tasks:\n\n")
		for i, task := range active {
			if i >= startIdx && i < endIdx {
				cursor := " "
				if m.cursor == i {
					cursor = ">"
				}
				taskTitle := task.Title
				if len(task.Tasks) > 0 {
					taskTitle += " ▶"
				}
				if m.cursor == i {
					taskTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(taskTitle)
				}
				mainPanel.WriteString(fmt.Sprintf("%s %s\n", cursor, taskTitle))
			}
		}

		// Show completed tasks if any
		if len(completed) > 0 {
			completedStartIdx := len(active)
			if completedStartIdx >= startIdx && completedStartIdx < endIdx {
				mainPanel.WriteString("\nCompleted Tasks:\n\n")
			}
			for i, task := range completed {
				globalIdx := len(active) + i
				if globalIdx >= startIdx && globalIdx < endIdx {
					cursor := " "
					if m.cursor == globalIdx {
						cursor = ">"
					}
					taskTitle := task.Title
					if len(task.Tasks) > 0 {
						taskTitle += " ▶"
					}
					style := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
					if m.cursor == globalIdx {
						style = style.Foreground(lipgloss.Color("86"))
					}
					mainPanel.WriteString(fmt.Sprintf("%s %s\n", cursor, style.Render(taskTitle)))
				}
			}
		}

		// Add scroll indicators if needed
		if startIdx > 0 {
			mainPanel.WriteString("\n↑ More tasks above")
		}
		if endIdx < totalTasks {
			mainPanel.WriteString("\n↓ More tasks below")
		}
	}

	// Build details panel if there's space
	var detailsPanel strings.Builder
	if detailsPanelWidth > 0 {
		detailsPanel.WriteString("Task Details\n\n")

		// Get the currently selected task
		var selectedTask *Task
		active, completed := m.getCurrentTasks()
		if m.cursor < len(active) && len(active) > 0 {
			selectedTask = &active[m.cursor]
		} else if len(completed) > 0 && m.cursor-len(active) < len(completed) {
			selectedTask = &completed[m.cursor-len(active)]
		}

		if selectedTask != nil {
			// Function to wrap text to fit panel width
			wrapText := func(text string) string {
				if text == "" {
					return text
				}
				words := strings.Fields(text)
				var lines []string
				currentLine := words[0]
				spaceLeft := detailsPanelWidth - 4 // Account for padding and borders
				
				for _, word := range words[1:] {
					if len(currentLine)+1+len(word) <= spaceLeft {
						currentLine += " " + word
					} else {
						lines = append(lines, currentLine)
						currentLine = word
					}
				}
				lines = append(lines, currentLine)
				return strings.Join(lines, "\n")
			}

			// Show task details with text wrapping
			detailsPanel.WriteString("Title: " + wrapText(selectedTask.Title) + "\n\n")

			detailsPanel.WriteString("Description: \n")
			if selectedTask.Description == "" {
				detailsPanel.WriteString("(Press 'i' to add description)\n")
			} else {
				detailsPanel.WriteString(wrapText(selectedTask.Description) + "\n")
			}
			detailsPanel.WriteString("\n")

			detailsPanel.WriteString("Notes: \n")
			if selectedTask.Notes == "" {
				detailsPanel.WriteString("(Press 'o' to add notes)\n")
			} else {
				detailsPanel.WriteString(wrapText(selectedTask.Notes) + "\n")
			}
			detailsPanel.WriteString("\n")

			detailsPanel.WriteString("Created: " + selectedTask.CreatedAt.Format("2006-01-02 15:04") + "\n")
			
			detailsPanel.WriteString("Due Date: ")
			if selectedTask.DueDate.IsZero() {
				detailsPanel.WriteString("(Press 't' to set due date)\n")
			} else {
				detailsPanel.WriteString(selectedTask.DueDate.Format("2006-01-02 15:04") + "\n")
			}

			// Add keyboard shortcuts at the bottom if there's space
			if m.height > 20 {
				detailsPanel.WriteString("\n\nKeyboard Shortcuts:\n")
				detailsPanel.WriteString("n: New task    d: Delete\n")
				detailsPanel.WriteString("r: Rename      i: Edit description\n")
				detailsPanel.WriteString("o: Edit notes  t: Set due date\n")
				detailsPanel.WriteString("Enter: Toggle completion\n")
				detailsPanel.WriteString("←/h: Back      →/l: Enter sublist\n")
			}
		} else {
			detailsPanel.WriteString("No task selected")
		}
	}

	// Combine panels with border
	mainPanelStr := lipgloss.NewStyle().
		Width(mainPanelWidth).
		Render(mainPanel.String())

	if detailsPanelWidth > 0 {
		detailsPanelStr := lipgloss.NewStyle().
			Width(detailsPanelWidth).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("12")).
			Padding(1).
			Render(detailsPanel.String())

		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, mainPanelStr, "  ", detailsPanelStr))
	} else {
		s.WriteString(mainPanelStr)
	}

	return s.String()
}

// removeTask removes a task from a list of tasks
func removeTask(tasks []Task, task Task) []Task {
	for i, t := range tasks {
		if t.Id == task.Id {
			return append(tasks[:i], tasks[i+1:]...)
		}
	}
	return tasks
}

// splitTasks splits tasks into active and completed tasks
func splitTasks(tasks []Task) ([]Task, []Task) {
	active := make([]Task, 0)
	completed := make([]Task, 0)

	for _, task := range tasks {
		if task.Completed {
			completed = append(completed, task)
		} else {
			active = append(active, task)
		}
	}

	return active, completed
}

// generateID creates a unique ID for tasks
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// UpdateTasks updates the task list and refreshes the UI
func (m *model) UpdateTasks(tasks []Task) {
	select {
	case m.updateChan <- tasks:
		// Task update sent successfully
		if m.googleTasks != nil {
			// Sync all tasks to Google
			go func() {
				err := ExportToGoogle(tasks)
				if err != nil {
					tea.Println("Error syncing with Google Tasks:", err)
				}
			}()
		}
	default:
		fmt.Println("Warning: Update channel full, skipping update")
	}
}

// syncToGoogle synchronizes local changes to Google Tasks
func (m *model) syncToGoogle(task Task) {
	if m.googleTasks == nil {
		return
	}

	go func() {
		var err error
		switch task.Status {
		case "needsAction":
			if task.Id == "" {
				// New task
				_, err = m.googleTasks.CreateTask(task, m.currentListID)
			} else {
				// Updated task
				err = m.googleTasks.UpdateTask(task)
			}
		case "completed":
			err = m.googleTasks.UpdateTask(task)
		case "deleted":
			err = m.googleTasks.DeleteTask(task.Id)
		}

		if err != nil {
			tea.Println("Error syncing with Google Tasks:", err)
		}

		// After individual task sync, sync all tasks to ensure consistency
		if err := ExportToGoogle(m.tasks); err != nil {
			tea.Println("Error syncing all tasks with Google:", err)
		}
	}()
}

// RunTaskUI starts the Bubble Tea program
func RunTaskUI(tasks []Task, client *GoogleTasksClient) {
	m := NewModel(tasks, client)
	p := tea.NewProgram(m)
	if err := p.Start(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
