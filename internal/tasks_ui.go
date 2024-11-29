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
	ID          string
	Title       string
	Description string
	Notes       string
	Completed   bool
	CreatedAt   time.Time
	DueDate     time.Time
	Tasks       []Task
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
}

// NewModel initializes the Bubble Tea model with tasks
func NewModel(tasks []Task) model {
	ti := textinput.New()
	ti.Placeholder = "Enter task title..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	// Get terminal dimensions
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))

	return model{
		tasks:       tasks,
		input:       ti,
		inputActive: false,
		width:       width,
		height:      height,
	}
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
	m.input = textinput.New()
	m.input.Focus()
	m.updateTerminalSize()  // Get initial terminal size
	return nil
}

// Update handles keypresses and updates the state of the UI
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, tea.ClearScreen // Clear screen when window size changes

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
							} else {
								active[m.cursor].Notes = m.input.Value()
							}
						} else {
							completedIdx := m.cursor - len(active)
							if m.inputAction == "description" {
								completed[completedIdx].Description = m.input.Value()
							} else {
								completed[completedIdx].Notes = m.input.Value()
							}
						}
					} else {
						parentTask := &m.currentPath[len(m.currentPath)-1]
						if m.cursor < len(parentTask.Tasks) {
							if m.inputAction == "description" {
								parentTask.Tasks[m.cursor].Description = m.input.Value()
							} else {
								parentTask.Tasks[m.cursor].Notes = m.input.Value()
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
						} else {
							completedIdx := m.cursor - len(active)
							completed[completedIdx].Title = m.input.Value()
						}
					} else {
						parentTask := &m.currentPath[len(m.currentPath)-1]
						if m.cursor < len(parentTask.Tasks) {
							parentTask.Tasks[m.cursor].Title = m.input.Value()
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
					dueDate, err := time.Parse("2006-01-02 15:04", dateStr)
					if err == nil {
						if len(m.currentPath) == 0 {
							if m.cursor < len(active) {
								active[m.cursor].DueDate = dueDate
							} else {
								completedIdx := m.cursor - len(active)
								completed[completedIdx].DueDate = dueDate
							}
						} else {
							parentTask := &m.currentPath[len(m.currentPath)-1]
							if m.cursor < len(parentTask.Tasks) {
								parentTask.Tasks[m.cursor].DueDate = dueDate
							}
						}
						if err := SaveTasks(m.tasks); err != nil {
							fmt.Printf("Error saving tasks: %v\n", err)
						}
					}
				case "new_task":
					newTask := Task{
						ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
						Title:     m.input.Value(),
						CreatedAt: time.Now(),
					}
					if len(m.currentPath) == 0 {
						m.tasks = append(m.tasks, newTask)
						m.cursor = len(active)
					} else {
						// Find the actual task in the main task list
						currentTask := &m.tasks
						var taskPtr *Task
						for i, pathTask := range m.currentPath {
							for j := range *currentTask {
								if (*currentTask)[j].ID == pathTask.ID {
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
							taskPtr.Tasks = append(taskPtr.Tasks, newTask)
							m.cursor = len(taskPtr.Tasks) - 1
							m.currentPath[len(m.currentPath)-1] = *taskPtr
						}
					}
					if err := SaveTasks(m.tasks); err != nil {
						fmt.Printf("Error saving tasks: %v\n", err)
					}
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
			m.updateTerminalSize()  // Update size on cursor movement
			active, completed := m.getCurrentTasks()
			if m.cursor < len(active)+len(completed)-1 {
				m.cursor++
				return m, tea.ClearScreen
			}

		case "up", "k":
			m.updateTerminalSize()  // Update size on cursor movement
			if m.cursor > 0 {
				m.cursor--
				return m, tea.ClearScreen
			}

		case "right", "l":
			active, _ := m.getCurrentTasks()
			if m.cursor < len(active) {
				task := &active[m.cursor]
				m.currentPath = append(m.currentPath, *task)
				m.cursor = 0
			}

		case "left", "h":
			if len(m.currentPath) > 0 {
				m.cursor = 0
				m.currentPath = m.currentPath[:len(m.currentPath)-1]
			}

		case "enter":
			active, completed := m.getCurrentTasks()
			if len(m.currentPath) == 0 {
				if m.cursor < len(active) {
					// Mark task as completed
					task := active[m.cursor]
					task.Completed = true
					m.completedTasks = append(m.completedTasks, task)
					m.tasks = removeTask(m.tasks, task)
				} else {
					// Move task back to active
					completedIdx := m.cursor - len(active)
					task := completed[completedIdx]
					task.Completed = false
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
						if (*currentTask)[j].ID == pathTask.ID {
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
						taskPtr.Tasks = removeTask(taskPtr.Tasks, task)
						taskPtr.Tasks = append(taskPtr.Tasks, task)
					} else {
						// Move subtask back to active
						completedIdx := m.cursor - len(active)
						task := completed[completedIdx]
						task.Completed = false
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
			if (m.cursor < len(active) && len(active) > 0) || 
			   (m.cursor >= len(active) && m.cursor-len(active) < len(completed)) {
				m.inputActive = true
				m.inputAction = "due_date"
				m.input.Placeholder = "YYYY-MM-DD HH:MM"
				
				// Get the existing due date if any
				var existingDate time.Time
				if m.cursor < len(active) {
					existingDate = active[m.cursor].DueDate
				} else {
					completedIdx := m.cursor - len(active)
					existingDate = completed[completedIdx].DueDate
				}
				
				if !existingDate.IsZero() {
					m.input.SetValue(existingDate.Format("2006-01-02 15:04"))
				} else {
					m.input.SetValue("")
				}
				m.input.Focus()
			}

		case "d":
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
						if (*currentTask)[j].ID == pathTask.ID {
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
						taskPtr.Tasks = removeTask(taskPtr.Tasks, task)
						if m.cursor >= len(active)+len(completed)-1 {
							m.cursor = len(active) + len(completed) - 2
							if m.cursor < 0 {
								m.cursor = 0
							}
						}
					}
					// Update current path with latest task data
					m.currentPath[len(m.currentPath)-1] = *taskPtr
				}
			}
			// Save tasks after deletion
			if err := SaveTasks(m.tasks); err != nil {
				fmt.Printf("Error saving tasks: %v\n", err)
			}
			return m, tea.ClearScreen

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
			input := m.input.Value()
			format := "YYYY-MM-DD HH:MM"
			
			// Get the existing due date if any
			var oldDate string
			active, completed := m.getCurrentTasks()
			if m.cursor < len(active) && !active[m.cursor].DueDate.IsZero() {
				oldDate = active[m.cursor].DueDate.Format("2006-01-02 15:04")
			} else if m.cursor >= len(active) && m.cursor-len(active) < len(completed) {
				completedIdx := m.cursor - len(active)
				if !completed[completedIdx].DueDate.IsZero() {
					oldDate = completed[completedIdx].DueDate.Format("2006-01-02 15:04")
				}
			}
			
			if len(input) > 0 {
				// Replace the format characters with actual input where available
				if len(input) >= 4 {
					format = input[:4] + format[4:]
				}
				if len(input) >= 7 {
					format = input[:7] + format[7:]
				}
				if len(input) >= 10 {
					format = input[:10] + format[10:]
				}
				if len(input) >= 13 {
					format = input[:13] + format[13:]
				}
				if len(input) >= 16 {
					format = input
				}
			}
			
			if oldDate != "" {
				mainPanel.WriteString("Current due date: " + oldDate + "\n")
			}
			mainPanel.WriteString("Enter due date > " + format + "\n" + m.input.View() + "\n\n")
		} else {
			mainPanel.WriteString("Enter " + m.inputAction + ": " + m.input.View() + "\n\n")
		}
	} else {
		// Get current level's tasks
		active, completed := m.getCurrentTasks()

		// Show active tasks
		mainPanel.WriteString("Tasks:\n\n")
		for i, task := range active {
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

		// Show completed tasks if any
		if len(completed) > 0 {
			mainPanel.WriteString("\nCompleted Tasks:\n\n")
			for i, task := range completed {
				cursor := " "
				if m.cursor == len(active)+i {
					cursor = ">"
				}
				taskTitle := task.Title
				if len(task.Tasks) > 0 {
					taskTitle += " ▶"
				}
				style := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
				if m.cursor == len(active)+i {
					style = style.Foreground(lipgloss.Color("86"))
				}
				mainPanel.WriteString(fmt.Sprintf("%s %s\n", cursor, style.Render(taskTitle)))
			}
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
		if t.ID == task.ID {
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

// RunTaskUI starts the Bubble Tea program
func RunTaskUI(tasks []Task) {
	p := tea.NewProgram(NewModel(tasks))
	if err := p.Start(); err != nil {
		fmt.Printf("Error: %v", err)
	}
}
