package main

import (
	"github.com/wraient/godo/internal"
	"time"
)

func main() {
	// Sample tasks
	tasks := []internal.Task{
		{ID: "1", Title: "Plan vacation", Notes: "Organize everything for the trip", Completed: false, DueDate: time.Now().AddDate(0, 0, 7), ParentID: ""},
		{ID: "2", Title: "Book flight", Notes: "", Completed: false, DueDate: time.Now().AddDate(0, 0, 3), ParentID: "1"},
		{ID: "3", Title: "Reserve hotel", Notes: "", Completed: false, DueDate: time.Now().AddDate(0, 0, 5), ParentID: "1"},
		{ID: "4", Title: "Prepare itinerary", Notes: "", Completed: true, DueDate: time.Now().AddDate(0, 0, 6), ParentID: ""},
	}

	// Run the Bubble Tea Task UI
	internal.RunTaskUI(tasks)
}
