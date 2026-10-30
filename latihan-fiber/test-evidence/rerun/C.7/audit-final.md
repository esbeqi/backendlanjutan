# C.7 — Audit Logging (rerun)

Status akhir: **LULUS**

Audit dilakukan pada baris log JSON aktual yang dipisahkan per skenario di test-evidence/rerun/C.1 sampai C.6/app-log-rerun.txt. Source code hanya dipakai sebagai konteks; status ditentukan dari baris log hasil eksekusi.

## Checklist

| Jenis log | Hasil pemeriksaan aktual | Status |
|---|---|---|
| relay_done | 7 baris pada C.1–C.6; seluruhnya memiliki field published. Jumlah publikasi tercatat sesuai skenario, termasuk 3 event C.2 setelah Redis pulih. | LULUS |
| relay_delayed | 7 baris, hanya di C.2 ketika Redis mati; semuanya memiliki published dan error dengan pesan koneksi Redis. Tidak ada relay_delayed pada C.1, C.3, C.4, C.5, atau C.6. | LULUS |
| notification_processed | 20 baris; semua memiliki outbox_id, event, retry, dan already_sent. C.5 mencatat tepat 6 untuk outbox 56 (1 false, 5 true). | LULUS |
| task_failed | 3 baris; semua memiliki type, attempt, dan error. Ketiganya menggunakan format attempt x/6. C.3 mencatat 2 kegagalan sementara; C.4 mencatat 1 kegagalan permanen. | LULUS |
| request_failed | 0 baris pada log terbaru C.1–C.6. | LULUS |

## Baris log aktual

relay_done — sumber: test-evidence/rerun/C.1/app-log-rerun.txt

    {"time":"2026-10-30T12:24:59.0824558+07:00","level":"INFO","msg":"relay_done","published":2}

relay_delayed — sumber: test-evidence/rerun/C.2/app-log-rerun.txt

    {"time":"2026-10-30T12:19:27.3592773+07:00","level":"WARN","msg":"relay_delayed","published":0,"error":"enqueue outbox 46: UNKNOWN: redis eval error: dial tcp [::1]:6379: connectex: No connection could be made because the target machine actively refused it."}

notification_processed — sumber: test-evidence/rerun/C.5/app-log-rerun.txt

    {"time":"2026-10-30T12:21:34.26232+07:00","level":"INFO","msg":"notification_processed","outbox_id":56,"event":"enrollment.created","retry":0,"already_sent":false}

task_failed — sumber: test-evidence/rerun/C.4/app-log-rerun.txt

    {"time":"2026-10-30T12:20:44.8702278+07:00","level":"WARN","msg":"task_failed","type":"notification:send","attempt":"1/6","error":"skip retry for the task: pengiriman gagal permanen: alamat salah@kampus.invalid tidak ditemukan"}

request_rejected untuk respons 409 — sumber: test-evidence/rerun/C.1/app-log-rerun.txt

    {"time":"2026-10-30T12:24:59.0668609+07:00","level":"WARN","msg":"request_rejected","request_id":"5171385e-2407-4c1b-b92c-3a28fb350044","path":"/api/v1/courses/2/enroll","code":"ALREADY_ENROLLED","status":409}

## Pemeriksaan field dan pencarian request_failed

- relay_done: seluruh 7/7 baris memiliki published.
- relay_delayed: seluruh 7/7 baris memiliki published dan error; seluruhnya terjadi ketika Redis mati pada C.2.
- notification_processed: seluruh 20/20 baris memiliki empat field wajib.
- task_failed: seluruh 3/3 baris memiliki tiga field wajib dan attempt cocok dengan pola angka/6.
- Pencarian literal request_failed pada enam file app-log-rerun.txt: tidak ditemukan.
- request_rejected tercatat pada duplikasi KRS C.1 dengan status 409.

## Referensi bukti

- C.1: app-log-rerun.txt, terminal-final.txt, snapshot-final.txt, mail-final.log.
- C.2: app-log-rerun.txt, terminal-redis-down.txt, terminal-redis-up.txt, redis-down-state.txt, redis-up-state.txt, snapshot-final.txt.
- C.3: app-log-rerun.txt, terminal.txt, snapshot-final.txt, mail.log.
- C.4: app-log-rerun.txt, queue-before.txt, terminal.txt, snapshot-final.txt.
- C.5: app-log-rerun.txt, redeliver.txt, terminal.txt, snapshot-final.txt, mail.log.
- C.6: app-log-rerun.txt, worker-final.txt, worker-env-restored.txt, verify-after-shutdown.txt, mail.log.

Percobaan pertama C.1 yang sempat memproses satu email task probe lama tetap tersimpan di C.1/first-attempt. Bukti itu tidak diubah dan tidak dipakai untuk hasil final C.1; pengujian bersih ulang menghasilkan tepat empat email mhsuji.
