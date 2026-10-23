package model

type Supervisor struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Quota           int    `json:"quota"`
	SupervisedCount int    `json:"supervised_count"`
	Version         int    `json:"version"`
}

type Supervision struct {
	StudentID    int `json:"student_id"`
	SupervisorID int `json:"supervisor_id"`
}

type AssignSupervisorRequest struct {
	StudentID int `json:"student_id" validate:"required,min=1"`
}
