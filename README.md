# Cuanku

Platform manajemen pengguna WPA2-Enterprise untuk penyedia layanan internet nirkabel (WISP):
pelanggan mengelola kredensial WiFi sendiri, AccessPoint dipantau via SNMP, pembayaran online
mengaktifkan/mensuspend layanan lewat VLAN dinamis FreeRADIUS.

- **`CONTEXT.md`** — glossary domain (istilah yang dipakai kode dan dokumen).
- **`docs/adr/`** — keputusan arsitektur yang sulit dibalik.
- **`docs/plans/0001-mvp-scope.md`** — tujuan, batas MVP, kriteria sukses.
- **`docs/research/`** — bukti teknis (FreeRADIUS, hostapd, SNMP, dukungan perangkat).
- **`docs/validation/0001-suspend-vlan-lab.md`** — runbook validasi lab (gerbang issue #6–#14).

## Struktur repositori

```
backend/    Go + Uber Fx + Gin + GORM + PostgreSQL (layout DDD: internal/application/<domain>/)
frontend/   SvelteKit (Svelte 5 runes) + Tailwind v4 + shadcn-svelte + TanStack Query + zod
docs/       ADR, riset, rencana, runbook validasi
```

Keduanya adalah aplikasi terpisah dengan toolchain terpisah: `make` di `backend/`, `pnpm` di
`frontend/`. Tidak ada `docker compose` — PostgreSQL dev sudah tersedia di VM (lihat `AGENTS.md`).

## Prasyarat

| Alat | Versi |
|---|---|
| Go | 1.25+ |
| Node | 22+ |
| pnpm | 10+ |
| PostgreSQL dev | VM `deb12` (lihat `AGENTS.md`) |

## Backend

DSN **selalu** dari environment; tidak ada alamat atau kredensial yang di-hardcode:

```bash
cd backend
export DATABASE_URL='postgres://<user>:<pass>@<vm-host>:5432/cuanku?sslmode=disable'

make migrate-up      # menerapkan migrasi (idempoten)
make migrate-version # versi skema saat ini
make run             # API di :8080
```

`make help` mencantumkan semua target. Verifikasi sebelum menyerahkan pekerjaan:

```bash
make build && make test && make lint   # atau: make ci
```

Health check:

```bash
curl -s localhost:8080/healthz | jq
# 200 {"status":"ok","service":"cuanku-api","version":"0.1.0","database":"up","checked_at":"..."}
# 503 {"status":"degraded",...,"database":"down"} bila PostgreSQL tidak terjangkau
```

`/healthz` sengaja berada di luar `/api/v1` supaya probe tidak ikut berubah saat versi API naik.

`Dockerfile` adalah artefak deploy (multi-stage, `CGO_ENABLED=0`, jalan sebagai non-root). Mesin
dev tidak punya container runtime, jadi image-nya tidak dibangun maupun diverifikasi di sini —
target `make docker-build` dan `make docker-run` ada untuk itu, tetapi sengaja tidak masuk CI.

### Migrasi

Migrasi adalah berkas SQL berversi di `backend/migrations/` (`<versi>_<nama>.up.sql` /
`.down.sql`), dijalankan `cmd/migration` lewat golang-migrate. Bukan GORM AutoMigrate, karena
aplikasi ini juga menulis ke tabel milik FreeRADIUS (`radcheck`, `radreply`, `radacct`) dan
perubahan skema harus eksplisit serta bisa di-review (ADR-0005).

## Frontend

```bash
cd frontend
cp .env.example .env      # BACKEND_URL, PUBLIC_*
pnpm dev                  # http://localhost:5173
```

`/` dialihkan ke `/status`, yang menampilkan hasil pemeriksaan **frontend → backend → PostgreSQL**.
Halaman itu membaca `/api/health` — sebuah route milik SvelteKit sendiri yang meneruskan ke
`/healthz` milik Go (pola BFF: alamat backend dan, nanti, session cookie httpOnly tidak pernah
sampai ke JavaScript klien).

`/api/health` selalu menjawab 200 dengan amplop `{ backend_reachable, health, checked_at }`, bukan
meneruskan 503, supaya UI bisa membedakan **"API mati"** dari **"API hidup, basis datanya mati"**.
Amplop itu hanya boleh berubah bersama skema zod di
`frontend/src/lib/domains/health/schemas/health.schema.ts` — backend Go tidak mengetahuinya.

Verifikasi sebelum menyerahkan pekerjaan:

```bash
pnpm check && pnpm lint && pnpm test && pnpm build
```

## CI

`.github/workflows/ci.yml` menjalankan dua job. Backend: build, vet, golangci-lint, **migrasi
terhadap PostgreSQL service container** (sekali ke DB kosong, sekali lagi untuk membuktikan
re-run tidak mengubah apa pun), lalu tes dengan `-race`. Frontend: install, check, lint, test,
build. Tidak ada kredensial nyata di CI — DSN-nya hanya hidup di dalam runner.

## Konvensi

Arsitektur, aturan layering, dan naming ada di `AGENTS.md`. Ringkasnya: satu domain = satu slice
vertikal (`backend/internal/application/<domain>/{dto,entity,repository,service,handler}` dan
`frontend/src/lib/domains/<domain>/{schemas,api,queries,components,index.ts}`), dependensi selalu
satu arah, dan handler/route tidak pernah memuat logika bisnis.
