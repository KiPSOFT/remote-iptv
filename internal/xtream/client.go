package xtream

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	BaseURL  string
	Username string
	Password string
	client   *http.Client
}

// Channel represents a channel in the Xtream API
type Channel struct {
	ID         int    `json:"stream_id"`
	Name       string `json:"name"`
	StreamType string `json:"stream_type"`
	StreamURL  string `json:"stream_url"`
	StreamIcon string `json:"stream_icon"`
	CategoryID string `json:"category_id"`
	Rating     string `json:"rating,omitempty"`
	URL        string // URL for streaming the channel
	Extension  string `json:"container_extension,omitempty"`
}

type Category struct {
	ID    json.Number `json:"category_id"`
	Name  string      `json:"category_name"`
	Type  string      `json:"category_type"`
}

// Helper function to get category ID as int
func (c *Category) GetID() (int, error) {
	// Boş string kontrolü
	if c.ID.String() == "" {
		return 0, nil // Boş string durumunda varsayılan olarak 0 döndür
	}
	
	id, err := c.ID.Int64()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func NewClient(baseURL, username, password string) *Client {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     120 * time.Second,
			DisableCompression:  true,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 2,
		},
	}

	return &Client{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		client:   client,
	}
}

// makeRequest performs an HTTP request with Tivimate User-Agent
func (c *Client) makeRequest(url string) (*http.Response, error) {
	time.Sleep(1 * time.Second)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Tivimate/4.8.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "keep-alive")
	return c.client.Do(req)
}

func (c *Client) GetLiveStreams() ([]Channel, error) {
	endpoint := fmt.Sprintf("%s/player_api.php", c.BaseURL)
	params := url.Values{}
	params.Add("username", c.Username)
	params.Add("password", c.Password)
	params.Add("action", "get_live_streams")

	fullURL := endpoint + "?" + params.Encode()
	log.Printf("Fetching live streams from: %s", fullURL)

	var resp *http.Response
	var err error
	for i := 0; i < 3; i++ {
		resp, err = c.makeRequest(fullURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if err != nil {
			log.Printf("Attempt %d: Error fetching live streams: %v", i+1, err)
		} else {
			log.Printf("Attempt %d: Got status code %d", i+1, resp.StatusCode)
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("failed after 3 attempts: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("server returned status code %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	var channels []Channel
	if err := json.Unmarshal(body, &channels); err != nil {
		log.Printf("Error unmarshaling channels: %v", err)
		return nil, err
	}

	// Set stream type and URL for all live channels
	for i := range channels {
		channels[i].StreamType = "live"
		//channels[i].URL = c.GetStreamURL(channels[i].ID)
	}

	log.Printf("Successfully fetched %d live streams", len(channels))
	return channels, nil
}

func (c *Client) GetCategories() ([]Category, error) {
	endpoint := fmt.Sprintf("%s/player_api.php", c.BaseURL)
	params := url.Values{}
	params.Add("username", c.Username)
	params.Add("password", c.Password)
	params.Add("action", "get_live_categories")

	resp, err := http.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var categories []Category
	if err := json.Unmarshal(body, &categories); err != nil {
		return nil, err
	}

	return categories, nil
}

func (c *Client) GetStreamURL(ch Channel) string {
	if ch.StreamType == "live" {
		return fmt.Sprintf("%s/%s/%s/%s/%d.m3u8", c.BaseURL, ch.StreamType, c.Username, c.Password, ch.ID)
	} else {
		return fmt.Sprintf("%s/%s/%s/%s/%d.%s", c.BaseURL, ch.StreamType, c.Username, c.Password, ch.ID, ch.Extension)
	}
}

func (c *Client) GetLiveCategories() ([]Category, error) {
	endpoint := fmt.Sprintf("%s/player_api.php", c.BaseURL)
	params := url.Values{}
	params.Add("username", c.Username)
	params.Add("password", c.Password)
	params.Add("action", "get_live_categories")

	fullURL := endpoint + "?" + params.Encode()
	log.Printf("Fetching live categories from: %s", fullURL)

	resp, err := c.makeRequest(fullURL)
	if err != nil {
		log.Printf("Error fetching live categories: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	// log.Printf("Live categories response: %s", string(body))

	var categories []Category
	if err := json.Unmarshal(body, &categories); err != nil {
		log.Printf("Error unmarshaling categories: %v", err)
		return nil, err
	}

	log.Printf("Successfully fetched %d live categories", len(categories))
	return categories, nil
}

func (c *Client) GetMovieCategories() ([]Category, error) {
	endpoint := fmt.Sprintf("%s/player_api.php", c.BaseURL)
	params := url.Values{}
	params.Add("username", c.Username)
	params.Add("password", c.Password)
	params.Add("action", "get_vod_categories")

	fullURL := endpoint + "?" + params.Encode()
	log.Printf("Fetching movie categories from: %s", fullURL)

	resp, err := c.makeRequest(fullURL)
	if err != nil {
		log.Printf("Error fetching movie categories: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	// log.Printf("Movie categories response: %s", string(body))

	var categories []Category
	if err := json.Unmarshal(body, &categories); err != nil {
		log.Printf("Error unmarshaling categories: %v", err)
		return nil, err
	}

	log.Printf("Successfully fetched %d movie categories", len(categories))
	return categories, nil
}

func (c *Client) GetSeriesCategories() ([]Category, error) {
	endpoint := fmt.Sprintf("%s/player_api.php", c.BaseURL)
	params := url.Values{}
	params.Add("username", c.Username)
	params.Add("password", c.Password)
	params.Add("action", "get_series_categories")

	fullURL := endpoint + "?" + params.Encode()
	log.Printf("Fetching series categories from: %s", fullURL)

	resp, err := c.makeRequest(fullURL)
	if err != nil {
		log.Printf("Error fetching series categories: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	// log.Printf("Series categories response: %s", string(body))

	var categories []Category
	if err := json.Unmarshal(body, &categories); err != nil {
		log.Printf("Error unmarshaling categories: %v", err)
		return nil, err
	}

	log.Printf("Successfully fetched %d series categories", len(categories))
	return categories, nil
}

// GetMovieStreams film akışlarını getirir
func (c *Client) GetMovieStreams() ([]Channel, error) {
	endpoint := fmt.Sprintf("%s/player_api.php", c.BaseURL)
	params := url.Values{}
	params.Add("username", c.Username)
	params.Add("password", c.Password)
	params.Add("action", "get_vod_streams")

	fullURL := endpoint + "?" + params.Encode()
	log.Printf("Fetching movie streams from: %s", fullURL)

	var resp *http.Response
	var err error
	for i := 0; i < 3; i++ {
		resp, err = c.makeRequest(fullURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if err != nil {
			log.Printf("Attempt %d: Error fetching movie streams: %v", i+1, err)
		} else {
			log.Printf("Attempt %d: Got status code %d", i+1, resp.StatusCode)
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("failed after 3 attempts: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("server returned status code %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading movie streams response: %v", err)
		return nil, err
	}

	var channels []Channel
	if err := json.Unmarshal(body, &channels); err != nil {
		log.Printf("Error unmarshaling movie streams: %v", err)
		return nil, err
	}

	// Set stream type for all movie channels
	for i := range channels {
		channels[i].StreamType = "movie"
		//channels[i].URL = c.GetStreamURL(channels[i].ID)
	}

	log.Printf("Successfully fetched %d movie streams", len(channels))
	return channels, nil
}

// GetSeriesStreams dizi akışlarını getirir
func (c *Client) GetSeriesStreams() ([]Channel, error) {
	endpoint := fmt.Sprintf("%s/player_api.php", c.BaseURL)
	params := url.Values{}
	params.Add("username", c.Username)
	params.Add("password", c.Password)
	params.Add("action", "get_series")

	fullURL := endpoint + "?" + params.Encode()
	log.Printf("Fetching series streams from: %s", fullURL)

	resp, err := c.makeRequest(fullURL)
	if err != nil {
		log.Printf("Error fetching series streams: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading series streams response: %v", err)
		return nil, err
	}

	var channels []Channel
	if err := json.Unmarshal(body, &channels); err != nil {
		log.Printf("Error unmarshaling series streams: %v", err)
		return nil, err
	}

	// Set stream type and URL for all series channels
	for i := range channels {
		channels[i].StreamType = "series"
		// channels[i].URL = c.GetStreamURL(channels[i].ID)
	}

	log.Printf("Successfully fetched %d series streams", len(channels))
	return channels, nil
}

// GetCategoryID returns the category ID as an integer
func (c *Channel) GetCategoryID() (int, error) {
	if c.CategoryID == "" {
		return 0, nil
	}
	return strconv.Atoi(c.CategoryID)
} 