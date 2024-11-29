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
	Tasks     []Task // Subtasks within this task
}

// Model represents the state of our Bubble Tea program
type model struct {
	tasks          []Task
	completedTasks []Task
	cursor         int
	currentPath    []int // Tracks the current task hierarchy path
	input          textinput.Model
	inputActive    bool
	inputAction    string
	deletedTaskID  string
}

// NewModel initializes the Bubble Tea model with tasks
func NewModel(tasks []Task) model {
	ti := textinput.New()
	ti.Placeholder = "Enter task name..."
	
	// Split initial tasks into active and completed for the root level
	var activeTasks, completedTasks []Task
	for _, task := range tasks {
		if task.Completed {
			completedTasks = append(completedTasks, task)
		} else {
			activeTasks = append(activeTasks, task)
		}
	}
	
	return model{
		tasks: activeTasks,
		completedTasks: completedTasks,
		cursor: 0,
		currentPath: []int{},
		input: ti,
		inputActive: false,
	}
}

// getCurrentTasks returns the current level's tasks based on currentPath
func (m *model) getCurrentTasks() ([]Task, []Task) {
	if len(m.currentPath) == 0 {
		return m.tasks, m.completedTasks
	}

	parentTask := &m.tasks[m.currentPath[0]]
	for i := 1; i < len(m.currentPath); i++ {
		parentTask = &parentTask.Tasks[m.currentPath[i]]
	}

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
				active, completed := m.getCurrentTasks()
				if m.cursor < len(active)-1 {
					m.cursor++
				} else if m.cursor == len(active)-1 && len(completed) > 0 {
					m.cursor = len(active)
				} else if m.cursor >= len(active) && m.cursor < len(active)+len(completed)-1 {
					m.cursor++
				}
			} else {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}

		case "up", "k":
			if !m.inputActive {
				active, _ := m.getCurrentTasks()
				if m.cursor > len(active) {
					m.cursor--
				} else if m.cursor == len(active) {
					m.cursor = len(active) - 1
				} else if m.cursor > 0 {
					m.cursor--
				}
			} else {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}

		case "right", "l":
			if !m.inputActive {
				active, _ := m.getCurrentTasks()
				if m.cursor < len(active) {
					// Enter the selected task's sublist
					m.currentPath = append(m.currentPath, m.cursor)
					m.cursor = 0
				}
			}

		case "left", "h":
			if !m.inputActive && len(m.currentPath) > 0 {
				// Go back to parent list
				m.cursor = m.currentPath[len(m.currentPath)-1]
				m.currentPath = m.currentPath[:len(m.currentPath)-1]
			}

		case "enter":
			if !m.inputActive {
				active, _ := m.getCurrentTasks()
				if len(m.currentPath) == 0 {
					if m.cursor < len(active) {
						task := active[m.cursor]
						task.Completed = true
						m.completedTasks = append(m.completedTasks, task)
						m.tasks = append(m.tasks[:m.cursor], m.tasks[m.cursor+1:]...)
					} else {
						completedIdx := m.cursor - len(active)
						task := m.completedTasks[completedIdx]
						task.Completed = false
						m.tasks = append(m.tasks, task)
						m.completedTasks = append(m.completedTasks[:completedIdx], m.completedTasks[completedIdx+1:]...)
					}
				} else {
					parentTask := &m.tasks[m.currentPath[0]]
					for i := 1; i < len(m.currentPath); i++ {
						parentTask = &parentTask.Tasks[m.currentPath[i]]
					}

					var activeTasks []Task
					var completedTasks []Task

					for _, task := range parentTask.Tasks {
						taskCopy := task
						if task.Completed {
							completedTasks = append(completedTasks, taskCopy)
						} else {
							activeTasks = append(activeTasks, taskCopy)
						}
					}

					if m.cursor < len(active) {
						task := activeTasks[m.cursor]
						task.Completed = true
						completedTasks = append(completedTasks, task)
						activeTasks = append(activeTasks[:m.cursor], activeTasks[m.cursor+1:]...)
					} else {
						completedIdx := m.cursor - len(active)
						task := completedTasks[completedIdx]
						task.Completed = false
						activeTasks = append(activeTasks, task)
						completedTasks = append(completedTasks[:completedIdx], completedTasks[completedIdx+1:]...)
					}

					parentTask.Tasks = make([]Task, 0, len(activeTasks)+len(completedTasks))
					parentTask.Tasks = append(parentTask.Tasks, activeTasks...)
					parentTask.Tasks = append(parentTask.Tasks, completedTasks...)

					if m.cursor >= len(activeTasks)+len(completedTasks) && m.cursor > 0 {
						m.cursor--
					}
				}

				if m.cursor >= len(active)-1 && m.cursor > 0 {
					m.cursor--
				}
			} else {
				taskTitle := m.input.Value()
				switch m.inputAction {
				case "new":
					newTask := Task{
						ID:        generateID(),
						Title:     taskTitle,
						Completed: false,
						DueDate:   time.Now(),
						Tasks:     []Task{},
					}

					if len(m.currentPath) == 0 {
						m.tasks = append(m.tasks, newTask)
						m.cursor = len(m.tasks) - 1
					} else {
						parentTask := &m.tasks[m.currentPath[0]]
						for i := 1; i < len(m.currentPath); i++ {
							parentTask = &parentTask.Tasks[m.currentPath[i]]
						}

						var activeTasks []Task
						var completedTasks []Task

						for _, task := range parentTask.Tasks {
							taskCopy := task
							if task.Completed {
								completedTasks = append(completedTasks, taskCopy)
							} else {
								activeTasks = append(activeTasks, taskCopy)
							}
						}

						activeTasks = append(activeTasks, newTask)

						parentTask.Tasks = make([]Task, 0, len(activeTasks)+len(completedTasks))
						parentTask.Tasks = append(parentTask.Tasks, activeTasks...)
						parentTask.Tasks = append(parentTask.Tasks, completedTasks...)

						m.cursor = len(activeTasks) - 1
					}
				case "rename":
					active, _ := m.getCurrentTasks()
					if len(m.currentPath) == 0 {
						if m.cursor < len(active) {
							active[m.cursor].Title = taskTitle
						} else {
							completedIdx := m.cursor - len(active)
							m.completedTasks[completedIdx].Title = taskTitle
						}
					} else {
						parentTask := &m.tasks[m.currentPath[0]]
						for i := 1; i < len(m.currentPath); i++ {
							parentTask = &parentTask.Tasks[m.currentPath[i]]
						}

						var activeTasks []Task
						var completedTasks []Task

						for _, task := range parentTask.Tasks {
							taskCopy := task
							if task.Completed {
								completedTasks = append(completedTasks, taskCopy)
							} else {
								activeTasks = append(activeTasks, taskCopy)
							}
						}

						if m.cursor < len(active) {
							activeTasks[m.cursor].Title = taskTitle
						} else {
							completedIdx := m.cursor - len(active)
							completedTasks[completedIdx].Title = taskTitle
						}

						parentTask.Tasks = make([]Task, 0, len(activeTasks)+len(completedTasks))
						parentTask.Tasks = append(parentTask.Tasks, activeTasks...)
						parentTask.Tasks = append(parentTask.Tasks, completedTasks...)
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

		case "r":
			if !m.inputActive {
				m.inputActive = true
				m.inputAction = "rename"
				active, completed := m.getCurrentTasks()
				if m.cursor < len(active) {
					m.input.SetValue(active[m.cursor].Title)
				} else {
					completedIdx := m.cursor - len(active)
					m.input.SetValue(completed[completedIdx].Title)
				}
				m.input.Focus()
			}

		case "d":
			if !m.inputActive {
				active, completed := m.getCurrentTasks()
				if len(m.currentPath) == 0 {
					if m.cursor < len(active) {
						m.deletedTaskID = active[m.cursor].ID
						m.tasks = append(m.tasks[:m.cursor], m.tasks[m.cursor+1:]...)
					} else {
						completedIdx := m.cursor - len(active)
						m.deletedTaskID = completed[completedIdx].ID
						m.completedTasks = append(m.completedTasks[:completedIdx], m.completedTasks[completedIdx+1:]...)
					}
				} else {
					parentTask := &m.tasks[m.currentPath[0]]
					for i := 1; i < len(m.currentPath); i++ {
						parentTask = &parentTask.Tasks[m.currentPath[i]]
					}

					var activeTasks []Task
					var completedTasks []Task

					for _, task := range parentTask.Tasks {
						taskCopy := task
						if task.Completed {
							completedTasks = append(completedTasks, taskCopy)
						} else {
							activeTasks = append(activeTasks, taskCopy)
						}
					}

					if m.cursor < len(active) {
						m.deletedTaskID = activeTasks[m.cursor].ID
						activeTasks = append(activeTasks[:m.cursor], activeTasks[m.cursor+1:]...)
					} else {
						completedIdx := m.cursor - len(active)
						m.deletedTaskID = completedTasks[completedIdx].ID
						completedTasks = append(completedTasks[:completedIdx], completedTasks[completedIdx+1:]...)
					}

					parentTask.Tasks = make([]Task, 0, len(activeTasks)+len(completedTasks))
					parentTask.Tasks = append(parentTask.Tasks, activeTasks...)
					parentTask.Tasks = append(parentTask.Tasks, completedTasks...)
				}

				if m.cursor >= len(active)+len(completed)-1 && m.cursor > 0 {
					m.cursor--
				}
			}

		case "esc":
			if m.inputActive {
				m.inputActive = false
				m.input.Blur()
			} else if len(m.currentPath) > 0 {
				m.cursor = m.currentPath[len(m.currentPath)-1]
				m.currentPath = m.currentPath[:len(m.currentPath)-1]
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

	// Build breadcrumb trail
	if len(m.currentPath) > 0 {
		breadcrumb := "Main"
		current := m.tasks
		for _, idx := range m.currentPath {
			if idx < len(current) {
				breadcrumb += " > " + current[idx].Title
				current = current[idx].Tasks
			}
		}
		s += lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33")).Render(breadcrumb) + "\n\n"
	} else {
		s += lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33")).Render("Task List") + "\n\n"
	}

	// Get current level's tasks
	active, completed := m.getCurrentTasks()

	// Render active tasks
	for i, task := range active {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		taskStyle := lipgloss.NewStyle().PaddingLeft(2)
		if m.cursor == i {
			taskStyle = taskStyle.Bold(true).Foreground(lipgloss.Color("36"))
		}

		hasSubtasks := len(task.Tasks) > 0
		taskTitle := task.Title
		if hasSubtasks {
			taskTitle += " ▶"  // Add arrow to indicate subtasks
		}

		s += fmt.Sprintf("%s %s\n", cursor, taskStyle.Render(taskTitle))
	}

	// Render completed tasks
	if len(completed) > 0 {
		s += "\n" + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("31")).Render("Completed Tasks") + "\n\n"
		for i, task := range completed {
			cursor := " "
			if m.cursor == len(active)+i {
				cursor = ">"
			}

			taskStyle := lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("240"))
			if m.cursor == len(active)+i {
				taskStyle = taskStyle.Bold(true).Foreground(lipgloss.Color("36"))
			}

			hasSubtasks := len(task.Tasks) > 0
			taskTitle := task.Title
			if hasSubtasks {
				taskTitle += " ▶"
			}

			s += fmt.Sprintf("%s %s\n", cursor, taskStyle.Render(taskTitle))
		}
	}

	// Input field
	if m.inputActive {
		s += "\n" + m.input.View() + "\n"
	} else {
		s += "\n"
		if len(m.currentPath) > 0 {
			s += "Press 'n' to add a task, 'r' to rename, 'd' to delete, arrow keys to navigate, 'h' to go back, and 'esc' to exit.\n"
		} else {
			s += "Press 'n' to add a task, 'r' to rename, 'd' to delete, arrow keys to navigate, and 'esc' to exit.\n"
		}
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
