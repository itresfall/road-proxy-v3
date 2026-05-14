# 📋 MİT (Stratejik Plan) Raporu
## Universal Game Proxy Engine - Yeniden Yapılandırma

**Hazırlanma Tarihi:** 2025  
**Versiyon:** 2.0  
**Durum:** Kapsamlı Analiz ve Detaylı Yol Haritası

---

## 📊 EXECUTIVE SUMMARY

Bu rapor, **Universal Game Proxy Engine** projesinin mevcut durumunu, tüm öğrenilen dersleri, kritik sorunları ve gelecek için detaylı bir stratejiyi içermektedir. Proje, başlangıçtan itibaren büyük ilerleme kaydetmiş ancak mimari, performans ve plugin sistemi açısından yeniden yapılandırma gereklidir.

### Kritik Bulgular

✅ **Başarılar:**
- TCP proxy sistemi çalışır durumda
- WebSocket tabanlı Cloudflare bypass başarılı
- Plugin mimarisi temel seviyede mevcut
- V2Ray-direct benzeri basit obfuscation çalışıyor

⚠️ **Kritik Sorunlar:**
- Plugin sistemi JSON tabanlı değil (sadece metadata JSON)
- Remote plugin download eksik (server → client)
- TCP optimizasyonları uygulanmış ama tam optimize değil
- UDP desteği %1 seviyesinde (Cloudflare kısıtı)
- Senkronizasyon problemleri var
- V2Ray-direct basit XOR+Base64 (gerçek V2Ray entegrasyonu yok)

---

## 🔍 MEVCUT DURUM ANALİZİ

### 1. TCP Optimizasyonları (Uygulanan)

#### 1.1. Buffer Optimizasyonları
**Durum:** ✅ Uygulanmış

| Özellik | Önceki | Şimdi | İyileşme |
|---------|--------|-------|----------|
| Buffer Size | 4KB | 32KB | %800 artış |
| Buffer Pool | Yok | Var (sync.Pool) | Memory allocation azaldı |
| ReadBufferSize | Varsayılan | 32KB | Optimize |
| WriteBufferSize | Varsayılan | 32KB | Optimize |

**Kod Referansı:**
```go
// engine/engine.go - NewProxyEngine
wsUpgrader: websocket.Upgrader{
    ReadBufferSize:  32768,  // 32KB
    WriteBufferSize: 32768,  // 32KB
}

// engine/engine.go - Buffer Pool
e.bufferPool = sync.Pool{
    New: func() any {
        return make([]byte, 32768)  // 32KB reusable buffers
    },
}
```

#### 1.2. Network Stack Optimizasyonları
**Durum:** ✅ Kısmen Uygulanmış

| Özellik | Durum | Değer |
|---------|-------|-------|
| TCP_NODELAY | ✅ | true (SetNoDelay) |
| Keep-Alive | ✅ | true (30s) |
| ReadHeaderTimeout | ✅ | 5s |
| WriteTimeout | ✅ | 10s |
| IdleTimeout | ✅ | ConnectionTimeout * 2 |

**Kod Referansı:**
```go
// engine/session.go - handleTCPConnection
if tcpCli, ok := clientConn.(*net.TCPConn); ok {
    _ = tcpCli.SetNoDelay(true)
    _ = tcpCli.SetKeepAlive(true)
    _ = tcpCli.SetKeepAlivePeriod(30 * time.Second)
}
```

#### 1.3. Logging ve Statistics Optimizasyonları
**Durum:** ✅ Uygulanmış

| Özellik | Önceki | Şimdi | İyileşme |
|---------|--------|-------|----------|
| Log Frequency | Her paket | Her 50 paket | %98 azalma |
| Stats Update | Her paket | Her 10 paket | %90 azalma |
| Session Update | Her paket | Her 200 paket | %99.5 azalma |

**Kod Referansı:**
```go
// engine/session.go - handleTCPProxy
packetCount++
if packetCount%100 == 0 {
    e.stats.mu.Lock()
    e.stats.TotalBytesTx += uint64(len(processedData) * 100)
    e.stats.mu.Unlock()
}
```

#### 1.4. Fast-Path Optimizasyonu
**Durum:** ⚠️ Kısmen Uygulanmış

**Mevcut Durum:**
- ✅ Tek plugin modunda protokol tespiti bypass ediliyor
- ✅ Passthrough modunda io.CopyBuffer kullanılıyor
- ❌ WebSocket path'de passthrough yok
- ❌ Plugin kancaları hala çağrılıyor (gereksiz)

**Kod Referansı:**
```go
// engine/session.go - handleTCPConnection
e.pluginMutex.RLock()
if len(e.activePlugins) == 1 {
    for name := range e.activePlugins {
        pluginName = name
        break
    }
}
e.pluginMutex.RUnlock()

// Passthrough check
if !cfg.EnableObfuscation {
    passthrough = true
    // io.CopyBuffer kullan
}
```

#### 1.5. Performans Metrikleri

**Mevcut Sonuçlar:**
- **Tek Oyuncu Ping:** 40-51ms (hedef: 30-35ms)
- **2. Oyuncu Ping:** >90ms (hedef: <90ms)
- **Throughput:** ~1000 byte/s (hedef: 8000+ byte/s)
- **Memory:** Yüksek allocation (hedef: %60-70 azalma)
- **CPU:** Yüksek usage (hedef: %40-50 azalma)

**Hedef vs Mevcut:**
| Metrik | Mevcut | Hedef | Durum |
|--------|--------|-------|-------|
| Tek Oyuncu Ping | 40-51ms | 30-35ms | ❌ %25-40 fark |
| Multi-User Ping | >90ms | <90ms | ❌ Hala yüksek |
| Throughput | 1000 B/s | 8000+ B/s | ❌ %87.5 eksik |
| Memory | Yüksek | %60-70 az | ⚠️ Kısmen |
| CPU | Yüksek | %40-50 az | ⚠️ Kısmen |

---

### 2. V2Ray-Direct Benzeri Uygulamalar

#### 2.1. Mevcut Implementasyon
**Durum:** ⚠️ Basit Obfuscation (Gerçek V2Ray Değil)

**Mevcut Sistem:**
```go
// cmd/v2ray-direct/main.go - encodeObfuscated
func (v *V2RayDirectServer) encodeObfuscated(data []byte) []byte {
    // XOR encryption
    encrypted := make([]byte, len(data))
    for i, b := range data {
        encrypted[i] = b ^ v.encryptionKey[i%len(v.encryptionKey)]
    }
    // Base64 encode
    return []byte(base64.StdEncoding.EncodeToString(encrypted))
}
```

**Sorunlar:**
1. ❌ Gerçek V2Ray kütüphanesi kullanılmıyor
2. ❌ Sadece XOR+Base64 (güvenlik zayıf)
3. ❌ V2Ray'ın tüm şifreleme modülleri yok
4. ❌ WebSocket transport entegrasyonu eksik

**V2Ray-Direct Neden Seçildi?**
- Cloudflare bypass için basit ve etkili
- WebSocket üzerinden çalışıyor
- HTTP camouflage ile normal web trafiği gibi görünüyor
- Ücretsiz ve basit implementasyon

**Önerilen İyileştirme:**
1. V2Ray-core entegrasyonu (opsiyonel, karmaşık)
2. Daha güçlü şifreleme (AES-256-GCM)
3. Obfuscation modları (VMess, VLESS style)

---

### 3. Senkronizasyon Problemleri

#### 3.1. WebSocket Senkronizasyon Sorunları (Çözüldü)
**Durum:** ✅ Çözüldü (Session-based architecture)

**Önceki Sorun:**
- Concurrent read/write sorunları
- WebSocket close 1006 errors
- Multi-client desteği yoktu

**Çözüm:**
- Session-based architecture
- Her session için ayrı WebSocket
- Bidirectional goroutines ile çözüldü

**Kod Referansı:**
```go
// engine/handlers.go - handleWebSocketProxy
// WebSocket -> Game Server
go func() {
    for {
        _, data, err := wsConn.ReadMessage()
        // Process and send to server
    }
}()

// Game Server -> WebSocket  
go func() {
    for {
        n, err := serverConn.Read(buffer)
        // Process and send to WebSocket
    }
}()
```

#### 3.2. TCP Senkronizasyon Sorunları (Kısmen Çözüldü)
**Durum:** ⚠️ İyileştirildi ama hala sorunlar var

**Mevcut Sorunlar:**
1. **Plugin Kancaları:** Her pakette çağrılıyor (overhead)
2. **Stats Locking:** Mutex contention hala var
3. **First Packet Handling:** İlk paket için özel işlem gerekli

**Çözümler:**
- ✅ Buffer pooling ile memory allocation azaldı
- ✅ Batch statistics updates
- ⚠️ Plugin kancaları hala her pakette (passthrough modunda bypass var)

#### 3.3. UDP Senkronizasyon (Eksik)
**Durum:** ❌ Çözülmedi

**Sorun:**
- UDP connectionless doğası
- Her paket bağımsız
- Session yönetimi zor
- Cloudflare UDP desteği yok

---

## 🏗️ PLUGIN SİSTEMİ ANALİZİ

### 4. Mevcut Plugin Sistemi

#### 4.1. Plugin Yapısı (Şu An)

**Dosya Yapısı:**
```
plugins/
├── minecraft/
│   └── plugin.json        # Sadece metadata
├── valve/
│   └── plugin.json
└── half-life/
    ├── plugin.json
    ├── config.json
    └── half_life.go       # Go kodu (build edilmiş olmalı)
```

**Plugin.json Formatı (Mevcut):**
```json
{
  "name": "minecraft",
  "version": "2.0.0",
  "description": "Minecraft TCP Proxy Plugin",
  "author": "Universal Game Proxy Engine",
  "supported_protocols": ["tcp"],
  "game_name": "Minecraft",
  "default_port": 25567,
  "target_addr": "127.0.0.1:25567",
  "listen_addr": "127.0.0.1:25567",
  "enabled": true,
  "config": {
    "enable_obfuscation": false,
    "timeout": "5s",
    "buffer_size": 32768,
    "log_level": "info"
  }
}
```

**Sorunlar:**
1. ❌ Plugin.json sadece metadata (config ayrı dosya)
2. ❌ Plugin loader JSON'dan plugin oluşturamıyor
3. ❌ Built-in plugin'ler kod içinde hard-coded
4. ❌ Remote plugin download yok

#### 4.2. Plugin Loader (Mevcut)

**Mevcut Durum:**
```go
// engine/plugin_loader.go
type PluginLoader struct {
    plugins   map[string]GamePlugin
    adapters  map[string]PluginAdapter
    registry  *PluginRegistry
    pluginDir string
    debugMode bool
}
```

**Yüklenen Plugin Tipleri:**
1. **Static Plugins:** Global registry'den (hard-coded)
2. **JSON Plugins:** plugin.json dosyalarından (sadece metadata)
3. **Dynamic Plugins:** .so/.go dosyalarından (Windows'ta çalışmıyor)

**Sorunlar:**
1. ❌ JSON plugin'ler sadece metadata taşıyor (işlevsel değil)
2. ❌ Plugin.json'dan çalışan plugin oluşturulamıyor
3. ❌ Remote plugin yükleme eksik

---

## 📐 YENİ PLUGIN SİSTEMİ TASARIMI

### 5. JSON Tabanlı Plugin Sistemi

#### 5.1. Hedef Yapı

**Dosya Yapısı (Hedef):**
```
proxy.exe (executable)
└── plugins/                    # Otomatik oluşturulacak
    ├── minecraft/
    │   ├── plugin.json         # Plugin tanımı + config
    │   ├── config.json         # Runtime config (opsiyonel)
    │   └── minecraft.so        # Compiled plugin (opsiyonel, built-in için yok)
    ├── valve/
    │   ├── plugin.json
    │   └── config.json
    └── .cache/                 # Remote plugin cache
        └── downloaded-plugins/
```

#### 5.2. Plugin.json Formatı (Yeni)

**Tam Format:**
```json
{
  "name": "minecraft",
  "version": "2.0.0",
  "description": "Minecraft TCP/UDP Proxy Plugin",
  "author": "Universal Game Proxy Engine",
  "license": "AGPL-3.0-only",
  
  "metadata": {
    "game_name": "Minecraft",
    "game_icon": "minecraft.png",
    "supported_versions": ["1.19", "1.20", "1.21"]
  },
  
  "protocols": {
    "supported": ["tcp", "websocket"],
    "primary": "tcp",
    "default_port": 25567
  },
  
  "config": {
    "server_addr": {
      "type": "string",
      "default": "127.0.0.1:25565",
      "description": "Target Minecraft server address",
      "required": true
    },
    "listen_addr": {
      "type": "string",
      "default": "127.0.0.1:25567",
      "description": "Local listen address",
      "required": false
    },
    "enable_obfuscation": {
      "type": "boolean",
      "default": false,
      "description": "Enable packet obfuscation"
    },
    "buffer_size": {
      "type": "integer",
      "default": 32768,
      "min": 1024,
      "max": 65535,
      "description": "Buffer size in bytes"
    },
    "timeout": {
      "type": "duration",
      "default": "5s",
      "description": "Connection timeout"
    }
  },
  
  "runtime": {
    "type": "builtin",
    "builtin_handler": "minecraft",
    "entry_point": ""
  },
  
  "remote": {
    "download_url": "",
    "checksum": "",
    "signature": ""
  },
  
  "dependencies": [],
  
  "hooks": {
    "on_session_start": false,
    "on_session_end": false,
    "process_client_data": false,
    "process_server_data": false
  }
}
```

#### 5.3. Plugin Yükleme Mantığı

**1. Başlangıç:**
```
proxy.exe başlatıldı
  ↓
plugins/ klasörü var mı?
  ├─ Yok → Oluştur
  └─ Var → Devam
  ↓
plugins/ altındaki klasörleri tara
  ↓
Her klasörde plugin.json var mı?
  ├─ Var → Yükle
  └─ Yok → Skip
```

**2. Plugin Yükleme Sırası:**
```
1. plugin.json'u oku
2. Config schema'yı validate et
3. Runtime type'ı kontrol et:
   ├─ "builtin" → Built-in handler kullan
   ├─ "so" → .so dosyasından yükle
   ├─ "remote" → Remote'dan indir
   └─ "json" → JSON-only plugin (basit proxy)
4. Config'i yükle (config.json varsa, yoksa default)
5. Plugin'i registry'ye ekle
```

**3. Built-in Plugin Handler:**
```go
// engine/plugin_factory.go
type BuiltinPluginHandler interface {
    CreatePlugin(config *PluginConfig) (GamePlugin, error)
}

var builtinHandlers = map[string]BuiltinPluginHandler{
    "minecraft": &MinecraftPluginHandler{},
    "basic-tcp": &BasicTCPPluginHandler{},
    "udp-forward": &UDPForwardPluginHandler{},
}
```

#### 5.4. Remote Plugin Download (Server → Client)

**Mimari:**
```
Client bağlanır → Server plugin listesi döner
  ↓
Client plugin seçer → Server'dan plugin.json + config indirir
  ↓
Client plugins/ klasörüne kaydeder
  ↓
Client plugin'i yükler ve kullanır
```

**API Endpoint'leri:**

1. **Plugin Listesi:**
```
GET /api/plugins
Response:
{
  "plugins": [
    {
      "name": "minecraft",
      "version": "2.0.0",
      "description": "...",
      "download_url": "/api/plugin/download/minecraft",
      "info_url": "/api/plugin/info/minecraft"
    }
  ]
}
```

2. **Plugin Bilgisi:**
```
GET /api/plugin/info/{pluginName}
Response: plugin.json içeriği
```

3. **Plugin İndirme:**
```
GET /api/plugin/download/{pluginName}
Response: 
  - Content-Type: application/json
  - Body: plugin.json + config.json (ZIP veya JSON bundle)
```

4. **Plugin Config:**
```
GET /api/plugin/config/{pluginName}
Response: config.json içeriği
```

**Client Tarafı:**
```go
// client/plugin_downloader.go
type PluginDownloader struct {
    serverURL string
    pluginDir string
}

func (pd *PluginDownloader) DownloadPlugin(pluginName string) error {
    // 1. Plugin info al
    info, err := pd.getPluginInfo(pluginName)
    
    // 2. Plugin klasörü oluştur
    pluginPath := filepath.Join(pd.pluginDir, pluginName)
    os.MkdirAll(pluginPath, 0755)
    
    // 3. plugin.json indir ve kaydet
    pluginJSON, err := pd.downloadFile(fmt.Sprintf("%s/api/plugin/info/%s", pd.serverURL, pluginName))
    os.WriteFile(filepath.Join(pluginPath, "plugin.json"), pluginJSON, 0644)
    
    // 4. config.json indir (varsa)
    configJSON, err := pd.downloadFile(fmt.Sprintf("%s/api/plugin/config/%s", pd.serverURL, pluginName))
    if err == nil {
        os.WriteFile(filepath.Join(pluginPath, "config.json"), configJSON, 0644)
    }
    
    return nil
}
```

---

## 🎯 DETAYLI YOL HARİTASI

### Faz 1: Plugin Sistemi Yeniden Yapılandırma

#### ✅ Faz 1.1: Plugin Dizin Yapısı
**Süre:** 1 gün  
**Öncelik:** YÜKSEK

**Adımlar:**
- [ ] `plugins/` klasörü otomatik oluşturma
- [ ] Plugin dizin yapısı doğrulama
- [ ] Plugin.json dosya kontrolü
- [ ] Hata yönetimi (eksik/geçersiz plugin)

**Kod Değişiklikleri:**
```go
// engine/plugin_loader.go
func (pl *PluginLoader) ensurePluginDir() error {
    if _, err := os.Stat(pl.pluginDir); os.IsNotExist(err) {
        if err := os.MkdirAll(pl.pluginDir, 0755); err != nil {
            return fmt.Errorf("plugin dizini oluşturulamadı: %v", err)
        }
        log.Printf("✅ Plugin dizini oluşturuldu: %s", pl.pluginDir)
    }
    return nil
}
```

#### ✅ Faz 1.2: Yeni Plugin.json Formatı
**Süre:** 2 gün  
**Öncelik:** YÜKSEK

**Adımlar:**
- [ ] Yeni JSON schema tanımı
- [ ] JSON validation
- [ ] Backward compatibility (eski format desteği)
- [ ] Config schema parsing
- [ ] Default değer yükleme

**Kod Değişiklikleri:**
```go
// engine/plugin_schema.go
type PluginSchema struct {
    Name        string                 `json:"name"`
    Version     string                 `json:"version"`
    Description string                 `json:"description"`
    Author      string                 `json:"author"`
    Metadata    map[string]interface{} `json:"metadata"`
    Protocols   ProtocolConfig         `json:"protocols"`
    Config      map[string]ConfigField `json:"config"`
    Runtime     RuntimeConfig          `json:"runtime"`
    Remote      RemoteConfig           `json:"remote"`
    Hooks       HookConfig             `json:"hooks"`
}

type ConfigField struct {
    Type        string      `json:"type"`
    Default     interface{} `json:"default"`
    Description string      `json:"description"`
    Required    bool        `json:"required"`
    Min         *int        `json:"min,omitempty"`
    Max         *int        `json:"max,omitempty"`
}

func (pl *PluginLoader) loadPluginSchema(pluginPath string) (*PluginSchema, error) {
    jsonPath := filepath.Join(pluginPath, "plugin.json")
    data, err := os.ReadFile(jsonPath)
    // ... parse and validate
}
```

#### ✅ Faz 1.3: Built-in Plugin Handler Sistemi
**Süre:** 2 gün  
**Öncelik:** YÜKSEK

**Adımlar:**
- [ ] Built-in handler interface
- [ ] Handler registry
- [ ] Minecraft handler implementation
- [ ] Basic-TCP handler implementation
- [ ] Handler factory pattern

**Kod Değişiklikleri:**
```go
// engine/builtin_handlers.go
type BuiltinPluginHandler interface {
    CreatePlugin(config *PluginConfig) (GamePlugin, error)
    ValidateConfig(config map[string]interface{}) error
}

var builtinHandlers = map[string]BuiltinPluginHandler{
    "minecraft": &MinecraftHandler{},
    "basic-tcp": &BasicTCPHandler{},
}

func CreateBuiltinPlugin(pluginName string, config *PluginConfig) (GamePlugin, error) {
    handler, exists := builtinHandlers[pluginName]
    if !exists {
        return nil, fmt.Errorf("builtin handler bulunamadı: %s", pluginName)
    }
    return handler.CreatePlugin(config)
}
```

#### ✅ Faz 1.4: Config Yönetimi
**Süre:** 1 gün  
**Öncelik:** ORTA

**Adımlar:**
- [ ] Config schema'dan default config oluşturma
- [ ] Config.json dosyası yükleme/kaydetme
- [ ] Config validation
- [ ] Config merge (schema defaults + config.json)

**Kod Değişiklikleri:**
```go
// engine/config_manager.go
func (cm *ConfigManager) LoadPluginConfig(pluginName string, schema *PluginSchema) (*PluginConfig, error) {
    // 1. Schema'dan default config oluştur
    defaultConfig := cm.createDefaultConfig(schema)
    
    // 2. config.json varsa yükle
    configPath := filepath.Join(cm.pluginDir, pluginName, "config.json")
    if _, err := os.Stat(configPath); err == nil {
        fileConfig, err := cm.loadConfigFile(configPath)
        // 3. Merge (file config overrides defaults)
        return cm.mergeConfig(defaultConfig, fileConfig), nil
    }
    
    return defaultConfig, nil
}
```

---

### Faz 2: Remote Plugin Download

#### ✅ Faz 2.1: Server API Endpoint'leri
**Süre:** 2 gün  
**Öncelik:** YÜKSEK

**Adımlar:**
- [ ] `/api/plugins` - Plugin listesi
- [ ] `/api/plugin/info/{name}` - Plugin bilgisi
- [ ] `/api/plugin/config/{name}` - Plugin config
- [ ] `/api/plugin/download/{name}` - Plugin bundle
- [ ] Error handling
- [ ] Security (rate limiting, authentication)

**Kod Değişiklikleri:**
```go
// engine/handlers.go
func (e *ProxyEngine) handlePluginList(w http.ResponseWriter, r *http.Request) {
    plugins := []PluginInfo{}
    
    // plugins/ klasöründeki tüm plugin'leri listele
    entries, _ := os.ReadDir(e.config.PluginDir)
    for _, entry := range entries {
        if entry.IsDir() {
            pluginJSON, err := e.loadPluginJSON(entry.Name())
            if err == nil {
                plugins = append(plugins, PluginInfo{
                    Name:        pluginJSON.Name,
                    Version:     pluginJSON.Version,
                    Description: pluginJSON.Description,
                    DownloadURL: fmt.Sprintf("/api/plugin/download/%s", pluginJSON.Name),
                })
            }
        }
    }
    
    json.NewEncoder(w).Encode(map[string]interface{}{
        "plugins": plugins,
    })
}

func (e *ProxyEngine) handlePluginDownload(w http.ResponseWriter, r *http.Request) {
    pluginName := extractPluginName(r.URL.Path)
    
    // plugin.json + config.json bundle oluştur
    bundle := PluginBundle{
        PluginJSON: e.loadPluginJSONContent(pluginName),
        ConfigJSON: e.loadConfigJSONContent(pluginName), // optional
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(bundle)
}
```

#### ✅ Faz 2.2: Client Plugin Downloader
**Süre:** 2 gün  
**Öncelik:** YÜKSEK

**Adımlar:**
- [ ] Plugin listesi alma
- [ ] Plugin indirme
- [ ] Plugin klasörüne kaydetme
- [ ] İndirilen plugin'i yükleme
- [ ] Hata yönetimi
- [ ] Retry mekanizması

**Kod Değişiklikleri:**
```go
// client/plugin_downloader.go
type PluginDownloader struct {
    serverURL string
    pluginDir string
    client    *http.Client
}

func (pd *PluginDownloader) ListAvailablePlugins() ([]PluginInfo, error) {
    resp, err := pd.client.Get(fmt.Sprintf("%s/api/plugins", pd.serverURL))
    // ... parse response
}

func (pd *PluginDownloader) DownloadPlugin(pluginName string) error {
    // 1. Plugin info al
    infoURL := fmt.Sprintf("%s/api/plugin/info/%s", pd.serverURL, pluginName)
    pluginJSON, err := pd.downloadJSON(infoURL)
    
    // 2. Config al (varsa)
    configURL := fmt.Sprintf("%s/api/plugin/config/%s", pd.serverURL, pluginName)
    configJSON, _ := pd.downloadJSON(configURL) // ignore error, optional
    
    // 3. Plugin klasörü oluştur
    pluginPath := filepath.Join(pd.pluginDir, pluginName)
    os.MkdirAll(pluginPath, 0755)
    
    // 4. Dosyaları kaydet
    os.WriteFile(filepath.Join(pluginPath, "plugin.json"), pluginJSON, 0644)
    if configJSON != nil {
        os.WriteFile(filepath.Join(pluginPath, "config.json"), configJSON, 0644)
    }
    
    return nil
}
```

#### ✅ Faz 2.3: Client Integration
**Süre:** 1 gün  
**Öncelik:** YÜKSEK

**Adımlar:**
- [ ] Client başlangıcında plugin kontrolü
- [ ] Eksik plugin uyarısı
- [ ] Interactive plugin seçimi
- [ ] Otomatik plugin indirme seçeneği

**Kod Değişiklikleri:**
```go
// client-tcp.go - RunInteractiveSetup
func (s *TCPClientSetup) RunInteractiveSetup() (*TCPProxyClient, error) {
    // ... server config
    
    // Plugin seçimi
    plugins := s.listPluginsFromServer() // Server'dan plugin listesi
    
    if len(plugins) == 0 {
        // Local plugin'leri kontrol et
        plugins = s.listLocalPlugins()
    }
    
    // Plugin seçimi UI
    selectedPlugin := s.selectPlugin(plugins)
    
    // Plugin local'de var mı?
    if !s.pluginExists(selectedPlugin) {
        // Server'dan indir
        if s.askYesNo("Plugin local'de yok. Server'dan indirilsin mi? (y/N): ") {
            downloader := NewPluginDownloader(config.ServerURL, "plugins")
            if err := downloader.DownloadPlugin(selectedPlugin); err != nil {
                return nil, fmt.Errorf("plugin indirme hatası: %v", err)
            }
        }
    }
    
    // Plugin'i yükle
    // ...
}
```

---

### Faz 3: TCP Optimizasyonları (Tamamlama)

#### ✅ Faz 3.1: Fast-Path Tam Optimizasyonu
**Süre:** 3 gün  
**Öncelik:** YÜKSEK

**Adımlar:**
- [ ] WebSocket path'de passthrough desteği
- [ ] Plugin kancalarını tamamen bypass (passthrough modunda)
- [ ] Tek plugin modunda protokol tespiti kaldır
- [ ] İlk paket okuma optimizasyonu

**Kod Değişiklikleri:**
```go
// engine/handlers.go - handleWebSocketProxy
func (e *ProxyEngine) handleWebSocketProxy(wsConn *websocket.Conn, plugin GamePlugin, session *GameSession) error {
    // Passthrough check
    passthrough := false
    if cfg, ok := e.config.PluginConfigs[plugin.Name()]; ok {
        passthrough = !cfg.EnableObfuscation
    }
    
    if passthrough {
        // Zero-copy passthrough
        errChan := make(chan error, 2)
        
        go func() {
            buf := e.bufferPool.Get().([]byte)
            defer e.bufferPool.Put(buf)
            // io.CopyBuffer ile zero-copy
            _, err := io.CopyBuffer(serverConn, wsConnWrapper, buf)
            errChan <- err
        }()
        
        go func() {
            buf := e.bufferPool.Get().([]byte)
            defer e.bufferPool.Put(buf)
            _, err := io.CopyBuffer(wsConnWrapper, serverConn, buf)
            errChan <- err
        }()
        
        return <-errChan
    }
    
    // Normal processing mode (existing code)
}
```

#### ✅ Faz 3.2: Control Plane Separation
**Süre:** 2 gün  
**Öncelik:** YÜKSEK

**Adımlar:**
- [ ] Tüm API endpoint'lerini 8081'e taşı
- [ ] 8080 sadece WebSocket için kullan
- [ ] Client config güncellemeleri
- [ ] Documentation update

**Kod Değişiklikleri:**
```go
// engine/engine.go - setupHTTPServer
func (e *ProxyEngine) setupHTTPServer() error {
    // Data-plane: SADECE WebSocket
    dataMux := http.NewServeMux()
    dataMux.HandleFunc(e.config.WSEndpoint, e.handleWebSocket)
    // /api/info KALDIRILACAK
    
    e.httpServer = &http.Server{
        Addr:    fmt.Sprintf("%s:%d", e.config.ListenAddr, e.config.HTTPPort),
        Handler: dataMux,
    }
    
    // Control-plane: TÜM API'ler
    controlMux := http.NewServeMux()
    controlMux.HandleFunc("/api/info", e.handleInfo) // 8081'e taşındı
    controlMux.HandleFunc("/api/health", e.handleHealth)
    controlMux.HandleFunc("/api/stats", e.handleStats)
    controlMux.HandleFunc("/api/plugins", e.handlePlugins)
    controlMux.HandleFunc("/api/plugin/", e.handlePluginAPI)
    // ... diğer endpoint'ler
    
    e.controlServer = &http.Server{
        Addr:    fmt.Sprintf("%s:%d", e.config.ListenAddr, 8081),
        Handler: controlMux,
    }
    
    go func() {
        _ = e.controlServer.ListenAndServe()
    }()
    
    return nil
}
```

#### ✅ Faz 3.3: WebSocket Compression
**Süre:** 1 gün  
**Öncelik:** ORTA

**Adımlar:**
- [ ] Varsayılan compression'ı kapat
- [ ] Config ile kontrol
- [ ] Production profilleri

**Kod Değişiklikleri:**
```go
// engine/engine.go - NewProxyEngine
wsUpgrader: websocket.Upgrader{
    EnableCompression: false, // Varsayılan: kapalı
    // ...
}
```

---

### Faz 4: UDP Desteği (Gelecek)

#### ⏳ Faz 4.1: UDP over WebSocket (Basit)
**Süre:** 5 gün  
**Öncelik:** DÜŞÜK (şimdilik)

**Adımlar:**
- [ ] UDP packet encapsulation formatı
- [ ] Client tarafı UDP → WebSocket dönüşümü
- [ ] Server tarafı WebSocket → UDP dönüşümü
- [ ] Packet routing (client IP/port tracking)

**Not:** Bu faz şimdilik bekletilebilir, TCP optimize edildikten sonra.

---

## 📋 DETAYLI TASK CHECKLIST

### Phase 1: Plugin System (8 gün)

#### Week 1
- [ ] **Day 1:** Plugin dizin yapısı (1 gün)
  - [ ] `plugins/` klasörü otomatik oluşturma
  - [ ] Plugin dizin doğrulama
  - [ ] Hata yönetimi
  - [ ] Test: Boş dizin, mevcut dizin, hatalı yapı

- [ ] **Day 2-3:** Yeni Plugin.json formatı (2 gün)
  - [ ] JSON schema tanımı
  - [ ] JSON validation
  - [ ] Backward compatibility
  - [ ] Config schema parsing
  - [ ] Test: Eski format, yeni format, geçersiz format

- [ ] **Day 4-5:** Built-in Plugin Handler (2 gün)
  - [ ] Handler interface
  - [ ] Handler registry
  - [ ] Minecraft handler
  - [ ] Basic-TCP handler
  - [ ] Test: Handler creation, config validation

- [ ] **Day 6:** Config yönetimi (1 gün)
  - [ ] Default config oluşturma
  - [ ] Config.json yükleme/kaydetme
  - [ ] Config validation
  - [ ] Config merge
  - [ ] Test: Default config, file config, merge

#### Week 2
- [ ] **Day 7-8:** Integration & Testing (2 gün)
  - [ ] Plugin loader entegrasyonu
  - [ ] End-to-end testler
  - [ ] Documentation
  - [ ] Migration guide (eski → yeni format)

### Phase 2: Remote Plugin Download (5 gün)

#### Week 2 (devam)
- [ ] **Day 9-10:** Server API (2 gün)
  - [ ] `/api/plugins` endpoint
  - [ ] `/api/plugin/info/{name}` endpoint
  - [ ] `/api/plugin/config/{name}` endpoint
  - [ ] `/api/plugin/download/{name}` endpoint
  - [ ] Error handling
  - [ ] Test: API endpoint'leri, error cases

- [ ] **Day 11-12:** Client Downloader (2 gün)
  - [ ] Plugin listesi alma
  - [ ] Plugin indirme
  - [ ] Plugin kaydetme
  - [ ] Retry mekanizması
  - [ ] Test: İndirme, hata durumları, retry

- [ ] **Day 13:** Client Integration (1 gün)
  - [ ] Interactive plugin seçimi
  - [ ] Otomatik indirme
  - [ ] UI iyileştirmeleri
  - [ ] Test: End-to-end plugin download

### Phase 3: TCP Optimizations (6 gün)

#### Week 3
- [ ] **Day 14-16:** Fast-Path (3 gün)
  - [ ] WebSocket passthrough
  - [ ] Plugin kancaları bypass
  - [ ] Protokol tespiti kaldırma
  - [ ] Performance testler
  - [ ] Test: Ping, throughput, CPU usage

- [ ] **Day 17-18:** Control Plane (2 gün)
  - [ ] API endpoint taşıma
  - [ ] Client config güncellemeleri
  - [ ] Documentation
  - [ ] Test: Port separation, API access

- [ ] **Day 19:** Compression (1 gün)
  - [ ] Varsayılan kapalı
  - [ ] Config kontrolü
  - [ ] Test: Compression on/off

---

## 🔧 TEKNİK DETAYLAR

### 6. Plugin.json Schema Detayları

#### 6.1. Config Field Types

**Desteklenen Tipler:**
- `string`: Text input
- `integer`: Number input (min/max validation)
- `boolean`: Checkbox
- `duration`: Time duration (e.g., "5s", "1m")
- `enum`: Dropdown (options array)

**Örnek:**
```json
{
  "buffer_size": {
    "type": "integer",
    "default": 32768,
    "min": 1024,
    "max": 65535,
    "description": "Buffer size in bytes"
  },
  "timeout": {
    "type": "duration",
    "default": "5s",
    "description": "Connection timeout"
  },
  "log_level": {
    "type": "enum",
    "default": "info",
    "options": ["debug", "info", "warn", "error"],
    "description": "Logging level"
  }
}
```

#### 6.2. Runtime Types

**1. Built-in:**
```json
{
  "runtime": {
    "type": "builtin",
    "builtin_handler": "minecraft"
  }
}
```

**2. Shared Library (.so):**
```json
{
  "runtime": {
    "type": "so",
    "entry_point": "NewPlugin"
  }
}
```

**3. JSON-only (Basit Proxy):**
```json
{
  "runtime": {
    "type": "json",
    "proxy_type": "tcp",
    "target_from_config": "server_addr"
  }
}
```

#### 6.3. Hooks Configuration

**Hooks, plugin'in hangi işlemleri yapacağını belirtir:**

```json
{
  "hooks": {
    "on_session_start": false,    // Session başlangıcında özel işlem yok
    "on_session_end": false,      // Session bitiminde özel işlem yok
    "process_client_data": false, // Client paketlerini işleme (passthrough)
    "process_server_data": false  // Server paketlerini işleme (passthrough)
  }
}
```

**Eğer tüm hooks false ise → Passthrough mode (fast-path)**

---

### 7. Remote Plugin Download Detayları

#### 7.1. Plugin Bundle Format

**Option 1: JSON Bundle (Önerilen)**
```json
{
  "plugin": {
    // plugin.json içeriği
  },
  "config": {
    // config.json içeriği (opsiyonel)
  }
}
```

**Option 2: ZIP Archive**
```
plugin.zip
├── plugin.json
└── config.json (opsiyonel)
```

**Option 3: Separate Requests**
```
GET /api/plugin/info/minecraft  → plugin.json
GET /api/plugin/config/minecraft → config.json (opsiyonel)
```

#### 7.2. Security Considerations

**1. Checksum Verification:**
```json
{
  "remote": {
    "checksum": "sha256:abc123...",
    "signature": "base64:xyz789..."
  }
}
```

**2. Version Control:**
```json
{
  "version": "2.0.0",
  "min_engine_version": "2.0.0",
  "max_engine_version": "2.1.0"
}
```

**3. Rate Limiting:**
- Plugin download: 10 request/minute/IP
- Plugin info: 60 request/minute/IP

---

## 🚨 RİSK ANALİZİ VE ÇÖZÜMLER

### Risk 1: Backward Compatibility
**Risk:** Eski plugin format'ı kullanan kullanıcılar etkilenebilir  
**Çözüm:** 
- Eski format desteği (auto-conversion)
- Migration script
- Detaylı hata mesajları

### Risk 2: Remote Plugin Security
**Risk:** Kötü niyetli plugin'ler indirilebilir  
**Çözüm:**
- Checksum verification
- Digital signature (gelecek)
- Sandbox execution (gelecek)
- User confirmation

### Risk 3: Performance Regression
**Risk:** Yeni sistem performansı düşürebilir  
**Çözüm:**
- Benchmark testler
- Performance monitoring
- Rollback planı
- Gradual rollout

### Risk 4: Plugin Dependency Hell
**Risk:** Plugin'ler birbirine bağımlı olabilir  
**Çözüm:**
- Dependency graph validation
- Circular dependency detection
- Version conflict resolution

---

## 📊 BAŞARI METRİKLERİ

### Plugin Sistemi Metrikleri

| Metrik | Hedef | Ölçüm |
|--------|-------|-------|
| Plugin yükleme süresi | <100ms | TBD |
| Remote download süresi | <2s | TBD |
| Plugin validation | %100 | TBD |
| Backward compatibility | %100 | TBD |

### Performans Metrikleri

| Metrik | Mevcut | Hedef | Durum |
|--------|--------|-------|-------|
| Tek Oyuncu Ping | 40-51ms | 30-35ms | ⏳ |
| Multi-User Ping | >90ms | <90ms | ⏳ |
| Throughput | 1000 B/s | 8000+ B/s | ⏳ |
| Memory | Yüksek | %60-70 az | ⏳ |
| CPU | Yüksek | %40-50 az | ⏳ |

---

## 📅 ZAMAN ÇİZELGESİ

### Milestone 1: Plugin System (2 Hafta)
- ✅ Plugin dizin yapısı
- ✅ Yeni JSON formatı
- ✅ Built-in handlers
- ✅ Config yönetimi

### Milestone 2: Remote Download (1 Hafta)
- ✅ Server API
- ✅ Client downloader
- ✅ Integration

### Milestone 3: TCP Optimizations (1.5 Hafta)
- ✅ Fast-path
- ✅ Control plane separation
- ✅ Compression

### Milestone 4: Testing & Documentation (1 Hafta)
- ✅ End-to-end testler
- ✅ Performance testler
- ✅ Documentation
- ✅ Migration guide

**Toplam Süre:** ~5.5 hafta

---

## 🎓 ÖĞRENİLEN DERSLER

### Mimari Dersler

1. **JSON-First Approach:** Plugin'ler JSON ile tanımlanmalı, kod ikincil
2. **Separation of Concerns:** Config, metadata, runtime ayrılmalı
3. **Backward Compatibility:** Eski sistemler için migration path gerekli
4. **Remote-First Design:** Server'dan plugin indirme baştan düşünülmeli

### Performans Dersler

1. **Fast-Path Critical:** Hot-path'te minimal işlem
2. **Plugin Hooks:** Sadece gerektiğinde çağrılmalı
3. **Config Defaults:** Sensible defaults performans için önemli
4. **Batch Operations:** İstatistikler batch update olmalı

### Güvenlik Dersler

1. **Plugin Validation:** İndirilen plugin'ler validate edilmeli
2. **Checksum Verification:** Dosya bütünlüğü kontrol edilmeli
3. **Sandbox Execution:** Plugin'ler izole çalışmalı (gelecek)
4. **User Confirmation:** Remote plugin indirme onay gerektirmeli

---

## 🔮 GELECEK VİZYON

### Kısa Vadeli (3 Ay)
- ✅ JSON tabanlı plugin sistemi
- ✅ Remote plugin download
- ✅ TCP optimizasyonları tamamlanması
- ✅ Documentation

### Orta Vadeli (6 Ay)
- UDP desteği (WebSocket over UDP)
- Plugin marketplace
- Digital signature verification
- Plugin sandbox execution

### Uzun Vadeli (1 Yıl)
- Rust migration (core engine)
- Multi-region deployment
- Plugin analytics
- Enterprise features

---

## 📞 SONUÇ

Bu rapor, Universal Game Proxy Engine'in yeniden yapılandırılması için kapsamlı bir plan sunmaktadır. Öncelikli hedefler:

1. **JSON-First Plugin System:** Plugin'ler JSON ile tanımlanacak
2. **Remote Plugin Download:** Server'dan client'a plugin aktarımı
3. **TCP Optimizasyonları:** Fast-path ve control plane separation
4. **Backward Compatibility:** Eski sistem desteği

Detaylı yol haritası ve task checklist ile adım adım ilerlenebilir. Her faz için test kriterleri ve başarı metrikleri tanımlanmıştır.

---

## 📚 KOD REFERANSLARI VE DOSYA HARİTASI

### Mevcut Projedeki Önemli Dosyalar

Bu bölüm, yeni projeye başlarken eski projeden hangi dosyalara referans alınacağını ve hangi kısımların kullanılabileceğini belirtir.

#### 🔧 Engine Core (Temel Motor)

| Dosya | Amaç | Referans Alınacak Kısımlar | Durum |
|-------|------|---------------------------|-------|
| `engine/engine.go` | Ana proxy motoru | - Buffer pool implementasyonu (32KB)<br>- HTTP server setup<br>- Control plane separation kısmı<br>- Context yönetimi | ⚠️ Referans al, ama yeniden yaz |
| `engine/session.go` | Session yönetimi | - `createSession()` fonksiyonu<br>- `cleanupSession()` fonksiyonu<br>- TCP_NODELAY ayarları<br>- Fast-path logic (tek plugin bypass) | ✅ Kullanılabilir, optimize et |
| `engine/handlers.go` | HTTP/WebSocket handlers | - `handleWebSocket()` - WebSocket upgrade<br>- `handleWebSocketProxy()` - WebSocket proxy logic<br>- `handleUDPTunnel()` - UDP tunnel (gelecek için)<br>- API endpoint handlers | ⚠️ Referans al, passthrough ekle |
| `engine/plugin.go` | Plugin interface | - `GamePlugin` interface tanımı<br>- `GameSession` struct<br>- `PluginConfig` struct<br>- Built-in plugin implementasyonları | ✅ Interface'i koru, implementasyonları yeniden yaz |

#### 🧩 Plugin System (Plugin Sistemi)

| Dosya | Amaç | Referans Alınacak Kısımlar | Durum |
|-------|------|---------------------------|-------|
| `engine/plugin_loader.go` | Plugin yükleme | - Plugin dizin tarama mantığı<br>- JSON plugin yükleme (metadata için)<br>- Plugin registry sistemi<br>- `ensurePluginDir()` benzeri logic | ⚠️ Mantığı referans al, JSON schema'yı yeniden yaz |
| `engine/plugin_registry.go` | Plugin kayıt sistemi | - Global registry pattern<br>- Plugin lookup logic | ✅ Kullanılabilir |
| `engine/builtin_plugin.go` | Built-in plugin'ler | - `BuiltinMinecraftPlugin` struct<br>- `BuiltinBasicTCPPlugin` struct<br>- `CreatePlugin()` factory fonksiyonu | ✅ Referans al, handler pattern'e çevir |
| `engine/remote-loader.go` | Remote plugin indirme | - `RemotePluginLoader` struct<br>- Plugin manifest yapısı<br>- Checksum verification logic<br>- Security checks | ✅ Çok iyi referans, adapte et |
| `engine/plugin_installer.go` | Plugin kurulum | - Interactive setup logic<br>- Config oluşturma | ⚠️ UI mantığı referans al, JSON schema entegrasyonu ekle |

#### 📡 Client & Server

| Dosya | Amaç | Referans Alınacak Kısımlar | Durum |
|-------|------|---------------------------|-------|
| `proxy-tcp.go` | TCP proxy server | - Interactive setup mantığı<br>- Config yapısı<br>- Server başlatma logic | ⚠️ Setup mantığını referans al, JSON config entegrasyonu ekle |
| `client-tcp.go` | TCP proxy client | - Interactive setup<br>- WebSocket bağlantı mantığı<br>- Plugin seçimi UI | ⚠️ UI mantığını referans al, remote download ekle |
| `cmd/v2ray-direct/main.go` | V2Ray-direct server | - XOR+Base64 obfuscation<br>- Session yönetimi pattern<br>- WebSocket handling | ⚠️ Obfuscation logic referans al (basit ama çalışıyor) |

#### 📁 Plugin Definitions

| Dosya | Amaç | Referans Alınacak Kısımlar | Durum |
|-------|------|---------------------------|-------|
| `plugins/minecraft/plugin.json` | Minecraft plugin metadata | - Metadata yapısı (eski format)<br>- Config örnekleri | ⚠️ Yeni JSON schema'ya göre dönüştür |
| `plugins/half-life/plugin.json` | Half-Life plugin | - Plugin örnekleri | ⚠️ Yeni formata çevir |

#### ⚙️ Configuration Files

| Dosya | Amaç | Referans Alınacak Kısımlar | Durum |
|-------|------|---------------------------|-------|
| `tcp-engine-config.json` | Engine config | - Config yapısı<br>- Port ayarları<br>- Plugin config örnekleri | ⚠️ Yapıyı referans al, JSON schema entegrasyonu ekle |
| `quic-udp-engine-config.json` | QUIC engine config | - QUIC ayarları (gelecek için) | 📝 Gelecek için referans |

### Önemli Fonksiyon Referansları

#### Buffer Pool Implementation
```go
// REFERANS: engine/engine.go - NewProxyEngine (satır ~145-155)
// 32KB buffer pool - Bu implementasyon kullanılabilir
e.bufferPool = sync.Pool{
    New: func() any {
        return make([]byte, 32768)
    },
}
```

#### TCP Low-Latency Settings
```go
// REFERANS: engine/session.go - handleTCPConnection (satır ~198-202)
// TCP_NODELAY ve Keep-Alive ayarları - Mükemmel referans
if tcpCli, ok := clientConn.(*net.TCPConn); ok {
    _ = tcpCli.SetNoDelay(true)
    _ = tcpCli.SetKeepAlive(true)
    _ = tcpCli.SetKeepAlivePeriod(30 * time.Second)
}
```

#### Fast-Path Logic (Tek Plugin Bypass)
```go
// REFERANS: engine/session.go - handleTCPConnection (satır ~212-240)
// Tek plugin modunda protokol tespiti bypass - Mantığı koru
e.pluginMutex.RLock()
if len(e.activePlugins) == 1 {
    for name := range e.activePlugins {
        pluginName = name
        break
    }
}
e.pluginMutex.RUnlock()
```

#### Passthrough Mode
```go
// REFERANS: engine/session.go - handleTCPProxy (satır ~332-368)
// Passthrough modunda io.CopyBuffer kullanımı - WebSocket'e de uygula
if passthrough {
    errChan := make(chan error, 2)
    go func() {
        buf := e.bufferPool.Get().([]byte)
        defer e.bufferPool.Put(buf)
        _, err := io.CopyBuffer(serverConn, clientConn, buf)
        errChan <- err
    }()
    // ...
}
```

#### Session Management
```go
// REFERANS: engine/session.go - createSession (satır ~79-107)
// Session oluşturma ve kaydetme - Mantığı koru
session := &GameSession{
    ID:         generateSessionID(),
    CreatedAt:  time.Now(),
    Active:     true,
    PluginName: pluginName,
    // ...
}
e.sessionMutex.Lock()
e.sessions[session.ID] = session
e.sessionMutex.Unlock()
```

#### WebSocket Proxy Logic
```go
// REFERANS: engine/handlers.go - handleWebSocketProxy (satır ~188-296)
// WebSocket bidirectional proxy - Passthrough desteği ekle
// WebSocket -> Server goroutine
go func() {
    for {
        _, data, err := wsConn.ReadMessage()
        // Process and send
    }
}()
// Server -> WebSocket goroutine
go func() {
    for {
        n, err := serverConn.Read(buffer)
        // Process and send
    }
}()
```

#### Remote Plugin Loader
```go
// REFERANS: engine/remote-loader.go - DownloadPlugin (satır ~93-161)
// Security checks ve checksum verification - Çok iyi referans
// Hash verification, file validation, cache management
```

#### V2Ray-Direct Obfuscation
```go
// REFERANS: cmd/v2ray-direct/main.go - encodeObfuscated (satır ~489-503)
// Basit XOR+Base64 obfuscation - Basit ama çalışıyor
func encodeObfuscated(data []byte, key []byte) []byte {
    encrypted := make([]byte, len(data))
    for i, b := range data {
        encrypted[i] = b ^ key[i%len(key)]
    }
    return []byte(base64.StdEncoding.EncodeToString(encrypted))
}
```

### Kullanılmayacak / Yeniden Yazılacak Kısımlar

| Dosya/Kısım | Neden | Alternatif |
|-------------|-------|------------|
| `engine/plugin_loader.go` - JSON plugin loading | Sadece metadata yüklüyor, çalışan plugin oluşturmuyor | Yeni JSON schema ile yeniden yaz |
| `engine/plugin.go` - Built-in plugin'ler | Hard-coded, handler pattern yok | Built-in handler pattern'e çevir |
| `engine/handlers.go` - Passthrough | WebSocket'te passthrough yok | WebSocket passthrough ekle |
| Config yapıları | JSON schema entegrasyonu yok | Schema-based config sistemi |

### Dosya Yolu Notasyonu

Raporda kullanılan dosya referansları **eski proje** klasöründen alınmıştır:

```
proje/                    # Eski proje klasörü (referans için)
├── engine/
│   ├── engine.go        # Referans alınacak
│   ├── session.go       # Referans alınacak
│   └── handlers.go      # Referans alınacak
├── client-tcp.go        # Referans alınacak
└── proxy-tcp.go         # Referans alınacak
```

**Yeni projede** dosya yapısı şöyle olabilir:

```
yeni-proje/              # Yeni proje klasörü
├── internal/
│   ├── engine/         # engine/ klasörü buraya taşınacak
│   ├── plugin/         # Plugin sistemi
│   └── client/         # Client kodları
├── cmd/
│   ├── server/         # Server main
│   └── client/         # Client main
└── plugins/            # Plugin'ler
```

### Kod Referans Stratejisi

1. **Kopyala-Yapıştır Yapma:** Kodları direkt kopyalamayın, mantığı anlayıp yeniden yazın
2. **Pattern'leri Koru:** İyi çalışan pattern'leri (session management, buffer pooling) koruyun
3. **Yeni Yapıya Uyarla:** Eski kodları yeni mimariye (JSON schema, handler pattern) uyarlayın
4. **Test Et:** Referans aldığınız her kısım için test yazın

---

**Rapor Hazırlayan:** AI Assistant  
**Tarih:** 2025  
**Versiyon:** 2.0  
**Durum:** Kapsamlı Strateji ve Yol Haritası

