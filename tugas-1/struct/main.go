package main

import "fmt"

//struct student
type Student struct {
	ID       int
	Name     string
	Grade    float64
	IsActive bool
}

//value receiver
func (s Student) GetInfo() string {
	return fmt.Sprintf("ID: %d | Nama: %s | Nilai: %.2f | Aktif: %t",
		s.ID, s.Name, s.Grade, s.IsActive)
}

//pointer receiver
func (s *Student) UpdateGrade(grade float64) {
	s.Grade = grade
}

//pointer receiver
func (s *Student) Activate() {
	s.IsActive = true
}

func (s *Student) Deactivate() {
	s.IsActive = false
}

func main() {

	student := Student{
		ID:       1,
		Name:     "Sari",
		Grade:    85.0,
		IsActive: false,
	}

	fmt.Println("=== Data Awal ===")
	fmt.Println(student.GetInfo())

	student.UpdateGrade(92.5)
	student.Activate()

	fmt.Println("\n=== Setelah Update ===")
	fmt.Println(student.GetInfo())

	student.Deactivate()

	fmt.Println("\n=== Setelah Deactivate ===")
	fmt.Println(student.GetInfo())
}
