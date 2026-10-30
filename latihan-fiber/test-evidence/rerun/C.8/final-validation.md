# C.8 — Validasi Akhir (rerun)

Status akhir: **LULUS**

## A. Build dan Vet

| Perintah | Hasil | Bukti |
|---|---|---|
| go build ./... | Selesai tanpa error (exit code 0). | go-build-result.txt, go-build-output.txt |
| go vet ./... | Selesai tanpa error atau warning relevan (exit code 0; output kosong). | go-vet-result.txt, go-vet-output.txt |

## B. Checklist C.1–C.6

| Skenario | Hasil aktual terhadap kriteria modul | Bukti utama | Status |
|---|---|---|---|
| C.1 Normal Path | 4 outbox event (created, dropped, created, swapped), 4 notification, 4 email untuk mhsuji; email memiliki subject sesuai. Request duplikat menghasilkan 409 dan tidak menambah event. Pengujian bersih final disimpan terpisah dari percobaan pertama yang memproses task probe lama. | ../C.1/terminal-final.txt, ../C.1/snapshot-final.txt, ../C.1/mail-final.log, ../C.1/app-log-rerun.txt | LULUS |
| C.2 Redis Down | Redis benar-benar berhenti. Tiga operasi KRS saat down mendapat 201; setelah 5 detik queue menunjukkan pending 3 dan queue_error. Setelah Redis aktif, pending 0, tiga notification diproses, total akun menjadi 7; tiga email C.2 diverifikasi sebagai delta. | ../C.2/redis-down-state.txt, ../C.2/terminal-redis-down.txt, ../C.2/redis-up-state.txt, ../C.2/terminal-redis-up.txt, ../C.2/snapshot-final.txt, ../C.2/mail-delta.log, ../C.2/app-log-rerun.txt | LULUS |
| C.3 Temporary Failure | 6 event, 6 notification, 6 email. Terdapat 2 task_failed saat MAILER_FAIL_EVERY=3 (attempt email ke-3 dan ke-6), kemudian retry menyelesaikan semua tugas. | ../C.3/worker-start.txt, ../C.3/terminal.txt, ../C.3/snapshot-final.txt, ../C.3/app-log-rerun.txt, ../C.3/mail.log | LULUS |
| C.4 Permanent Failure | KRS berstatus 201. queue.archived naik tepat 1 (1 menjadi 2), queue.retry tetap 0. Hanya satu task_failed untuk alamat kampus.invalid dengan attempt 1/6; tidak ada notification maupun email. | ../C.4/queue-before.txt, ../C.4/terminal.txt, ../C.4/snapshot-final.txt, ../C.4/app-log-rerun.txt, ../C.4/mail.log | LULUS |
| C.5 Idempotency | Lima task duplikat ditambah satu task relay. Untuk outbox 56 tercatat tepat 6 notification_processed (1 already_sent=false, 5 true), satu notification, dan satu email. | ../C.5/redeliver.txt, ../C.5/terminal.txt, ../C.5/snapshot-final.txt, ../C.5/app-log-rerun.txt, ../C.5/mail.log | LULUS |
| C.6 Graceful Shutdown | Ctrl+C dikirim melalui console host yang sama dengan worker sekitar 2 detik setelah relay_done. Worker mencatat sinyal diterima, menyelesaikan email setelah jeda 5 detik, menulis notification_processed, kemudian worker berhenti dengan rapi. Proses worker exit terverifikasi; MAILER_DELAY_MS dikembalikan ke 1500. | ../C.6/host-start-corrected.txt, ../C.6/worker-final.txt, ../C.6/app-log-rerun.txt, ../C.6/verify-after-shutdown.txt, ../C.6/mail.log, ../C.6/worker-env-restored.txt | LULUS |

## C. C.7 dan batas verifikasi

C.7 LULUS berdasarkan audit aktual di ../C.7/audit-final.md. Semua field wajib hadir, task_failed menggunakan format attempt x/6, relay_delayed hanya muncul saat Redis down pada C.2, dan pencarian request_failed pada enam log rerun menghasilkan nol kecocokan.

Tidak ada kriteria C.1–C.6 yang belum terverifikasi pada bukti rerun final. Percobaan awal C.1 tetap dipertahankan di ../C.1/first-attempt/ untuk transparansi; hasilnya tidak dipakai sebagai hasil final karena satu task probe lama sempat menghasilkan email tambahan yang bukan milik mhsuji.

## Status akhir

C.1–C.6: **LULUS**  
C.7: **LULUS**  
Build dan vet: **LULUS**  
C.8: **LULUS**
