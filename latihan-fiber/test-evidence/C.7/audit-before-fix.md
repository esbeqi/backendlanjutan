# Audit C.7 sebelum perbaikan

Sumber: log JSON asli dalam test-evidence/C.1 hingga C.6-final, termasuk tambahan C.6-retest; baris worker/API yang tersalin lintas file dihitung sebagai bukti fisik sesuai file dan tidak dihapus. Lihat field-audit-before.json untuk daftar sumber dan contoh raw line.

## Hasil field
- notification_processed: 34 baris; field hilang=0; attempt tidak sesuai=0.
- task_failed: 3 baris; field hilang=0; attempt tidak sesuai=0.
- relay_done: 26 baris; field hilang=0; attempt tidak sesuai=0.
- relay_delayed: 128 baris; field hilang=0; attempt tidak sesuai=0.
- request_failed: 1 baris; field hilang=0; attempt tidak sesuai=0.

## Ketidaksesuaian
- `request_failed` muncul 1 kali di `test-evidence/C.1/api.log`: respons 409 `ALREADY_ENROLLED` pada langkah duplikasi KRS. Baris persis tersimpan di `request-failed-before.log`. C.7 melarang event ini di seluruh C.1-C.6.
- Baris `relay_delayed` berada pada log C.2 saat Redis mati; `published=0` dan field `error` berisi kegagalan enqueue ke Redis. Tidak ditemukan `relay_delayed` pada skenario lain yang terpilah.
- Semua `relay_done`, `notification_processed`, dan `task_failed` yang terpilah memiliki field wajib; setiap `attempt` cocok `x/6`.
- `config/app.go` memiliki komentar yang menyatakan hanya kegagalan server dicatat sebagai Error dan 4xx adalah kesalahan pemakai, tetapi implementasi memakai kondisi `<500` untuk `request_failed` Error. Akar masalah: kondisi klasifikasi terbalik.

