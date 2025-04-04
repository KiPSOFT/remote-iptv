# Remote IPTV Control System

Web tabanlı IPTV kontrol sistemi. Bu sistem, web arayüzü üzerinden IPTV kanallarını yönetmenizi sağlar.

## Özellikler

- Web tabanlı kontrol arayüzü
- MPV medya oynatıcısı entegrasyonu
- Xtream Codes API desteği
- Favori kanal yönetimi
- M3U playlist desteği

## Gereksinimler

- Go 1.21 veya üzeri
- MPV medya oynatıcısı
- SQLite3
- Node.js ve npm (web arayüzü için)

## Kurulum

1. Go'yu yükleyin:
   ```bash
   # macOS için
   brew install go
   ```

2. MPV'yi yükleyin:
   ```bash
   # macOS için
   brew install mpv
   ```

3. Bağımlılıkları yükleyin:
   ```bash
   go mod download
   ```

4. Web arayüzü bağımlılıklarını yükleyin:
   ```bash
   cd web
   npm install
   ```

## Çalıştırma

1. Backend sunucusunu başlatın:
   ```bash
   go run cmd/server/main.go
   ```

2. Web arayüzünü geliştirme modunda başlatın:
   ```bash
   cd web
   npm start
   ```

## Yapılandırma

Sistem varsayılan olarak aşağıdaki portları kullanır:
- Backend API: 8080
- Web arayüzü: 3000

## Lisans

MIT 