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
	tasks          []Task
	cursor         int // Tracks which task the user is on in the main list
	subtaskCursor  int // Tracks which subtask the user is on
	viewingSubtasks bool // Whether the user is currently viewing subtasks
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
	return model{tasks: tasks, cursor: 0, selectedID: "", input: ti, inputActive: false}
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

		// Move cursor down
		case "down", "j":
			if !m.inputActive {
				if m.viewingSubtasks {
					// Navigate through subtasks
					if m.subtaskCursor < len(m.tasks[m.cursor].Subtasks)-1 {
						m.subtaskCursor++
					}
				} else if m.cursor < len(m.tasks)-1 {
					// Navigate through tasks
					m.cursor++
				}
			} else {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}

		// Move cursor up
		case "up", "k":
			if !m.inputActive {
				if m.viewingSubtasks {
					// Navigate through subtasks
					if m.subtaskCursor > 0 {
						m.subtaskCursor--
					}
				} else if m.cursor > 0 {
					// Navigate through tasks
					m.cursor--
				}
			} else {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}

		// Enter subtask view
		case "enter":
			if !m.inputActive {
				
				if m.viewingSubtasks {
					// Toggle completion of the subtask
					m.tasks[m.cursor].Subtasks[m.subtaskCursor].Completed = !m.tasks[m.cursor].Subtasks[m.subtaskCursor].Completed
				} else if len(m.tasks[m.cursor].Subtasks) > 0 {
					// Enter subtask view
					m.viewingSubtasks = true
					m.subtaskCursor = 0
				} else {
					// Toggle completion of the task
					m.tasks[m.cursor].Completed = !m.tasks[m.cursor].Completed
				}
			} else if m.inputActive {
				// Handle submission of text input
				taskTitle := m.input.Value()
				switch m.inputAction {
				case "new":
					newTask := Task{ID: generateID(), Title: taskTitle, Completed: false, DueDate: time.Now()}
					m.tasks = append(m.tasks, newTask)
				case "subtask":
					newSubtask := Task{ID: generateID(), Title: taskTitle, Completed: false, DueDate: time.Now(), ParentID: m.tasks[m.cursor].ID}
					m.tasks[m.cursor].Subtasks = append(m.tasks[m.cursor].Subtasks, newSubtask)
				case "rename":
					if m.viewingSubtasks {
						m.tasks[m.cursor].Subtasks[m.subtaskCursor].Title = taskTitle
					} else {
						m.tasks[m.cursor].Title = taskTitle
					}
				}
				m.inputActive = false
				m.input.Blur()
			}

			// Add a new task
		case "n":
			if !m.inputActive {
				m.inputActive = true
				m.inputAction = "new"
				m.input.SetValue("")
				m.input.Focus()
			} else {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}

		// Add a subtask
		case "s":
			if !m.inputActive {
				m.inputActive = true
				m.inputAction = "subtask"
				m.input.SetValue("")
				m.input.Focus()
			} else {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}

		// Rename task
		case "r":
			if !m.inputActive {
				m.inputActive = true
				m.inputAction = "rename"
				m.input.SetValue(m.tasks[m.cursor].Title)
				m.input.Focus()
			} else {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}

		// Delete task
		case "d":
			if !m.inputActive {
				m.deletedTaskID = m.tasks[m.cursor].ID
				m.tasks = append(m.tasks[:m.cursor], m.tasks[m.cursor+1:]...)
				if m.cursor > 0 {
					m.cursor--
				}
			} else {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}


		// Exit subtask view
		case "esc":
			if m.inputActive {
				m.inputActive = false
				m.input.Blur()
			} else if m.viewingSubtasks {
				// Exit subtask view
				m.viewingSubtasks = false
				m.subtaskCursor = 0
			} else {
				return m, tea.Quit
			}

		// Handle input text update when adding/renaming tasks
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

	// Render tasks or subtasks depending on the view state
	if m.viewingSubtasks {
		s += lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33")).Render("Subtasks") + "\n\n"
		for i, subtask := range m.tasks[m.cursor].Subtasks {
			cursor := " " // No cursor
			if m.subtaskCursor == i {
				cursor = ">" // Cursor for the current selection
			}

			subtaskStyle := lipgloss.NewStyle().PaddingLeft(2)
			if m.subtaskCursor == i {
				subtaskStyle = subtaskStyle.Bold(true).Foreground(lipgloss.Color("36"))
			}

			s += fmt.Sprintf("%s %s\n", cursor, subtaskStyle.Render(subtask.Title))
		}
	} else {
		// Render main tasks
		for i, task := range m.tasks {
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
				subtaskStyle := lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("244"))
				s += fmt.Sprintf("    %s\n", subtaskStyle.Render(subtask.Title))
			}
		}
	}

	// Handle input prompt if active
	if m.inputActive {
		s += "\n" + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33")).Render("Enter Task Name") + "\n"
		s += m.input.View()
	}

	// Help
	s += "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("↑/↓ to navigate, enter to mark complete, n to add task, s to add subtask, r to rename, d to delete, esc to cancel, q to quit.")

	return s
}

// generateID generates a unique ID for tasks (for simplicity, using current timestamp)
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
