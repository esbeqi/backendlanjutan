# C.8 - Validasi Akhir Modul 11

## A. Build dan vet

| Perintah | Exit code | Output | Status |
|---|---:|---|---|
| `go build ./...` | 0 | Tidak ada error/output. | LULUS |
| `go vet ./...` | 0 | Tidak ada warning/output. | LULUS |

Log terminal: `build.txt` dan `vet.txt`.

## B. Kecocokan bukti C.1-C.6

| Skenario | Kriteria modul | Bukti aktual yang diperiksa | Hasil | Status |
|---|---|---|---|---|
| C.1 Normal Path | 4 event, 4 notification, 4 email dengan subject tepat. | `C.1/database.txt`: outbox=4, notifications=4, API=4, pending=0. `C.1/mail.log`: empat email: terdaftar IF102, keluar IF102, terdaftar IF102, pindah IF102 ke IF302. | Jumlah dan empat subject sesuai. | LULUS |
| C.2 Redis Down | 3 KRS diterima saat Redis mati; 3 notification tiba setelah Redis hidup. | `C.2/redis-down.txt`: POST course 5/6/7 semua 201; pending=3; Redis TCP false. `database-down.txt`: IDs 23-25 belum published dan notif total 4. `database-up.txt`: IDs 23-25 published dan notif total 7. `mail.log`: tiga email untuk IF303/IF304/IF305. | Delta notification 4→7 = 3; ketiga event tidak hilang dari outbox. | LULUS |
| C.3 Temporary Failure | 6 event = 6 notification = 6 email; kegagalan terjadi sebelum retry berhasil. | `C.3/database.txt`: outbox=6, notifications=6, API=6, mail=6, pending=0. `worker-events.log`: enam `notification_processed`, dua `task_failed` pada kegagalan ke-3 dan ke-6; retry berikutnya sukses. `mail.log`: enam baris. | Semua jumlah cocok dan kegagalan sementara benar-benar terjadi. | LULUS |
| C.4 Permanent Failure | Satu attempt, langsung archived. | `C.4/terminal.txt`: archived naik 0→1, retry=0. `worker-events.log`: satu `task_failed`, attempt=1/6, `skip retry` untuk alamat permanen gagal. `database.txt`: notification=0; `mail.log` kosong. | Satu percobaan dan archived sesuai mekanisme. | LULUS |
| C.5 Idempotency | Enam salinan task, satu email. | `C.5/redeliver.txt`: lima salinan ditambahkan; relay menyediakan salinan keenam. `worker-events.log`: tepat enam baris untuk outbox 33 (satu `already_sent=false`, lima `true`). `mail.log`: satu baris. | Enam task diproses dan hanya satu email ditulis. | LULUS |
| C.6 Graceful Shutdown | Ctrl+C tidak memutus email aktif; worker menyelesaikan tugas lalu berhenti rapi. | `C.6-final/stdout.log`: `relay_done` 10:46:51.645; penerimaan sinyal 10:46:53.657; `notification_processed` 10:46:57.395; shutdown rapi 10:46:57.408. `database-after.txt`: 1 outbox, 1 notification/API, pending=0. `mail.log`: satu email IF307. | Sinyal diterima sekitar 2 detik sesudah relay; email selesai sebelum worker berhenti. | LULUS |

## Temuan dan keterbatasan

- Pengujian perilaku C.1-C.6 sesuai target pada bukti yang tersedia.
- C.7 menemukan dan memperbaiki klasifikasi log 4xx. Uji duplikat pascaperbaikan mencatat `request_rejected`, tetapi log C.1 historis sebelum perbaikan tetap mengandung satu `request_failed` dan sengaja tidak diubah. Karena itu status audit arsip C.7 tetap **BELUM LULUS** sampai C.1-C.6 dijalankan ulang sebagai satu set setelah perbaikan. Keterbatasan audit historis ini tidak mengubah hasil hitungan operasional C.1-C.6 yang dicek pada tabel di atas.
- Snapshot helper C.6 mencetak `mail_log_lines=0` untuk akun khusus karena helper tidak mengenali alamat akun baru. Log email asli difilter berdasarkan penerima dan memuat satu email; rincian ada di `C.6-final/verification-notes.txt`.
- `build.txt` dan `vet.txt` berasal dari pemeriksaan setelah perbaikan `config/app.go`; tidak ada test suite yang ditambahkan atau dijalankan pada tahap ini.

## Status akhir C.8

**LULUS** untuk kriteria C.8 yang diminta: build/vet bersih dan hasil perilaku C.1-C.6 cocok dengan checklist modul. Status C.7 dilaporkan terpisah sebagai BELUM LULUS untuk seluruh arsip historis karena bukti pra-perbaikan dipertahankan.
