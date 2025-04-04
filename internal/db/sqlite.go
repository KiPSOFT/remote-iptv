package db

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

type Database struct {
	db *sql.DB
}

type Channel struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	StreamType string `json:"stream_type"`
	CategoryID int    `json:"category_id"`
	StreamIcon string `json:"stream_icon"`
	Rating     string `json:"rating,omitempty"`
	Extension  string `json:"extension,omitempty"`
}

type XtreamSettings struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Category struct {
	ID   int    `json:"category_id"`
	Name string `json:"category_name"`
	Type string `json:"type"`
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS xtream_settings (
			id INTEGER PRIMARY KEY,
			url TEXT NOT NULL,
			username TEXT NOT NULL,
			password TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS favorites (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			stream_type TEXT,
			category_id INTEGER,
			stream_icon TEXT
		);
		CREATE TABLE IF NOT EXISTS categories (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS channels (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			stream_type TEXT NOT NULL,
			category_id INTEGER NOT NULL,
			stream_icon TEXT,
			rating TEXT,
			last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			extension TEXT
		);
	`)
	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) SaveXtreamSettings(settings XtreamSettings) error {
	// Önce tabloyu temizle (sadece en son ayarları tutmak için)
	if _, err := d.db.Exec("DELETE FROM xtream_settings"); err != nil {
		return err
	}

	query := `INSERT INTO xtream_settings (url, username, password) VALUES (?, ?, ?)`
	_, err := d.db.Exec(query, settings.URL, settings.Username, settings.Password)
	return err
}

func (d *Database) GetXtreamSettings() (*XtreamSettings, error) {
	query := `SELECT url, username, password FROM xtream_settings ORDER BY id DESC LIMIT 1`
	row := d.db.QueryRow(query)

	var settings XtreamSettings
	err := row.Scan(&settings.URL, &settings.Username, &settings.Password)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &settings, nil
}

func (d *Database) AddFavorite(name, url string) error {
	query := `INSERT INTO favorites (name, url) VALUES (?, ?)`
	_, err := d.db.Exec(query, name, url)
	return err
}

func (d *Database) GetFavorites() ([]Channel, error) {
	query := `SELECT id, name, url, stream_type, category_id, stream_icon FROM favorites`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var ch Channel
		var streamType sql.NullString
		var categoryID sql.NullInt64
		var streamIcon sql.NullString
		
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.URL, &streamType, &categoryID, &streamIcon); err != nil {
			return nil, err
		}
		
		ch.StreamType = streamType.String
		ch.CategoryID = int(categoryID.Int64)
		ch.StreamIcon = streamIcon.String
		
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

func (d *Database) RemoveFavorite(id int) error {
	_, err := d.db.Exec("DELETE FROM favorites WHERE id = ?", id)
	return err
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (db *Database) GetCategories(categoryType string) ([]Category, error) {
	rows, err := db.db.Query("SELECT id, name, type FROM categories WHERE type = ?", categoryType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var cat Category
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Type); err != nil {
			return nil, err
		}
		categories = append(categories, cat)
	}
	return categories, nil
}

// SaveChannels kanal listesini veritabanına kaydeder
func (d *Database) SaveChannels(channels []Channel) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Önce mevcut kanalları temizle
	if _, err := tx.Exec("DELETE FROM channels"); err != nil {
		return err
	}

	// Yeni kanalları ekle
	stmt, err := tx.Prepare("INSERT INTO channels (id, name, url, stream_type, category_id, stream_icon, rating, extension) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ch := range channels {
		_, err = stmt.Exec(ch.ID, ch.Name, ch.URL, ch.StreamType, ch.CategoryID, ch.StreamIcon, ch.Rating, ch.Extension)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetChannels veritabanından kanal listesini getirir
func (db *Database) GetChannels() ([]Channel, error) {
	rows, err := db.db.Query("SELECT id, name, url, stream_type, category_id, stream_icon, rating, extension FROM channels")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []Channel
	emptyURLCount := 0
	
	for rows.Next() {
		var ch Channel
		err := rows.Scan(&ch.ID, &ch.Name, &ch.URL, &ch.StreamType, &ch.CategoryID, &ch.StreamIcon, &ch.Rating, &ch.Extension)
		if err != nil {
			return nil, err
		}
		
		if ch.URL == "" {
			emptyURLCount++
		}
		
		channels = append(channels, ch)
	}
	
	if emptyURLCount > 0 {
		log.Printf("WARNING: Found %d channels with empty URL out of %d total channels", emptyURLCount, len(channels))
	}
	
	return channels, nil
}

// GetChannelsByType belirli bir türdeki kanalları getirir
func (db *Database) GetChannelsByType(streamType string) ([]Channel, error) {
	rows, err := db.db.Query("SELECT id, name, url, stream_type, category_id, stream_icon, rating, extension FROM channels WHERE stream_type = ?", streamType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []Channel
	emptyURLCount := 0
	
	for rows.Next() {
		var ch Channel
		err := rows.Scan(&ch.ID, &ch.Name, &ch.URL, &ch.StreamType, &ch.CategoryID, &ch.StreamIcon, &ch.Rating, &ch.Extension)
		if err != nil {
			return nil, err
		}
		
		if ch.URL == "" {
			emptyURLCount++
			log.Printf("WARNING: Empty URL for channel '%s' (ID: %d, Type: %s)", ch.Name, ch.ID, streamType)
		}
		
		channels = append(channels, ch)
	}
	
	if emptyURLCount > 0 {
		log.Printf("WARNING: Found %d %s channels with empty URL out of %d total", emptyURLCount, streamType, len(channels))
	}
	
	return channels, nil
}

func (db *Database) GetChannelsByCategory(streamType string, categoryID int) ([]Channel, error) {
	rows, err := db.db.Query("SELECT id, name, url, stream_type, category_id, stream_icon, rating, extension FROM channels WHERE stream_type = ? AND category_id = ?", streamType, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []Channel
	emptyURLCount := 0
	
	for rows.Next() {
		var ch Channel
		err := rows.Scan(&ch.ID, &ch.Name, &ch.URL, &ch.StreamType, &ch.CategoryID, &ch.StreamIcon, &ch.Rating, &ch.Extension)
		if err != nil {
			return nil, err
		}
		
		if ch.URL == "" {
			emptyURLCount++
			log.Printf("WARNING: Empty URL for channel '%s' (ID: %d, Type: %s, Category: %d)", 
				ch.Name, ch.ID, streamType, categoryID)
		}
		
		channels = append(channels, ch)
	}

	log.Printf("Channels: %v", len(channels))
	
	if emptyURLCount > 0 {
		log.Printf("WARNING: Found %d %s channels in category %d with empty URL out of %d total", 
			emptyURLCount, streamType, categoryID, len(channels))
	}
	
	return channels, nil
}

// SaveCategories kategorileri veritabanına kaydeder
func (db *Database) SaveCategories(categories []Category) error {
	if len(categories) == 0 {
		return nil
	}

	// Sadece aynı tipteki kategorileri temizle
	_, err := db.db.Exec("DELETE FROM categories WHERE type = ?", categories[0].Type)
	if err != nil {
		return err
	}

	// Yeni kategorileri ekle
	tx, err := db.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("INSERT INTO categories (id, name, type) VALUES (?, ?, ?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, cat := range categories {
		_, err = stmt.Exec(cat.ID, cat.Name, cat.Type)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
} 