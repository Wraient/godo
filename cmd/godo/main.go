package main

import (
	"github.com/wraient/godo/internal"
	"time"
)

func main() {
	now := time.Now()
	
	// Sample tasks with hierarchy and all fields
	tasks := []internal.Task{
		{
			ID:          "1",
			Title:       "Plan vacation",
			Description: "Plan a week-long vacation to Japan",
			Notes:       "Need to coordinate with family members",
			Completed:   false,
			CreatedAt:   now,
			DueDate:     now.AddDate(0, 0, 14),
			Tasks: []internal.Task{
				{
					ID:          "2",
					Title:       "Book flight",
					Description: "Find and book round-trip flights",
					Notes:       "Check both direct and connecting flights",
					Completed:   false,
					CreatedAt:   now,
					DueDate:     now.AddDate(0, 0, 3),
					Tasks:       []internal.Task{},
				},
				{
					ID:          "3",
					Title:       "Reserve hotel",
					Description: "Book hotels for each city",
					Notes:       "Check reviews and locations",
					Completed:   false,
					CreatedAt:   now,
					DueDate:     now.AddDate(0, 0, 5),
					Tasks:       []internal.Task{},
				},
			},
		},
		{
			ID:          "4",
			Title:       "Work projects",
			Description: "Current work assignments",
			Notes:       "Track progress of all work items",
			Completed:   false,
			CreatedAt:   now,
			DueDate:     now.AddDate(0, 0, 30),
			Tasks: []internal.Task{
				{
					ID:          "5",
					Title:       "Project presentation",
					Description: "Prepare quarterly review presentation",
					Notes:       "Include metrics and future plans",
					Completed:   false,
					CreatedAt:   now,
					DueDate:     now.AddDate(0, 0, 7),
					Tasks:       []internal.Task{},
				},
			},
		},
		{
			ID:          "6",
			Title:       "Shopping list",
			Description: "Items to buy this week",
			Notes:       "Check for deals and coupons",
			Completed:   true,
			CreatedAt:   now.AddDate(0, 0, -2),
			DueDate:     now.AddDate(0, 0, 5),
			Tasks:       []internal.Task{},
		},
	}

	// Run the Bubble Tea Task UI
	internal.RunTaskUI(tasks)
}
