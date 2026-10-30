# C.7 - Audit Log Modul 11

## Metode dan ruang lingkup

Saya memeriksa baris log JSON aktual yang tersimpan pada `test-evidence/C.1` sampai `C.6-final`, bukan hanya ringkasan skenario. File C.2 memuat duplikasi capture dari proses worker yang sama; duplikasi itu tidak dihitung sebagai event unik. `C.2/worker.log` mencatat 28 `relay_delayed` unik. Bukti lama tidak diedit.

## Checklist hasil

| Pesan | Proses | Field wajib | Pemeriksaan aktual | Contoh aktual / sumber | Hasil |
|---|---|---|---|---|---|
| `relay_done` | worker | `published` | Field ada pada setiap baris yang diaudit. Nilai publish sesuai skenario: C.1=4, C.2=3 setelah Redis pulih, C.3=6, C.4=1, C.5=1, C.6-final=1. | `C.1/worker-snapshot.txt`: `{"time":"2026-10-30T10:19:14.2355383+07:00","level":"INFO","msg":"relay_done","published":4}`. Lihat juga `C.2/worker.log`, `C.3/worker-events.log`, `C.4/worker-events.log`, `C.5/worker-events.log`, `C.6-final/stdout.log`. | LULUS |
| `relay_delayed` | worker | `published`, `error` | Semua baris memiliki dua field; pada C.2 `published=0` dan `error` berisi kegagalan enqueue ke Redis. Event hanya ditemukan pada rentang Redis mati; file capture yang tumpang tindih berisi salinan baris yang sama. | Baris aktual: `{"time":"2026-10-30T10:20:57.275744+07:00","level":"WARN","msg":"relay_delayed","published":0,"error":"enqueue outbox 23: UNKNOWN: redis eval error: dial tcp [::1]:6379: connectex: No connection could be made because the target machine actively refused it."}` di `C.2/worker.log`. | LULUS |
| `notification_processed` | worker | `outbox_id`, `event`, `retry`, `already_sent` | Semua field tersedia. Nilainya sesuai alur: C.1=4, C.2=3, C.3=6, C.5=6 (satu `false`, lima `true`), C.6-final=1. C.4 memang tidak menghasilkan notification karena kegagalan permanen. | Contoh aktual di `C.3/worker-events.log`: `{"time":"2026-10-30T10:23:16.2342497+07:00","level":"INFO","msg":"notification_processed","outbox_id":26,"event":"enrollment.created","retry":0,"already_sent":false}`. C.6-final: `C.6-final/stdout.log`. | LULUS |
| `task_failed` | worker | `type`, `attempt`, `error` | Ketiga field tersedia pada semua 3 event. Semua `attempt` aktual cocok pola `x/6`: dua kegagalan sementara C.3 dan satu kegagalan permanen C.4 menggunakan `1/6`. | C.3: `{"time":"2026-10-30T10:23:16.2422459+07:00","level":"WARN","msg":"task_failed","type":"notification:send","attempt":"1/6","error":"pengiriman gagal sementara: server email sibuk (attempt ke-3)"}`. C.4: `C.4/worker-events.log`. | LULUS |
| `request_failed` | API | tidak boleh muncul | **Sebelum perbaikan:** 1 baris pada C.1 untuk duplikat KRS status 409 `ALREADY_ENROLLED`. Baris lama disimpan utuh di `request-failed-before.log`. **Sesudah perbaikan:** probe duplikat menghasilkan HTTP 409 yang sama, dicatat sebagai `request_rejected`/WARN; tidak ada `request_failed` di log probe baru. | Sebelum: `C.1/api.log` dan `request-failed-before.log`. Sesudah: `retest-api.log`, request `201` lalu `409`, satu `request_rejected`, nol `request_failed`. | BELUM LULUS untuk arsip C.1-C.6 historis |

## Temuan, akar masalah, dan perbaikan

Komentar di `config/app.go` menyatakan hanya kegagalan server yang dicatat sebagai Error dan 4xx merupakan penolakan pemakai. Implementasi sebelumnya menggunakan kondisi `Status < 500` untuk menulis `request_failed`/Error, sehingga 409 yang diharapkan pada langkah duplikat C.1 melanggar C.7. Saya menukar klasifikasi: 5xx menjadi `request_failed`/Error, 4xx menjadi `request_rejected`/Warn. Komentar dan perilaku HTTP dipertahankan.

Probe pascaperbaikan menggunakan akun uji baru: pendaftaran `201`, daftar kelas pertama `201`, duplikat `409 ALREADY_ENROLLED`. Log aktual mencatat `request_rejected` dan tidak mencatat `request_failed`; lihat `retest-terminal.txt`, `retest-api.log`, dan `retest-verification.txt`.

## Status akhir C.7

**BELUM LULUS untuk seluruh arsip C.1-C.6 yang diminta audit**, karena log historis C.1 masih berisi satu `request_failed`. Log tersebut tidak diubah atau dihapus. Perbaikan telah diverifikasi dengan uji relevan yang baru; agar kriteria “tidak boleh muncul pada seluruh uji C.1-C.6” lulus secara menyeluruh, suite C.1-C.6 perlu dijalankan kembali setelah perbaikan dan disimpan di direktori bukti baru.
