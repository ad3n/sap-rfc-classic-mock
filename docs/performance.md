# Pengukuran codec RFC

Diukur pada 23 November 2026: Go 1.27.4, darwin/arm64, Apple M3 Pro.
Baseline source: `98fb61993ba9cd963c800d8e9bb07db1e57a63f1` dengan benchmark
pada `internal/classic/benchmark_test.go` (benchmark Into hanya untuk versi baru).
Enam pengulangan per kasus, `go test -run '^$' -bench . -benchmem -count=6
./internal/classic`, dibandingkan menggunakan benchstat. Nilai waktu berikut median.

| Kasus | Sebelum | Sesudah | Alokasi sebelum → sesudah | Bytes sebelum → sesudah |
|---|---:|---:|---:|---:|
| Encode STRUCTURE | 151.85 ns | 94.31 ns (-37.89%) | 6 → 1 | 136 → 48 |
| Handler TABLE 128 baris | 60.61 µs | 51.88 µs (-14.40%) | 1812 → 1044 | 85872 → 71536 |
| Handler scalar CHAR echo | 340.2 ns | 295.8 ns (-13.02%) | 6 → 5 | 296 → 272 |

Perbandingan waktu sebelum/sesudah memiliki p=0.002, n=6. Hasil mentah:
[sebelum](benchmarks/before.txt), [sesudah](benchmarks/after.txt).
Ini benchmark codec dan handler dalam proses, bukan throughput TCP atau latency
SAP end-to-end. Angka ini belum membuktikan besaran percepatan pada native
linux/amd64; jalankan benchmark di host deployment untuk mengukurnya.

## Perubahan yang dipertahankan

Encoding CHAR langsung menulis UTF-16 ke destination tanpa slice rune/unit sementara.
Encoding STRUCTURE menulis setiap field langsung ke satu buffer hasil, termasuk
padding nol. Perhitungan ukuran tidak lagi membuat slice offsets ketika hanya
ukuran yang diperlukan. Respons tetap memiliki buffer sendiri.

`encodeInto` memakai buffer yang disediakan pemanggil dan mencapai 0 B/op serta
0 allocs/op pada benchmark STRUCTURE berisi CHAR, INT, FLOAT ([hasil](benchmarks/into.txt)).
Tes alokasi menjaga kontrak kasus ini. Ini bukan klaim nol alokasi untuk seluruh
request; `encode` tetap mengalokasikan satu buffer hasil, dan decoding/handler
masih mengalokasikan objek hasil.

## Percobaan pooling yang dibatalkan

Dicoba satu `sync.Pool` per function handler untuk map input `map[string]any`.
Map diambil saat masuk handler, lalu di-clear dan dikembalikan lewat defer pada
setiap return. Dibandingkan dengan codec baru tanpa pool, scalar melambat 10.34%
(p=0.002) dengan alokasi tetap 5. TABLE tidak menunjukkan peningkatan signifikan
(p=0.093), dan alokasi bertambah 1044 → 1045. Karena tidak bermanfaat pada workload
ini, pooling tidak dipertahankan. [Hasil mentah](benchmarks/pool.txt).

Tes mencakup Unicode/surrogate/truncation, input UTF-8 invalid, nilai awal, padding,
dan independensi buffer hasil. Validasi juga menjalankan race detector, vet,
betteralign, serta integrasi SDK native untuk scalar, STRUCTURE, TABLE dan koneksi paralel.
