# Student REST API

REST API sederhana menggunakan Go dan Fiber untuk mengelola data mahasiswa.

## Base URL

```
http://localhost:3000/api/v1
```

---

## Kontrak API

| Method | Endpoint | Parameter | Contoh Request Body | Status | Contoh Response |
|--------|----------|-----------|---------------------|--------|-----------------|
| GET | `/students` | `page`, `limit`, `search`, `sort`, `order`, `is_active` | - | 200 | `{ "success": true, "data": [...] }` |
| GET | `/students/:id` | `id` | - | 200, 400, 404 | `{ "success": true, "data": {...} }` |
| POST | `/students` | - | `{"nim":"231524001","name":"Faris","grade":90}` | 201, 409, 415, 422 | `{ "success": true, "data": {...} }` |
| PUT | `/students/:id` | `id` | `{"nim":"231524001","name":"Muhammad Faris","grade":95,"is_active":false}` | 200, 400, 404, 409, 415, 422 | `{ "success": true, "data": {...} }` |
| PATCH | `/students/:id` | `id` | `{"grade":100}` | 200, 400, 404, 409, 415, 422 | `{ "success": true, "data": {...} }` |
| DELETE | `/students/:id` | `id` | - | 204, 400, 404 | *(tidak ada body response)* |

---

## Struktur Data Mahasiswa

```json
{
    "id": 1,
    "nim": "231524001",
    "name": "Faris",
    "grade": 90,
    "is_active": true
}
```

---

## Query String

| Parameter | Keterangan |
|-----------|------------|
| page | Nomor halaman |
| limit | Jumlah data per halaman (maksimal 50) |
| search | Pencarian nama mahasiswa |
| sort | Field pengurutan (`id`, `nim`, `name`, `grade`, `is_active`) |
| order | `asc` atau `desc` |
| is_active | Filter status mahasiswa (`true` atau `false`) |

---

## Response Sukses

```json
{
    "success": true,
    "message": "mahasiswa berhasil ditambahkan",
    "data": {
        "id": 1,
        "nim": "231524001",
        "name": "Faris",
        "grade": 90,
        "is_active": true
    }
}
```

---

## Response Gagal

```json
{
    "success": false,
    "message": "validasi gagal",
    "errors": {
        "name": "Nama tidak boleh kosong"
    }
}
```

---

## HTTP Status

- 200 OK
- 201 Created
- 204 No Content
- 400 Bad Request
- 404 Not Found
- 409 Conflict
- 415 Unsupported Media Type
- 422 Unprocessable Entity
