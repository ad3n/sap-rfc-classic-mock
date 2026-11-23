# SAP RFC Classic Mock

Go module: `github.com/ad3n/sap-rfc-classic-mock`.

Server mock **SAP RFC klasik melalui TCP**, tanpa SAP Gateway dan tanpa SAP ABAP backend. Nama function, parameter, schema `STRUCTURE`/`TABLE`, serta output statis atau echo diatur lewat YAML.

Client penguji memakai `github.com/ad3n/gonwrfc v1.0.11` dan SAP NetWeaver RFC SDK asli. Transport klasik server menggunakan module [github.com/ad3n/open-rfc-go](https://github.com/ad3n/open-rfc-go) melalui package `mockbridge`. Patch protokol, adapter, tes, dan provenance dikelola di repo tersebut. API server SDK SAP sendiri menyediakan registrasi Gateway dan standalone WebSocket; transport klasik standalone ini tidak memakai API server SDK.

## Jalankan

Prasyarat build: Go 1.27.3+, C compiler, dan SAP NetWeaver RFC SDK sesuai arsitektur. SDK digunakan client penguji dan mode WebSocket lama; library SAP tidak didistribusikan dalam repo. Pengujian lokal memakai SDK 7.50 PL18 macOS arm64.

```sh
task run SAPNWRFC_HOME="$HOME/nwrfcsdk"
```

Konfigurasi default [mock.yml](mock.yml):

```yaml
transport: classic
listen_host: 127.0.0.1
port: 3313
```

Server menerima kredensial apa pun untuk simulasi, tanpa SNC/TLS. Bind default hanya loopback. Untuk container/mesin lain, ubah `listen_host` ke alamat interface yang sesuai atau `0.0.0.0` di jaringan testing. Jangan dipakai sebagai server production.

```sh
task build-local SAPNWRFC_HOME="$HOME/nwrfcsdk"
export DYLD_LIBRARY_PATH="$HOME/nwrfcsdk/lib" # macOS
# export LD_LIBRARY_PATH="$HOME/nwrfcsdk/lib" # Linux
./bin/rfc-server -config mock.yml -check
./bin/rfc-server -config mock.yml
```

Ctrl+C menghentikan listener dan koneksi aktif. Restart server setelah perubahan YAML. Restart proses client/worker ketika schema function berubah; reconnect saja belum tentu membersihkan cache metadata SDK. Koneksi testing dibatasi sampai lima menit per sesi.

## Docker Compose

```sh
docker compose up -d
docker compose logs -f rfc-mock
docker compose exec rfc-mock rfc-call -params '{"REQUTEXT":"Hello Compose"}'
docker compose down
```

Jika subcommand `docker compose` belum tersedia, gunakan `docker-compose` untuk perintah yang sama (binary ini tersedia pada mesin pengujian).

[docker-compose.yml](docker-compose.yml) memakai image registry dan mem-publish port
`3313`, dan me-mount `./mock.yml` dari host ke `/app/mock.yml` secara read-only.
Lawang-Core di host memakai `127.0.0.1:13`. Opsi `-listen-host 0.0.0.0` mengoverride
binding listener untuk container tanpa mengubah file YAML host. Config dibaca saat
startup; setelah mengubah YAML, jalankan `docker compose restart rfc-mock`. Restart
juga proses client/worker jika schema RFC berubah agar cache metadata diperbarui.

Implementasi protokol memakai module `github.com/ad3n/open-rfc-go v0.0.1`.
Build mengambil dependency dari rilis tersebut tanpa membutuhkan repo lokal.

## Docker

Build dan push mengikuti pola Taskfile Lawang-Core:

```sh
docker login git.moklaar.cloud
task build -- 0.0.1
```

Ganti `0.0.1` dengan tag image yang ingin dirilis. Task ini membangun image
`linux/amd64`, lalu push `git.moklaar.cloud/lawang/rfc-server:latest` dan tag
versi yang diberikan. `--load` menyediakan image lokal untuk langkah tag/push.

SDK Linux x86-64 sudah disalin dari `lawang-core/docker/nwrfcsdk` ke `docker/nwrfcsdk`.
Build image menggunakan platform yang sesuai library tersebut:

```sh
docker build --platform linux/amd64 -t rfc-server:local .
docker run --rm --name rfc-mock -p 127.0.0.1:3313:3313 rfc-server:local
```

Image berisi server dan client `rfc-call`, berjalan sebagai user non-root.
Konfigurasi bawaan image mengikuti `mock.yml`, dengan `listen_host` diubah menjadi
`0.0.0.0` agar port container dapat diakses. File YAML lokal tidak diubah.
Lawang-Core di host tetap menggunakan `127.0.0.1:13`.

Uji melalui client di container:

```sh
docker exec rfc-mock rfc-call -function STFC_CONNECTION -params '{"REQUTEXT":"Hello Docker"}'
```

Untuk YAML sendiri, set `listen_host: 0.0.0.0`, lalu mount file sebagai read-only:

```sh
docker run --rm --name rfc-mock -p 127.0.0.1:3313:3313 \
  --mount type=bind,src="$(pwd)/custom.yml",dst=/app/mock.yml,readonly \
  rfc-server:local
```

Jika Lawang-Core juga di Docker, gunakan network Docker yang sama dan alamat backend
`rfc-mock:13`. Untuk host lain, sesuaikan alamat publish port dan akses jaringan testing.

## Lawang-Core: cukup ubah alamat backend

Lawang-Core yang menggunakan `ashost`/`sysnr` dapat memakai host berikut, tanpa perubahan kode atau parameter WebSocket:

```json
{
  "name": "rfc-mock",
  "protocol": "rfc",
  "hosts": ["127.0.0.1:13"],
  "username": "MOCK",
  "password": "MOCK",
  "database": "000"
}
```

Gabungkan field tersebut dengan konfigurasi endpoint/backend Anda yang sudah ada. Nama function dan parameter endpoint tetap sama, asalkan didefinisikan di YAML. Kredensial existing juga diterima mock ini.

**Angka setelah titik dua pada URL RFC Lawang-Core adalah system number, bukan port TCP literal.** Kode `ParseConnectionParams` memetakannya ke `sysnr`; SDK menghubungi port `3300 + sysnr`.

| Port listener mock | Host backend Lawang-Core | Parameter SDK |
|---|---|---|
| 3300 | `127.0.0.1:00` | `ashost=127.0.0.1`, `sysnr=00` |
| 3313 | `127.0.0.1:13` | `ashost=127.0.0.1`, `sysnr=13` |
| 3399 | `127.0.0.1:99` | `ashost=127.0.0.1`, `sysnr=99` |

Contoh URL lengkap: `rfc://127.0.0.1:13`. Tidak perlu `wshost`, `wsport`, `use_tls`, Program ID, SM59, atau SAP Gateway. Untuk Lawang-Core di container, gunakan alamat mock yang bisa diakses container dan bind interface yang sesuai.

Untuk alur RFC table/sync Lawang-Core, pilih function `Z_LAWANG_DATA`, output key `T_ITEMS`, dan fields `CODE`, `QUANTITY`, `PRICE`. Untuk hasil seluruh object, gunakan output key kosong. Instance dan konfigurasi endpoint Lawang-Core penuh tidak dijalankan dalam test repo ini; integration test menguji koneksi SDK dengan parameter yang sama dengan hasil parsing Lawang-Core.

## YAML

Contoh parameter kompleks:

```yaml
transport: classic
port: 3313
functions:
  - name: Z_EXAMPLE
    parameters:
      - name: IS_DATA
        direction: import
        type: STRUCTURE
        fields: &row_fields
          - {name: CODE, type: CHAR, length: 5}
          - {name: QUANTITY, type: INT}
          - {name: PRICE, type: FLOAT}
      - name: ES_DATA
        direction: export
        type: STRUCTURE
        fields: *row_fields
        from: IS_DATA
      - name: T_ITEMS
        direction: tables
        type: TABLE
        fields: *row_fields
        value:
          - {CODE: A001, QUANTITY: 2, PRICE: 12.5}
          - {CODE: B002, QUANTITY: 3, PRICE: 7.25}
      - name: T_ECHO
        direction: tables
        type: TABLE
        fields: *row_fields
        from: T_ECHO
```

- `name`: uppercase, 1–30 karakter, diawali huruf; sisanya huruf, angka, underscore.
- Scalar: `CHAR` (wajib `length` dalam unit UTF-16), `STRING`, `INT` signed 32-bit, `FLOAT` finite.
- `STRUCTURE`: object dengan `fields`; arah `import`, `export`, atau `changing`.
- `TABLE`: list object, schema satu baris di `fields`, arah `tables` (parameter RFC TABLES dua arah).
- Field struktur/baris tabel saat ini mendukung `CHAR`, `INT`, dan `FLOAT`. Struktur/tabel bersarang dan field `STRING` di dalamnya belum didukung dan ditolak saat startup.
- `value`: output statis sesuai tipe. Object boleh menghilangkan field; field yang hilang memakai nilai awal (teks kosong/angka nol). Field yang tidak dikenal ditolak.
- `from`: echo parameter input/changing/tables dengan schema yang identik, termasuk nama dan urutan field. Semua input dibaca sebelum output ditulis.
- `value: []` menghasilkan tabel kosong. Output statis mengganti isi tabel input, bukan menambah baris.
- Tanpa `value`/`from`, `changing` dan `tables` mempertahankan input; parameter export memakai nilai awal.
- `value` dan `from` tidak boleh bersamaan, serta tidak boleh pada parameter import.

Nama/field duplikat, tipe/arah tidak didukung, output salah tipe, field asing, nilai null di field, serta echo dengan schema berbeda ditolak. YAML mendukung anchors/aliases seperti contoh. Function metadata internal (`RFC_PING`, `RFC_GET_FUNCTION_INTERFACE`, `RFC_GET_STRUCTURE_DEFINITION`, `DDIF_FIELDINFO_GET`) tidak boleh ditimpa.

## Client penguji

Dengan server tetap berjalan:

```sh
export DYLD_LIBRARY_PATH="$HOME/nwrfcsdk/lib"
./bin/rfc-call
./bin/rfc-call -function Z_LAWANG_TEST -params '{"IV_NAME":"Lawang","CV_COUNTER":42}'
./bin/rfc-call -function Z_LAWANG_DATA -describe
./bin/rfc-call -function Z_LAWANG_DATA -params '{"IS_HEADER":{"CODE":"A001","QUANTITY":2,"PRICE":12.5},"T_ECHO":[{"CODE":"B002","QUANTITY":3,"PRICE":7.25}]}'
```

Angka pada `-params` dikonversi menurut metadata RFC, termasuk field di dalam struktur dan tabel. Gunakan literal integer seperti `2`, `-3`, atau `2147483647` untuk `INT`; pecahan, notasi eksponen pada `INT`, dan nilai di luar rentang ditolak dengan lokasi parameter (misalnya `T_ECHO[0].QUANTITY`). Untuk `FLOAT`, input `7`, `12.5`, dan `1.25e2` dapat digunakan. String seperti `"0002"` tetap berupa string.

Default `-host 127.0.0.1 -port 3313 -transport classic`. Client mengonversi port `3300..3399` menjadi system number dan memakai `ashost`/`sysnr` saja. Kegagalan menghasilkan exit code nonzero. `-describe` mengambil metadata melalui jaringan.

## Verifikasi dan batas kompatibilitas

```sh
task check SAPNWRFC_HOME="$HOME/nwrfcsdk"
task test SAPNWRFC_HOME="$HOME/nwrfcsdk"
# integration test, server mock.yml harus berjalan sebagai proses terpisah:
RFC_MOCK_INTEGRATION=1 task test SAPNWRFC_HOME="$HOME/nwrfcsdk"
```

Pengujian native mencakup ping, metadata/DDIC, Unicode, scalar, echo struktur, output tabel statis, tabel kosong, 128 baris input, dan empat koneksi paralel. Server dan client diuji dengan race detector.

`betteralign` diperlukan untuk `task check`. Integration test dilewati tanpa `RFC_MOCK_INTEGRATION=1`; `RFC_MOCK_PORT` dapat mengubah port testing dalam rentang 3300–3399.

Target kompatibilitas adalah synchronous RFC yang dipakai gonwrfc/SAP SDK dengan schema yang didukung. Ini bukan emulasi penuh SAP application server atau produk SAP tersertifikasi. Belum mendukung SNC, autentikasi SAP, message-server/load-balancing, tRFC/qRFC, SAP LZH compressed table input, transactional BAPI semantics, `DATE`/`BCD`, nested/deep types, atau arbitrary ABAP exceptions. Nama function BAPI bisa dimock, tetapi efek transaksi hanya sebatas output YAML.

Mode lama `transport: websocket` tetap tersedia untuk function scalar dengan `port: 8088`; client dapat memakai `-transport websocket -port 8088`. Mode itu tidak digunakan untuk koneksi Lawang-Core klasik di atas, dan parameter kompleks hanya diaktifkan pada transport `classic`.
