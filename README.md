# AutoRoot

Aplikasi desktop **offline** untuk otomatisasi rooting HP Android. Semua proses berjalan lokal di komputer kamu (Electron + React + backend Go) — tidak butuh koneksi internet saat dipakai.

- **Samsung** → alur Odin (semi-manual: patch Magisk + klik Start di Odin)
- **OnePlus / Google / Xiaomi / Motorola / Nothing** → alur fastboot _(belum aktif)_

---

## Kebutuhan

Sebelum pakai, pastikan sudah terpasang:

- **Windows 10/11 (x64)**
- **Android Platform Tools** (`adb` harus ada di PATH) — untuk alur Samsung wajib
- **Samsung USB Driver** — supaya HP kebaca saat Download Mode & Odin
- **fastboot** di PATH (hanya untuk HP non-Samsung)
- **Firmware** yang cocok untuk model HP kamu (`.zip`), taruh di `%APPDATA%\AutoRoot\firmware`
- **Magisk** — sudah dibundel di aplikasi (`Magisk-v30.7.apk`)

---

## Instalasi & Build

```bash
npm install

# jalankan mode development (frontend + Electron)
npm run dev

# build backend Go + frontend
npm run build:backend:win
npm run build:frontend

# bikin installer
npm run package:win     # → ./release/AutoRoot Setup [version].exe
```

Perintah lain:

| Perintah | Fungsi |
|---|---|
| `npm run package:mac` / `package:linux` | Build installer untuk macOS / Linux |
| `npm run clean` | Bersihkan `bin/`, `frontend/dist/`, `release/` |

---

## Cara Pakai (Samsung)

### 1. Colok USB ke HP

1. Buka aplikasi AutoRoot.
2. Colok kabel USB ke HP.
3. Kalau HP belum terdeteksi, layar awal menampilkan panduan **Enable USB Debugging**:
   - **Settings** → **About Phone** → tap **Build Number** 7x
   - Balik → **Developer Options** → aktifkan **USB Debugging**
   - Klik tombol **"I've Enabled USB Debugging"**
4. Di layar HP muncul **"Allow USB Debugging?"** → tap **Allow**.
5. Aplikasi cek HP tiap 2 detik (timeout 60 detik).

### 2. Layar "Device Connected"

Setelah HP terbaca, aplikasi menampilkan brand, model, versi, dan status root, lalu otomatis mengecek ketersediaan firmware:

- **Firmware belum ada** → klik **"📂 Select Firmware File"** atau drag & drop file `.zip`.
- **Firmware ada & belum root** → klik **"🔥 Root Samsung"**.

### 3. Proses Otomatis (HP masih nyala normal)

Setelah klik **Root Samsung**, aplikasi berjalan sendiri:

1. Layar HP dijaga tetap nyala.
2. Magisk otomatis di-install kalau belum ada.
3. Firmware `.zip` diekstrak, lalu dicari file **AP / BL / CP / CSC**.
4. File **AP** dikirim ke HP (`/sdcard/Download/AP_file.tar`).

### 4. ⚠️ Manual #1 — Patch Magisk (di HP)

Aplikasi menampilkan instruksi. Di HP:

1. Buka **Magisk** → **Install** → **Select & Patch a File**
2. Pilih **`AP_file.tar`**
3. Tunggu sampai muncul **"All done!"**

Di aplikasi, klik tombol kuning **"✅ I've Patched the File, Continue"**.

### 5. Proses Otomatis Lagi

1. File AP yang sudah dipatch ditarik dari HP.
2. HP di-reboot ke **Download Mode** → **tekan Volume UP** di HP.
3. Tunggu sekitar 15 detik.

### 6. ⚠️ Manual #2 — Odin (di Windows)

> Odin tidak punya mode command-line, jadi langkah flash-nya manual. (Aplikasi otomatis membuka GUI Odin untuk kamu.)

1. Tunggu **ID:COM** berubah **biru** (HP terdeteksi).
2. Klik **AP** → pilih file AP patched (Explorer otomatis menyorot filenya).
3. Pastikan **Auto Reboot** dan **F.Reset Time** tercentang.
4. Klik **Start** → tunggu sampai **PASS!** (hijau).

Di aplikasi, klik tombol **"✅ Odin Finished, Verify Root"**.

### 7. Verifikasi Otomatis

Aplikasi menunggu HP selesai boot, lalu mengecek root (`su -c id`):

- Berhasil → **"✅ Root verified"** dan HP ditandai sebagai rooted.
- Gagal → cek HP secara manual, lalu coba lagi.

---

## Catatan Penting

- **Jangan lepas kabel di tengah proses.** Saat rooting berjalan, aplikasi sengaja tidak mereset status walau HP sementara hilang dari ADB (misal saat Download Mode).
- **Verifikasi root bisa lama** (sampai ~10 menit) karena menunggu HP selesai boot pertama kali.
- **Non-Samsung belum bisa dipakai** — alur fastboot belum diimplementasikan.
- Aplikasi **100% offline** saat dipakai. Satu-satunya bagian yang butuh internet adalah unduhan firmware (Google Drive) — itu sebabnya firmware perlu sudah tersedia lokal di `%APPDATA%\AutoRoot\firmware`.

---

## Struktur Proyek

| Folder | Isi |
|---|---|
| `electron/` | Main process Electron — spawn backend Go + jembatan IPC |
| `frontend/` | UI React + Vite |
| `backend/` | Engine Go (ADB / fastboot / Odin / ekstrak firmware) |
| `odin/` | `Odin3.exe` (bundled) |
| `setup/` | Wizard setup firmware + `firmware-db.json` |
| `scripts/` | Script installer firmware (PowerShell) |
| `db/` | Skema SQLite (belum dipakai) |
