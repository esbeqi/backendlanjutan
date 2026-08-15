package main

import "fmt"

func main() {

	// Variabel
	var nama string = "Sari"
	umur := 20
	ipk := 3.85
	aktif := true
	hobi := []string{"Membaca", "Coding", "Musik"}

	fmt.Println("===== Variabel =====")
	fmt.Println("Nama   :", nama)
	fmt.Println("Umur   :", umur)
	fmt.Println("IPK    :", ipk)
	fmt.Println("Aktif  :", aktif)
	fmt.Println("Hobi   :", hobi)

	// Map
	skor := make(map[string]int)

	// Menambah data
	skor["Sari"] = 90
	skor["Budi"] = 85
	skor["Andi"] = 95

	fmt.Println("\n===== Data Awal =====")
	fmt.Println(skor)

	// Membaca data dengan pengecekan keberadaan
	if nilai, ada := skor["Budi"]; ada {
		fmt.Println("Nilai Budi :", nilai)
	} else {
		fmt.Println("Budi belum memiliki nilai")
	}

	// Menghapus data
	delete(skor, "Budi")

	fmt.Println("\n===== Setelah Menghapus Budi =====")

	// Menampilkan seluruh isi map
	for nama, nilai := range skor {
		fmt.Printf("%s : %d\n", nama, nilai)
	}
}
