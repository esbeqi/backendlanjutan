package main

import "fmt"

// Pass by Value
func ubahNilai(x int) {
	x = 100
}

// Pass by Pointer
func swap(a, b *int) {
	*a, *b = *b, *a
}

func main() {

	// =========================
	// Pass by Value
	// =========================
	a := 10

	fmt.Println("===== Pass by Value =====")
	fmt.Println("Sebelum :", a)

	ubahNilai(a)

	fmt.Println("Sesudah :", a)

	// =========================
	// Pass by Pointer
	// =========================
	x := 10
	y := 20

	fmt.Println("\n===== Pass by Pointer =====")
	fmt.Println("Sebelum swap :", x, y)

	swap(&x, &y)

	fmt.Println("Sesudah swap :", x, y)
}
