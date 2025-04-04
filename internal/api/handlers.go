package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"net/url"

	"remote-iptv/internal/db"
	"remote-iptv/internal/player"
	"remote-iptv/internal/xtream"
	"github.com/gorilla/mux"
)

// convertXtreamChannels xtream.Channel türünü db.Channel türüne dönüştürür
func (h *Handler) convertXtreamChannels(channels []xtream.Channel, streamType string) []db.Channel {
	result := make([]db.Channel, 0, len(channels))
	usedIDs := make(map[int]bool)
	autoID := 1000000

	for _, ch := range channels {
		// Kanal ID'sini kontrol et
		channelID := ch.ID
		
		// ID 0 ise veya zaten kullanılmışsa yeni bir ID ata
		if channelID == 0 || usedIDs[channelID] {
			if streamType == "movie" {
				log.Printf("Assigning new ID %d to channel '%s' (original ID: %d)", channelID, ch.Name, ch.ID)
			}
			channelID = autoID
			autoID++
		}
		
		// ID'yi kullanılmış olarak işaretle
		usedIDs[channelID] = true

		// Category ID'yi integer'a çevir
		categoryID := 0
		if ch.CategoryID != "" {
			categoryIDInt, err := strconv.Atoi(ch.CategoryID)
			if err == nil {
				categoryID = categoryIDInt
			}
		}

		// URL'yi seç (URL dolu değilse StreamURL'yi kullan)
		ch.URL = h.xtream.GetStreamURL(ch)
		
		dbChannel := db.Channel{
			ID:         channelID,
			Name:       ch.Name,
			URL:        ch.URL,
			StreamType: ch.StreamType,
			CategoryID: categoryID,
			StreamIcon: ch.StreamIcon,
			Rating:     ch.Rating,
			Extension:  ch.Extension,
		}
		result = append(result, dbChannel)
	}

	return result
}

type Handler struct {
	player         *player.MPVPlayer
	db            *db.Database
	xtream        *xtream.Client
	mu            sync.Mutex
	currentChannel *db.Channel
}

type ChannelRequest struct {
	URL string `json:"url"`
}

func NewHandler(player *player.MPVPlayer, db *db.Database, xtream *xtream.Client) *Handler {
	return &Handler{
		player: player,
		db:     db,
		xtream: xtream,
	}
}

func (h *Handler) GetChannels(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Veritabanından kanalları al
	channels, err := h.db.GetChannels()
	if err != nil {
		log.Printf("Error getting channels from database: %v", err)
		http.Error(w, "Failed to get channels from database", http.StatusInternalServerError)
		return
	}

	// Kanallar boş olsa bile Xtream'den otomatik çekme işlemi yapma
	// Doğrudan mevcut kanalları JSON olarak döndür
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channels)
}

func (h *Handler) GetXtreamSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.db.GetXtreamSettings()
	if err != nil {
		http.Error(w, "Failed to get Xtream settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if settings == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "settings_not_found",
		})
		return
	}

	json.NewEncoder(w).Encode(settings)
}

func (h *Handler) SaveXtreamSettings(w http.ResponseWriter, r *http.Request) {
	var settings db.XtreamSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.db.SaveXtreamSettings(settings); err != nil {
		http.Error(w, "Failed to save Xtream settings", http.StatusInternalServerError)
		return
	}

	// Xtream client'ı güncelle
	h.xtream = xtream.NewClient(settings.URL, settings.Username, settings.Password)

	w.WriteHeader(http.StatusOK)
}

// getRedirectURL HTTP isteği yaparak nihai URL'yi döndürür
func getRedirectURL(initialURL string) (string, error) {
	log.Printf("Getting redirect URL for: %s", initialURL)
	
	// HTTP GET isteği hazırla
	client := &http.Client{
		// Redirect'leri takip etme (manuel takip edeceğiz)
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 10 * time.Second,
	}

	// User-Agent ekleyelim
	req, err := http.NewRequest("GET", initialURL, nil)
	if err != nil {
		return initialURL, fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	req.Header.Set("User-Agent", "Tivimate")

	// İsteği gönder
	resp, err := client.Do(req)
	if err != nil {
		return initialURL, fmt.Errorf("HTTP isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	// Redirect kodu ise (3xx) sonraki URL'yi al
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {
			// Location header'ı göreli URL ise mutlak URL'ye çevir
			if !strings.HasPrefix(location, "http") {
				baseURL, err := url.Parse(initialURL)
				if err != nil {
					return initialURL, fmt.Errorf("base URL parse edilemedi: %w", err)
				}
				
				relativeURL, err := url.Parse(location)
				if err != nil {
					return initialURL, fmt.Errorf("relative URL parse edilemedi: %w", err)
				}
				
				absoluteURL := baseURL.ResolveReference(relativeURL)
				location = absoluteURL.String()
			}
			
			log.Printf("Redirect detected: %s -> %s", initialURL, location)
			return location, nil
		}
	}
	
	// Redirect yoksa veya location header'ı boşsa orijinal URL'yi döndür
	log.Printf("No redirect for URL: %s (Status: %d)", initialURL, resp.StatusCode)
	return initialURL, nil
}

func (h *Handler) PlayChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL        string `json:"url"`
		Name       string `json:"name"`
		ID         int    `json:"id"`
		StreamType string `json:"stream_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// URL kontrolü
	if req.URL == "" {
		log.Printf("Error: Empty URL received in PlayChannel")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}
	
	log.Printf("PlayChannel called with URL: %s, Name: %s, ID: %d, Type: %s", 
		req.URL, req.Name, req.ID, req.StreamType)

	if h.player == nil {
		var err error
		h.player, err = player.NewMPVPlayer()
		if err != nil {
			log.Printf("Error creating MPV player: %v", err)
			http.Error(w, "Failed to create player", http.StatusInternalServerError)
			return
		}
	}

	// Film veya dizi için URL'yi düzenle
	playURL := req.URL
	
	// URL kontrolleri:
	// 1. URL zaten bir protokol içeriyor mu?
	hasProtocol := strings.HasPrefix(playURL, "http://") || 
				   strings.HasPrefix(playURL, "https://") || 
				   strings.HasPrefix(playURL, "rtmp://") ||
				   strings.HasPrefix(playURL, "rtsp://")
				   
	log.Printf("URL has protocol: %v", hasProtocol)
	
	// 2. Xtream client ayarları var mı?
	if !hasProtocol && h.xtream != nil && req.ID > 0 {
		// Eğer URL'de protokol yoksa ve geçerli bir ID varsa, 
		// xtream ile tam URL oluştur
		settings, err := h.db.GetXtreamSettings()
		if err != nil {
			log.Printf("Error getting xtream settings: %v", err)
		}
		
		if err == nil && settings != nil {
			log.Printf("Generating URL from Xtream settings. BaseURL: %s", settings.URL)
			
			// Film ve dizi için özel endpoint kullan
			baseURL := settings.URL
			userName := settings.Username
			password := settings.Password
			
			// URL'nin sonunda / var mı kontrol et ve temizle
			baseURL = strings.TrimSuffix(baseURL, "/")
			
			if req.StreamType == "movie" {
				originalURL := playURL
				// Movie ID kullanarak Xtream formatında URL oluştur
				// VOD içerik için alternatif API endpoint formatları:
				// 1. standart movie endpoint: http://example.com:80/movie/username/password/12345.mp4
				playURL = fmt.Sprintf("%s/movie/%s/%s/%d.mp4", baseURL, userName, password, req.ID)
				log.Printf("Generated standard movie URL: %s (original: %s)", playURL, originalURL)
				
				// Filmler için oluşturulan URL'yi doğrudan oynamak yerine
				// redirect URL'yi tespit et ve o adresi oyna
				redirectURL, err := getRedirectURL(playURL)
				if err != nil {
					log.Printf("Error getting redirect URL: %v", err)
				} else if redirectURL != playURL {
					log.Printf("Found redirect URL: %s", redirectURL)
					playURL = redirectURL
				}
				
				// Film URL'lerini backendde oluşturarak hazırda tut
				alternativeURLs := []string{
					// 2. vod endpoint: http://example.com:80/vod/username/password/12345.mp4
					fmt.Sprintf("%s/vod/%s/%s/%d.mp4", baseURL, userName, password, req.ID),
					
					// 3. XC get.php API: http://example.com:80/get.php?username=user&password=pass&type=movie&id=12345
					fmt.Sprintf("%s/get.php?username=%s&password=%s&type=movie&id=%d", 
						baseURL, userName, password, req.ID),
					
					// 4. Uzantısız formata
					fmt.Sprintf("%s/movie/%s/%s/%d", baseURL, userName, password, req.ID),
					
					// 5. m3u8 formatı
					fmt.Sprintf("%s/movie/%s/%s/%d.m3u8", baseURL, userName, password, req.ID),
				}
				
				// Alternatif URL'leri log dosyasına yaz
				for i, altURL := range alternativeURLs {
					log.Printf("Alternative movie URL #%d: %s", i+1, altURL)
				}
			} else if req.StreamType == "series" {
				originalURL := playURL
				// Series ID kullanarak Xtream formatında URL oluştur
				// Series içerik için API endpoint formatı:
				// http://example.com:80/series/username/password/12345.mp4
				playURL = fmt.Sprintf("%s/series/%s/%s/%d.mp4", baseURL, userName, password, req.ID)
				log.Printf("Generated series URL: %s (original: %s)", playURL, originalURL)
				
				// Diziler için oluşturulan URL'yi doğrudan oynamak yerine
				// redirect URL'yi tespit et ve o adresi oyna
				redirectURL, err := getRedirectURL(playURL)
				if err != nil {
					log.Printf("Error getting redirect URL: %v", err)
				} else if redirectURL != playURL {
					log.Printf("Found redirect URL: %s", redirectURL)
					playURL = redirectURL
				}
				
				// Alternatif series URL'leri
				alternativeURLs := []string{
					// Uzantısız
					fmt.Sprintf("%s/series/%s/%s/%d", baseURL, userName, password, req.ID),
					
					// m3u8 formatı
					fmt.Sprintf("%s/series/%s/%s/%d.m3u8", baseURL, userName, password, req.ID),
				}
				
				// Alternatif URL'leri log dosyasına yaz
				for i, altURL := range alternativeURLs {
					log.Printf("Alternative series URL #%d: %s", i+1, altURL)
				}
			} else if req.StreamType == "live" {
				originalURL := playURL
				// Live stream için URL oluştur
				// format: http://example.com:80/live/username/password/12345.ts
				playURL = fmt.Sprintf("%s/live/%s/%s/%d", baseURL, userName, password, req.ID)
				log.Printf("Generated live URL: %s (original: %s)", playURL, originalURL)
			}
		} else {
			log.Printf("No Xtream settings found or error: %v", err)
		}
	} else if !hasProtocol {
		log.Printf("Cannot generate URL: hasProtocol=%v, xtream=%v, ID=%d", 
			hasProtocol, h.xtream != nil, req.ID)
	}
	
	// Eğer film veya dizi ise ve protokol ile başlıyorsa redirect URL kontrolü yap
	if hasProtocol && (req.StreamType == "movie" || req.StreamType == "series") {
		// Redirect URL'yi kontrol et
		redirectURL, err := getRedirectURL(playURL)
		if err != nil {
			log.Printf("Error getting redirect URL: %v", err)
		} else if redirectURL != playURL {
			log.Printf("Found redirect URL for direct URL: %s", redirectURL)
			playURL = redirectURL
		}
	}
	
	// Doğrudan URL'yi kontrol et, gerekirse düzelt
	if !hasProtocol && !strings.HasPrefix(playURL, "http") {
		// Eğer hala protokol yoksa, varsayılan olarak http:// ekle
		log.Printf("URL still has no protocol, adding http:// prefix")
		playURL = "http://" + playURL
	}
	
	// Debug: URL'yi logla
	log.Printf("Final URL that will be played: %s", playURL)

	if err := h.player.Play(playURL); err != nil {
		log.Printf("Error playing URL: %v, trying different format", err)
		
		// Film ya da dizi için farklı uzantılar ve formatlar deneyelim
		if req.StreamType == "movie" {
			log.Printf("Trying alternative movie URLs")
			
			// Alternatif URL'leri dene
			if h.xtream != nil && req.ID > 0 {
				settings, err := h.db.GetXtreamSettings()
				if err == nil && settings != nil {
					baseURL := settings.URL
					userName := settings.Username
					password := settings.Password
					
					// URL'nin sonunda / var mı kontrol et ve temizle
					baseURL = strings.TrimSuffix(baseURL, "/")
					
					// Önceden hazırladığımız alternatif URL'leri dene
					alternativeURLs := []string{
						// 2. vod endpoint
						fmt.Sprintf("%s/vod/%s/%s/%d.mp4", baseURL, userName, password, req.ID),
						
						// 3. XC get.php API
						fmt.Sprintf("%s/get.php?username=%s&password=%s&type=movie&id=%d", 
							baseURL, userName, password, req.ID),
						
						// 4. Uzantısız format
						fmt.Sprintf("%s/movie/%s/%s/%d", baseURL, userName, password, req.ID),
						
						// 5. m3u8 formatı
						fmt.Sprintf("%s/movie/%s/%s/%d.m3u8", baseURL, userName, password, req.ID),
						
						// 6. mkv formatı
						fmt.Sprintf("%s/movie/%s/%s/%d.mkv", baseURL, userName, password, req.ID),
						
						// 7. ts formatı
						fmt.Sprintf("%s/movie/%s/%s/%d.ts", baseURL, userName, password, req.ID),
					}
					
					// Her bir alternatif URL'yi dene
					for i, altURL := range alternativeURLs {
						log.Printf("Trying alternative movie URL #%d: %s", i+1, altURL)
						
						// Önce redirect URL'yi kontrol et
						redirectURL, err := getRedirectURL(altURL)
						if err != nil {
							log.Printf("Error getting redirect URL for alt #%d: %v", i+1, err)
						} else if redirectURL != altURL {
							log.Printf("Found redirect URL for alt #%d: %s", i+1, redirectURL)
							altURL = redirectURL
						}
						
						if err := h.player.Play(altURL); err != nil {
							log.Printf("Error playing alternative URL #%d: %v", i+1, err)
						} else {
							log.Printf("Successfully playing alternative URL #%d", i+1)
							// Başarılı olduysa döngüden çık
							break
						}
						
						// Son alternatif de denendiyse ve hala başarısızsa hata döndür
						if i == len(alternativeURLs)-1 {
							log.Printf("All alternative URLs failed")
							http.Error(w, "Failed to play movie with any format", http.StatusInternalServerError)
							return
						}
					}
				} else {
					log.Printf("No Xtream settings available for generating alternative URLs")
					http.Error(w, "Failed to play movie", http.StatusInternalServerError)
					return
				}
			} else {
				log.Printf("No Xtream client or invalid ID for generating alternative URLs")
				http.Error(w, "Failed to play movie", http.StatusInternalServerError)
				return
			}
		} else if req.StreamType == "series" {
			// Farklı uzantıları dene
			if h.xtream != nil && req.ID > 0 {
				settings, err := h.db.GetXtreamSettings()
				if err == nil && settings != nil {
					baseURL := settings.URL
					userName := settings.Username
					password := settings.Password
					
					// URL'nin sonunda / var mı kontrol et ve temizle
					baseURL = strings.TrimSuffix(baseURL, "/")
					
					// Alternatif series URL'leri
					alternativeURLs := []string{
						// Uzantısız
						fmt.Sprintf("%s/series/%s/%s/%d", baseURL, userName, password, req.ID),
						
						// m3u8 formatı
						fmt.Sprintf("%s/series/%s/%s/%d.m3u8", baseURL, userName, password, req.ID),
						
						// mkv formatı
						fmt.Sprintf("%s/series/%s/%s/%d.mkv", baseURL, userName, password, req.ID),
						
						// ts formatı
						fmt.Sprintf("%s/series/%s/%s/%d.ts", baseURL, userName, password, req.ID),
					}
					
					// Her bir alternatif URL'yi dene
					for i, altURL := range alternativeURLs {
						log.Printf("Trying alternative series URL #%d: %s", i+1, altURL)
						
						// Önce redirect URL'yi kontrol et
						redirectURL, err := getRedirectURL(altURL)
						if err != nil {
							log.Printf("Error getting redirect URL for alt series #%d: %v", i+1, err)
						} else if redirectURL != altURL {
							log.Printf("Found redirect URL for alt series #%d: %s", i+1, redirectURL)
							altURL = redirectURL
						}
						
						if err := h.player.Play(altURL); err != nil {
							log.Printf("Error playing alternative URL #%d: %v", i+1, err)
						} else {
							log.Printf("Successfully playing alternative URL #%d", i+1)
							// Başarılı olduysa döngüden çık
							break
						}
						
						// Son alternatif de denendiyse ve hala başarısızsa hata döndür
						if i == len(alternativeURLs)-1 {
							log.Printf("All alternative URLs failed")
							http.Error(w, "Failed to play series with any format", http.StatusInternalServerError)
							return
						}
					}
				} else {
					log.Printf("No Xtream settings available for generating alternative series URLs")
					http.Error(w, "Failed to play series", http.StatusInternalServerError)
					return
				}
			} else {
				log.Printf("No Xtream client or invalid ID for generating alternative series URLs")
				http.Error(w, "Failed to play series", http.StatusInternalServerError)
				return
			}
		} else {
			// Film veya dizi değilse, orijinal hata döndür
			http.Error(w, "Failed to play URL", http.StatusInternalServerError)
			return
		}
	}

	// Kanal bilgisini sakla
	h.currentChannel = &db.Channel{
		URL:        req.URL,
		Name:       req.Name,
		ID:         req.ID,
		StreamType: req.StreamType,
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) StopChannel(w http.ResponseWriter, r *http.Request) {
	if h.player == nil || !h.player.IsActive() {
		http.Error(w, "Player is not active", http.StatusNotFound)
		return
	}

	if err := h.player.Stop(); err != nil {
		log.Printf("Error stopping player: %v", err)
		http.Error(w, "Failed to stop player", http.StatusInternalServerError)
		return
	}

	h.currentChannel = nil
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetFavorites(w http.ResponseWriter, r *http.Request) {
	log.Println("Getting favorites...")
	favorites, err := h.db.GetFavorites()
	if err != nil {
		log.Printf("Error getting favorites: %v", err)
		http.Error(w, "Failed to fetch favorites", http.StatusInternalServerError)
		return
	}

	log.Printf("Found %d favorites", len(favorites))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(favorites)
}

func (h *Handler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	var channel db.Channel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		log.Printf("Error decoding favorite request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Adding favorite - Name: %s, URL: %s", channel.Name, channel.URL)

	if err := h.db.AddFavorite(channel.Name, channel.URL); err != nil {
		log.Printf("Error adding favorite: %v", err)
		http.Error(w, "Failed to add favorite", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully added favorite: %s", channel.Name)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetLiveCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.db.GetCategories("live")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(categories)
}

func (h *Handler) GetMovieCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.db.GetCategories("movie")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(categories)
}

func (h *Handler) GetSeriesCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.db.GetCategories("series")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(categories)
}

// convertXtreamCategories converts xtream categories to database categories
func convertXtreamCategories(categories []xtream.Category, streamType string) []db.Category {
	var result []db.Category
	for _, cat := range categories {
		id, err := cat.GetID()
		if err != nil {
			log.Printf("Error converting category ID for %s: %v", cat.Name, err)
			continue
		}
		result = append(result, db.Category{
			ID:   id,
			Name: cat.Name,
			Type: streamType,
		})
	}
	return result
}

func (h *Handler) UpdateChannels(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Println("Starting channel update process...")

	// Xtream ayarlarını kontrol et
	settings, err := h.db.GetXtreamSettings()
	if err != nil {
		log.Printf("Error getting Xtream settings: %v", err)
		http.Error(w, "Failed to get Xtream settings", http.StatusInternalServerError)
		return
	}

	if settings == nil {
		log.Println("No Xtream settings found")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "xtream_settings_required",
		})
		return
	}

	// Xtream client'ı güncelle
	h.xtream = xtream.NewClient(settings.URL, settings.Username, settings.Password)

	// Kategori ve kanal verileri için bir grup oluştur
	var wg sync.WaitGroup
	var categoryErr, channelErr error
	var liveCategories, movieCategories, seriesCategories []xtream.Category
	var liveStreams, movies, series []xtream.Channel

	// Kategorileri paralel olarak çek
	wg.Add(3)
	go func() {
		defer wg.Done()
		log.Println("Fetching live categories...")
		categories, err := h.xtream.GetLiveCategories()
		if err != nil {
			log.Printf("Error fetching live categories: %v", err)
			categoryErr = err
			return
		}
		liveCategories = categories
		log.Printf("Found %d live categories", len(liveCategories))
	}()

	go func() {
		defer wg.Done()
		log.Println("Fetching movie categories...")
		categories, err := h.xtream.GetMovieCategories()
		if err != nil {
			log.Printf("Error fetching movie categories: %v", err)
			categoryErr = err
			return
		}
		movieCategories = categories
		log.Printf("Found %d movie categories", len(movieCategories))
	}()

	go func() {
		defer wg.Done()
		log.Println("Fetching series categories...")
		categories, err := h.xtream.GetSeriesCategories()
		if err != nil {
			log.Printf("Error fetching series categories: %v", err)
			categoryErr = err
			return
		}
		seriesCategories = categories
		log.Printf("Found %d series categories", len(seriesCategories))
	}()

	wg.Wait()

	if categoryErr != nil {
		http.Error(w, "Failed to fetch categories", http.StatusInternalServerError)
		return
	}

	// Kategorileri veritabanına kaydet
	log.Println("Saving categories to database...")
	if err := h.db.SaveCategories(convertXtreamCategories(liveCategories, "live")); err != nil {
		log.Printf("Error saving live categories: %v", err)
		http.Error(w, "Failed to save categories", http.StatusInternalServerError)
		return
	}

	if err := h.db.SaveCategories(convertXtreamCategories(movieCategories, "movie")); err != nil {
		log.Printf("Error saving movie categories: %v", err)
		http.Error(w, "Failed to save categories", http.StatusInternalServerError)
		return
	}

	if err := h.db.SaveCategories(convertXtreamCategories(seriesCategories, "series")); err != nil {
		log.Printf("Error saving series categories: %v", err)
		http.Error(w, "Failed to save categories", http.StatusInternalServerError)
		return
	}
	
	// Kanal verilerini paralel olarak çek
	wg.Add(3)
	go func() {
		defer wg.Done()
		log.Println("Fetching live streams...")
		streams, err := h.xtream.GetLiveStreams()
		if err != nil {
			log.Printf("Error fetching live streams: %v", err)
			channelErr = err
			return
		}
		liveStreams = streams
		log.Printf("Found %d live streams", len(liveStreams))
	}()

	go func() {
		defer wg.Done()
		log.Println("Fetching movies...")
		videoStreams, err := h.xtream.GetMovieStreams()
		if err != nil {
			log.Printf("Error fetching movies: %v", err)
			channelErr = err
			return
		}
		movies = videoStreams
		log.Printf("Found %d movies", len(movies))
	}()

	go func() {
		defer wg.Done()
		log.Println("Fetching series...")
		seriesStreams, err := h.xtream.GetSeriesStreams()
		if err != nil {
			log.Printf("Error fetching series: %v", err)
			channelErr = err
			return
		}
		series = seriesStreams
		log.Printf("Found %d series", len(series))
	}()

	wg.Wait()

	if channelErr != nil {
		http.Error(w, "Failed to fetch channel data", http.StatusInternalServerError)
		return
	}

	// Tüm kanalları veritabanına kaydet
	log.Println("Saving channels to database...")
	allChannels := append(append(
		h.convertXtreamChannels(liveStreams, "live"),
		h.convertXtreamChannels(movies, "movie")...),
		h.convertXtreamChannels(series, "series")...)

	if err := h.db.SaveChannels(allChannels); err != nil {
		log.Printf("Error saving channels: %v", err)
		http.Error(w, "Failed to save channels", http.StatusInternalServerError)
		return
	}

	log.Println("Channel update process completed successfully")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) RemoveFavorite(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// String id'yi integer'a dönüştür
	idInt, err := strconv.Atoi(id)
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	if err := h.db.RemoveFavorite(idInt); err != nil {
		http.Error(w, "Failed to remove favorite", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetChannelsByType(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	streamType := vars["type"]

	channels, err := h.db.GetChannelsByType(streamType)
	if err != nil {
		http.Error(w, "Failed to get channels", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channels)
}

func (h *Handler) GetChannelsByCategory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	streamType := vars["type"]
	categoryID := vars["categoryId"]

	log.Printf("GetChannelsByCategory - Type: %s, CategoryID: %s", streamType, categoryID)

	// String categoryID'yi integer'a dönüştür
	categoryIDInt, err := strconv.Atoi(categoryID)
	if err != nil {
		log.Printf("Error converting categoryID to int: %v", err)
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	// Stream type'ı kontrol et ve düzelt
	var dbStreamType string
	switch streamType {
	case "live":
		dbStreamType = "live"
	case "movie":
		dbStreamType = "movie"
	case "series":
		dbStreamType = "series"
	default:
		log.Printf("Invalid stream type: %s", streamType)
		http.Error(w, "Invalid stream type", http.StatusBadRequest)
		return
	}

	channels, err := h.db.GetChannelsByCategory(dbStreamType, categoryIDInt)
	if err != nil {
		log.Printf("Error getting channels by category: %v", err)
		http.Error(w, "Failed to get channels", http.StatusInternalServerError)
		return
	}

	log.Printf("Found %d channels for type %s and category %d", len(channels), dbStreamType, categoryIDInt)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channels)
}

func EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.Use(EnableCORS)

	router.HandleFunc("/api/channels", h.GetChannels).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/channels/live", h.GetChannelsByType).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/channels/movie", h.GetChannelsByType).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/channels/series", h.GetChannelsByType).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/channels/{type}/{categoryId}", h.GetChannelsByCategory).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/categories/live", h.GetLiveCategories).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/categories/movie", h.GetMovieCategories).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/categories/series", h.GetSeriesCategories).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/player/play", h.PlayChannel).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/player/stop", h.StopChannel).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/player/status", h.GetPlayerStatus).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/favorites", h.GetFavorites).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/favorites", h.AddFavorite).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/favorites/{id}", h.RemoveFavorite).Methods("DELETE", "OPTIONS")
	router.HandleFunc("/api/xtream/settings", h.GetXtreamSettings).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/xtream/settings", h.SaveXtreamSettings).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/xtream/update", h.UpdateChannels).Methods("POST", "OPTIONS")
}

type PlayerStatus struct {
	IsRunning      bool       `json:"isRunning"`
	CurrentChannel *db.Channel `json:"currentChannel"`
}

func (h *Handler) GetPlayerStatus(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// MPV'nin aktif olup olmadığını kontrol et
	isActive := false
	if h.player != nil && h.player.IsActive() {
		// MPV process durumunu kontrol etmek için özel API metodu kullanılmalı
		// İç yapıya erişmek yerine, MPV player'a prosesin durumunu kontrol ettir
		isAlive, err := h.player.IsProcessAlive()
		if err != nil {
			log.Printf("Error checking MPV process: %v", err)
			// Process kontrol edilemiyorsa varsayılan olarak kapalı kabul et
			isActive = false
			h.currentChannel = nil
		} else {
			// Process durumunu logla
			if isAlive {
				log.Printf("MPV process is active")
				isActive = true
			} else {
				log.Printf("MPV process is not alive")
				// Player aktif değilse kanal bilgisini sıfırla
				h.currentChannel = nil
			}
		}
	}

	status := PlayerStatus{
		IsRunning:      isActive,
		CurrentChannel: h.currentChannel,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
} 