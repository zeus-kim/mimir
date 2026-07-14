package discovery

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// YouTubeDiscoverer discovers YouTube channels and their RSS feeds
type YouTubeDiscoverer struct {
	httpClient *http.Client
	userAgent  string
}

// YouTubeChannel represents a discovered YouTube channel
type YouTubeChannel struct {
	ChannelID   string `json:"channel_id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	RSSURL      string `json:"rss_url"`
	Category    string `json:"category,omitempty"`
}

// NewYouTubeDiscoverer creates a new YouTube channel discoverer
func NewYouTubeDiscoverer() *YouTubeDiscoverer {
	return &YouTubeDiscoverer{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		userAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
	}
}

func (d *YouTubeDiscoverer) Name() string { return "youtube" }

// Discover finds YouTube channels for the given topic and keywords
func (d *YouTubeDiscoverer) Discover(topic string, keywords []string, limit int) ([]Source, error) {
	var sources []Source
	seen := make(map[string]bool)

	// Search terms
	searchTerms := append([]string{topic}, keywords...)
	if len(searchTerms) > 5 {
		searchTerms = searchTerms[:5]
	}

	for _, term := range searchTerms {
		channels := d.searchChannels(term)
		for _, ch := range channels {
			if !seen[ch.ChannelID] {
				seen[ch.ChannelID] = true
				sources = append(sources, Source{
					URL:         ch.RSSURL,
					Title:       ch.Title,
					Description: fmt.Sprintf("YouTube channel about %s", term),
					Type:        "youtube",
				})
			}
		}

		if len(sources) >= limit {
			break
		}
	}

	if len(sources) > limit {
		sources = sources[:limit]
	}

	return sources, nil
}

// searchChannels searches YouTube for channels matching the keyword
func (d *YouTubeDiscoverer) searchChannels(keyword string) []YouTubeChannel {
	var channels []YouTubeChannel

	// YouTube search with channel filter (sp=EgIQAg%3D%3D is channel filter)
	query := url.QueryEscape(keyword + " channel")
	searchURL := fmt.Sprintf("https://www.youtube.com/results?search_query=%s&sp=EgIQAg%%3D%%3D", query)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return channels
	}
	req.Header.Set("User-Agent", d.userAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return channels
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return channels
	}

	// Extract channel IDs from the response
	// YouTube channel IDs start with "UC" and are 24 characters
	channelIDPattern := regexp.MustCompile(`"channelId":"(UC[a-zA-Z0-9_-]{22})"`)
	matches := channelIDPattern.FindAllStringSubmatch(string(body), -1)

	// Deduplicate
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		channelID := match[1]
		if seen[channelID] {
			continue
		}
		seen[channelID] = true

		rssURL := fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?channel_id=%s", channelID)
		channels = append(channels, YouTubeChannel{
			ChannelID: channelID,
			Title:     fmt.Sprintf("YouTube Channel %s", channelID[:8]),
			RSSURL:    rssURL,
			Category:  keyword,
		})

		if len(channels) >= 10 {
			break
		}
	}

	return channels
}

// GetChannelRSSURL returns the RSS feed URL for a YouTube channel
func GetChannelRSSURL(channelID string) string {
	return fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?channel_id=%s", channelID)
}

// GetPlaylistRSSURL returns the RSS feed URL for a YouTube playlist
func GetPlaylistRSSURL(playlistID string) string {
	return fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?playlist_id=%s", playlistID)
}

// ExtractChannelID extracts channel ID from various YouTube URL formats
func ExtractChannelID(youtubeURL string) (string, error) {
	patterns := []struct {
		pattern *regexp.Regexp
		group   int
	}{
		// /channel/UCxxxxxx
		{regexp.MustCompile(`youtube\.com/channel/(UC[a-zA-Z0-9_-]{22})`), 1},
		// /c/ChannelName or /user/Username - these need additional lookup
		{regexp.MustCompile(`youtube\.com/(?:c|user)/([^/]+)`), 1},
		// /@handle
		{regexp.MustCompile(`youtube\.com/@([^/]+)`), 1},
	}

	for _, p := range patterns {
		if matches := p.pattern.FindStringSubmatch(youtubeURL); len(matches) > p.group {
			return matches[p.group], nil
		}
	}

	return "", fmt.Errorf("could not extract channel ID from URL: %s", youtubeURL)
}

// DiscoverChannelInfo fetches additional info about a channel
func (d *YouTubeDiscoverer) DiscoverChannelInfo(channelID string) (*YouTubeChannel, error) {
	channelURL := fmt.Sprintf("https://www.youtube.com/channel/%s", channelID)

	req, err := http.NewRequest("GET", channelURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", d.userAgent)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract channel title from meta tags
	titlePattern := regexp.MustCompile(`<meta property="og:title" content="([^"]+)"`)
	descPattern := regexp.MustCompile(`<meta property="og:description" content="([^"]+)"`)

	var title, description string
	if matches := titlePattern.FindStringSubmatch(string(body)); len(matches) > 1 {
		title = matches[1]
	}
	if matches := descPattern.FindStringSubmatch(string(body)); len(matches) > 1 {
		description = matches[1]
	}

	return &YouTubeChannel{
		ChannelID:   channelID,
		Title:       title,
		Description: description,
		RSSURL:      GetChannelRSSURL(channelID),
	}, nil
}

// ValidateChannel checks if a YouTube channel exists and has content
func (d *YouTubeDiscoverer) ValidateChannel(channelID string) bool {
	rssURL := GetChannelRSSURL(channelID)

	req, err := http.NewRequest("HEAD", rssURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", d.userAgent)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// DiscoverFromHandle converts a YouTube handle (@username) to channel ID
func (d *YouTubeDiscoverer) DiscoverFromHandle(handle string) (*YouTubeChannel, error) {
	// Remove @ if present
	handle = regexp.MustCompile(`^@?`).ReplaceAllString(handle, "")

	handleURL := fmt.Sprintf("https://www.youtube.com/@%s", handle)

	req, err := http.NewRequest("GET", handleURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", d.userAgent)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract channel ID from the page
	channelIDPattern := regexp.MustCompile(`"channelId":"(UC[a-zA-Z0-9_-]{22})"`)
	if matches := channelIDPattern.FindStringSubmatch(string(body)); len(matches) > 1 {
		return d.DiscoverChannelInfo(matches[1])
	}

	return nil, fmt.Errorf("could not find channel ID for handle: @%s", handle)
}
