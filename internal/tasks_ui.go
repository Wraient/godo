package internal

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// Task represents a task or subtask
type Task struct {
	ID        string
	Title     string
	Notes     string
	Completed bool
	DueDate   time.Time
	ParentID  string
	Subtasks  []Task
}

// Model represents the state of our Bubble Tea program
type model struct {
	activeTasks    []Task
	completedTasks []Task
	cursor         int    // Tracks which task the user is on in the active list
	subtaskCursor  int    // Tracks which subtask the user is on
	viewingSubtasks bool  // Whether the user is currently viewing subtasks
	selectedID     string
	input          textinput.Model // For adding/renaming tasks
	inputActive    bool            // Whether we are currently adding or renaming tasks
	inputAction    string          // Type of action: "new", "subtask", "rename"
	deletedTaskID  string          // ID of the last deleted task (could be used for undo)
}

// NewModel initializes the Bubble Tea model with tasks
func NewModel(tasks []Task) model {
	ti := textinput.New()
	ti.Placeholder = "Enter task name..."
	
	// Split initial tasks into active and completed
	var activeTasks, completedTasks []Task
	for _, task := range tasks {
		if task.Completed {
			completedTasks = append(completedTasks, task)
		} else {
			activeTasks = append(activeTasks, task)
		}
	}
	
	return model{
		activeTasks: activeTasks,
		completedTasks: completedTasks,
		cursor: 0,
		selectedID: "",
		input: ti,
		inputActive: false,
	}
}

// Init starts the program
func (m model) Init() tea.Cmd {
	return nil
}

// Update handles keypresses and updates the state of the UI
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "down", "j":
			if !m.inputActive {
				if m.viewingSubtasks {
					if m.subtaskCursor < len(m.activeTasks[m.cursor].Subtasks)-1 {
						m.subtaskCursor++
					}
				} else if m.cursor < len(m.activeTasks)-1 {
					m.cursor++
				} else if m.cursor == len(m.activeTasks)-1 && len(m.completedTasks) > 0 {
					// Move to completed tasks section
					m.cursor = len(m.activeTasks)
				} else if m.cursor >= len(m.activeTasks) && m.cursor < len(m.activeTasks)+len(m.completedTasks)-1 {
					// Navigate within completed tasks
					m.cursor++
				}
			} else {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}

		case "up", "k":
			if !m.inputActive {
				if m.viewingSubtasks {
					if m.subtaskCursor > 0 {
						m.subtaskCursor--
					}
				} else if m.cursor > len(m.activeTasks) {
					// Navigate within completed tasks
					m.cursor--
				} else if m.cursor == len(m.activeTasks) {
					// Move back to active tasks section
					m.cursor = len(m.activeTasks) - 1
				} else if m.cursor > 0 {
					m.cursor--
				}
			} else {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}

		case "enter":
			if !m.inputActive {
				if m.viewingSubtasks {
					m.activeTasks[m.cursor].Subtasks[m.subtaskCursor].Completed = !m.activeTasks[m.cursor].Subtasks[m.subtaskCursor].Completed
				} else if m.cursor < len(m.activeTasks) && len(m.activeTasks[m.cursor].Subtasks) > 0 {
					m.viewingSubtasks = true
					m.subtaskCursor = 0
				} else if m.cursor < len(m.activeTasks) {
					// Complete an active task
					task := m.activeTasks[m.cursor]
					task.Completed = true
					m.completedTasks = append(m.completedTasks, task)
					m.activeTasks = append(m.activeTasks[:m.cursor], m.activeTasks[m.cursor+1:]...)
					if m.cursor >= len(m.activeTasks) && m.cursor > 0 {
						m.cursor--
					}
				} else {
					// Uncomplete a completed task
					completedIdx := m.cursor - len(m.activeTasks)
					task := m.completedTasks[completedIdx]
					task.Completed = false
					m.activeTasks = append(m.activeTasks, task)
					m.completedTasks = append(m.completedTasks[:completedIdx], m.completedTasks[completedIdx+1:]...)
					if m.cursor >= len(m.activeTasks)+len(m.completedTasks) {
						m.cursor--
					}
				}
			} else {
				// Handle submission of text input
				taskTitle := m.input.Value()
				switch m.inputAction {
				case "new":
					newTask := Task{ID: generateID(), Title: taskTitle, Completed: false, DueDate: time.Now()}
					m.activeTasks = append(m.activeTasks, newTask)
					m.cursor = len(m.activeTasks) - 1
				case "subtask":
					newSubtask := Task{ID: generateID(), Title: taskTitle, Completed: false, DueDate: time.Now(), ParentID: m.activeTasks[m.cursor].ID}
					m.activeTasks[m.cursor].Subtasks = append(m.activeTasks[m.cursor].Subtasks, newSubtask)
					m.subtaskCursor = len(m.activeTasks[m.cursor].Subtasks) - 1
				case "rename":
					if m.viewingSubtasks {
						m.activeTasks[m.cursor].Subtasks[m.subtaskCursor].Title = taskTitle
					} else if m.cursor < len(m.activeTasks) {
						m.activeTasks[m.cursor].Title = taskTitle
					} else {
						completedIdx := m.cursor - len(m.activeTasks)
						m.completedTasks[completedIdx].Title = taskTitle
					}
				}
				m.inputActive = false
				m.input.Blur()
			}

		case "n":
			if !m.inputActive {
				m.inputActive = true
				m.inputAction = "new"
				m.input.SetValue("")
				m.input.Focus()
			}

		case "s":
			if !m.inputActive {
				m.inputActive = true
				m.inputAction = "subtask"
				m.input.SetValue("")
				m.input.Focus()
			}

		case "r":
			if !m.inputActive {
				m.inputActive = true
				m.inputAction = "rename"
				if m.cursor < len(m.activeTasks) {
					m.input.SetValue(m.activeTasks[m.cursor].Title)
				} else {
					completedIdx := m.cursor - len(m.activeTasks)
					m.input.SetValue(m.completedTasks[completedIdx].Title)
				}
				m.input.Focus()
			}

		case "d":
			if !m.inputActive {
				if m.cursor < len(m.activeTasks) {
					// Delete from active tasks
					m.deletedTaskID = m.activeTasks[m.cursor].ID
					m.activeTasks = append(m.activeTasks[:m.cursor], m.activeTasks[m.cursor+1:]...)
					if m.cursor >= len(m.activeTasks) && m.cursor > 0 {
						m.cursor--
					}
				} else if len(m.completedTasks) > 0 {
					// Delete from completed tasks
					completedIdx := m.cursor - len(m.activeTasks)
					m.deletedTaskID = m.completedTasks[completedIdx].ID
					m.completedTasks = append(m.completedTasks[:completedIdx], m.completedTasks[completedIdx+1:]...)
					if m.cursor >= len(m.activeTasks)+len(m.completedTasks) {
						m.cursor--
					}
				}
			}

		case "esc":
			if m.inputActive {
				m.inputActive = false
				m.input.Blur()
			} else if m.viewingSubtasks {
				m.viewingSubtasks = false
				m.subtaskCursor = 0
			} else {
				return m, tea.Quit
			}

		default:
			if m.inputActive {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
		}
	}
	return m, nil
}

// View renders the UI
func (m model) View() string {
	var s string

	// Title
	s += lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33")).Render("Task List") + "\n\n"

	// Render active tasks
	for i, task := range m.activeTasks {
		cursor := " " // No cursor
		if m.cursor == i {
			cursor = ">" // Cursor for the current selection
		}

		taskStyle := lipgloss.NewStyle().PaddingLeft(2)
		if m.cursor == i {
			taskStyle = taskStyle.Bold(true).Foreground(lipgloss.Color("36"))
		}

		s += fmt.Sprintf("%s %s\n", cursor, taskStyle.Render(task.Title))

		// Display subtasks
		for _, subtask := range task.Subtasks {
			subtaskStyle := lipgloss.NewStyle().PaddingLeft(4)
			if subtask.Completed {
				subtaskStyle = subtaskStyle.Foreground(lipgloss.Color("240")) // Dimmed color for completed subtasks
			}
			s += fmt.Sprintf("    %s\n", subtaskStyle.Render(subtask.Title))
		}
	}

	// Render completed tasks
	if len(m.completedTasks) > 0 {
		s += "\n" + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("31")).Render("Completed Tasks") + "\n\n"
		for i, task := range m.completedTasks {
			cursor := " "
			if m.cursor == len(m.activeTasks)+i {
				cursor = ">"
			}

			taskStyle := lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("240")) // Dimmed color for completed tasks
			if m.cursor == len(m.activeTasks)+i {
				taskStyle = taskStyle.Bold(true).Foreground(lipgloss.Color("36"))
			}

			s += fmt.Sprintf("%s %s\n", cursor, taskStyle.Render(task.Title))
		}
	}

	// Input field
	if m.inputActive {
		s += "\n" + m.input.View() + "\n"
	} else {
		s += "\nPress 'n' to add a task, 's' for a subtask, 'r' to rename, 'd' to delete, and 'esc' to exit.\n"
	}

	return s
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
