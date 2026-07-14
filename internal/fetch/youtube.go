package fetch

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// YouTubeFetcher fetches YouTube data via public RSS and oEmbed APIs (no API key needed)
type YouTubeFetcher struct {
	Client     *http.Client
	RSSBaseURL string
	OEmbedURL  string
}

// VideoInfo contains metadata for a YouTube video
type VideoInfo struct {
	VideoID     string
	Title       string
	Description string
	ChannelID   string
	ChannelName string
	Published   time.Time
	Thumbnail   string
	Link        string
	Views       int64
	Duration    string
}

// ChannelInfo contains metadata for a YouTube channel
type ChannelInfo struct {
	ChannelID   string
	Name        string
	Description string
	Videos      []VideoInfo
	RSSURL      string
}

// PlaylistInfo contains metadata for a YouTube playlist
type PlaylistInfo struct {
	PlaylistID string
	Title      string
	Videos     []VideoInfo
	RSSURL     string
}

// YouTube RSS feed structures
type youtubeFeed struct {
	XMLName xml.Name `xml:"feed"`
	Title   string   `xml:"title"`
	Author  struct {
		Name string `xml:"name"`
		URI  string `xml:"uri"`
	} `xml:"author"`
	Entries []youtubeEntry `xml:"entry"`
}

type youtubeEntry struct {
	VideoID   string `xml:"videoId"`
	ChannelID string `xml:"channelId"`
	Title     string `xml:"title"`
	Link      struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
	Author struct {
		Name string `xml:"name"`
		URI  string `xml:"uri"`
	} `xml:"author"`
	Published  string `xml:"published"`
	Updated    string `xml:"updated"`
	MediaGroup struct {
		Title       string `xml:"title"`
		Description string `xml:"description"`
		Thumbnail   struct {
			URL    string `xml:"url,attr"`
			Width  int    `xml:"width,attr"`
			Height int    `xml:"height,attr"`
		} `xml:"thumbnail"`
		Content struct {
			URL    string `xml:"url,attr"`
			Type   string `xml:"type,attr"`
			Width  int    `xml:"width,attr"`
			Height int    `xml:"height,attr"`
		} `xml:"content"`
		Community struct {
			Statistics struct {
				Views string `xml:"views,attr"`
			} `xml:"statistics"`
		} `xml:"community"`
	} `xml:"group"`
}

// oEmbed response structure
type oembedResponse struct {
	Title           string `json:"title"`
	AuthorName      string `json:"author_name"`
	AuthorURL       string `json:"author_url"`
	Type            string `json:"type"`
	Height          int    `json:"height"`
	Width           int    `json:"width"`
	Version         string `json:"version"`
	ProviderName    string `json:"provider_name"`
	ProviderURL     string `json:"provider_url"`
	ThumbnailURL    string `json:"thumbnail_url"`
	ThumbnailWidth  int    `json:"thumbnail_width"`
	ThumbnailHeight int    `json:"thumbnail_height"`
	HTML            string `json:"html"`
}

func NewYouTubeFetcher() *YouTubeFetcher {
	return &YouTubeFetcher{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		RSSBaseURL: "https://www.youtube.com/feeds/videos.xml",
		OEmbedURL:  "https://www.youtube.com/oembed",
	}
}

// FetchChannelByID fetches channel info and recent videos via RSS
func (f *YouTubeFetcher) FetchChannelByID(channelID string) (*ChannelInfo, error) {
	rssURL := fmt.Sprintf("%s?channel_id=%s", f.RSSBaseURL, channelID)

	resp, err := f.Client.Get(rssURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel RSS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("channel RSS returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var feed youtubeFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed: %w", err)
	}

	channel := &ChannelInfo{
		ChannelID: channelID,
		Name:      feed.Author.Name,
		RSSURL:    rssURL,
		Videos:    make([]VideoInfo, 0, len(feed.Entries)),
	}

	for _, entry := range feed.Entries {
		video := f.entryToVideoInfo(entry)
		channel.Videos = append(channel.Videos, video)
	}

	return channel, nil
}

// FetchChannelByUsername attempts to resolve a channel by username/handle
// Note: YouTube's public APIs don't directly support username lookup,
// so this fetches from the channel page and extracts the channel ID
func (f *YouTubeFetcher) FetchChannelByUsername(username string) (*ChannelInfo, error) {
	// Try common URL patterns
	patterns := []string{
		"https://www.youtube.com/@%s",
		"https://www.youtube.com/c/%s",
		"https://www.youtube.com/user/%s",
	}

	for _, pattern := range patterns {
		pageURL := fmt.Sprintf(pattern, username)
		channelID, err := f.extractChannelIDFromPage(pageURL)
		if err == nil && channelID != "" {
			return f.FetchChannelByID(channelID)
		}
	}

	return nil, fmt.Errorf("could not resolve channel for username: %s", username)
}

// FetchPlaylist fetches playlist videos via RSS
func (f *YouTubeFetcher) FetchPlaylist(playlistID string) (*PlaylistInfo, error) {
	rssURL := fmt.Sprintf("%s?playlist_id=%s", f.RSSBaseURL, playlistID)

	resp, err := f.Client.Get(rssURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch playlist RSS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("playlist RSS returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var feed youtubeFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed: %w", err)
	}

	playlist := &PlaylistInfo{
		PlaylistID: playlistID,
		Title:      feed.Title,
		RSSURL:     rssURL,
		Videos:     make([]VideoInfo, 0, len(feed.Entries)),
	}

	for _, entry := range feed.Entries {
		video := f.entryToVideoInfo(entry)
		playlist.Videos = append(playlist.Videos, video)
	}

	return playlist, nil
}

// FetchVideoMetadata fetches metadata for a single video via oEmbed
func (f *YouTubeFetcher) FetchVideoMetadata(videoID string) (*VideoInfo, error) {
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	params := url.Values{}
	params.Set("url", videoURL)
	params.Set("format", "json")

	oembedURL := fmt.Sprintf("%s?%s", f.OEmbedURL, params.Encode())

	resp, err := f.Client.Get(oembedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch oEmbed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oEmbed returned status %d", resp.StatusCode)
	}

	var oembed oembedResponse
	if err := json.NewDecoder(resp.Body).Decode(&oembed); err != nil {
		return nil, fmt.Errorf("failed to parse oEmbed response: %w", err)
	}

	// Extract channel ID from author URL
	channelID := ""
	if oembed.AuthorURL != "" {
		channelID = extractChannelIDFromURL(oembed.AuthorURL)
	}

	return &VideoInfo{
		VideoID:     videoID,
		Title:       oembed.Title,
		ChannelID:   channelID,
		ChannelName: oembed.AuthorName,
		Thumbnail:   oembed.ThumbnailURL,
		Link:        videoURL,
	}, nil
}

// FetchMultipleVideos fetches metadata for multiple videos
func (f *YouTubeFetcher) FetchMultipleVideos(videoIDs []string) ([]VideoInfo, error) {
	videos := make([]VideoInfo, 0, len(videoIDs))

	for _, id := range videoIDs {
		video, err := f.FetchVideoMetadata(id)
		if err != nil {
			// Log error but continue with other videos
			continue
		}
		videos = append(videos, *video)
	}

	return videos, nil
}

// GetChannelRSSURL returns the RSS feed URL for a channel
func (f *YouTubeFetcher) GetChannelRSSURL(channelID string) string {
	return fmt.Sprintf("%s?channel_id=%s", f.RSSBaseURL, channelID)
}

// GetPlaylistRSSURL returns the RSS feed URL for a playlist
func (f *YouTubeFetcher) GetPlaylistRSSURL(playlistID string) string {
	return fmt.Sprintf("%s?playlist_id=%s", f.RSSBaseURL, playlistID)
}

// ParseYouTubeURL extracts video ID, channel ID, or playlist ID from various YouTube URL formats
func ParseYouTubeURL(rawURL string) (kind string, id string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.ToLower(u.Host)
	if !strings.Contains(host, "youtube.com") && !strings.Contains(host, "youtu.be") {
		return "", "", fmt.Errorf("not a YouTube URL")
	}

	// Short URL: youtu.be/VIDEO_ID
	if strings.Contains(host, "youtu.be") {
		videoID := strings.TrimPrefix(u.Path, "/")
		if videoID != "" {
			return "video", videoID, nil
		}
	}

	path := u.Path
	query := u.Query()

	// Video URL: youtube.com/watch?v=VIDEO_ID
	if videoID := query.Get("v"); videoID != "" {
		return "video", videoID, nil
	}

	// Playlist URL: youtube.com/playlist?list=PLAYLIST_ID
	if playlistID := query.Get("list"); playlistID != "" && path == "/playlist" {
		return "playlist", playlistID, nil
	}

	// Channel URL patterns
	channelPatterns := []struct {
		prefix string
		kind   string
	}{
		{"/channel/", "channel"},
		{"/c/", "handle"},
		{"/user/", "user"},
		{"/@", "handle"},
	}

	for _, p := range channelPatterns {
		if strings.HasPrefix(path, p.prefix) {
			id := strings.TrimPrefix(path, p.prefix)
			id = strings.Split(id, "/")[0] // Remove any trailing path
			if id != "" {
				return p.kind, id, nil
			}
		}
	}

	// Shorts URL: youtube.com/shorts/VIDEO_ID
	if strings.HasPrefix(path, "/shorts/") {
		videoID := strings.TrimPrefix(path, "/shorts/")
		videoID = strings.Split(videoID, "/")[0]
		if videoID != "" {
			return "video", videoID, nil
		}
	}

	// Live URL: youtube.com/live/VIDEO_ID
	if strings.HasPrefix(path, "/live/") {
		videoID := strings.TrimPrefix(path, "/live/")
		videoID = strings.Split(videoID, "/")[0]
		if videoID != "" {
			return "video", videoID, nil
		}
	}

	return "", "", fmt.Errorf("could not parse YouTube URL: %s", rawURL)
}

// Helper function to convert RSS entry to VideoInfo
func (f *YouTubeFetcher) entryToVideoInfo(entry youtubeEntry) VideoInfo {
	published, _ := time.Parse(time.RFC3339, entry.Published)

	var views int64
	if entry.MediaGroup.Community.Statistics.Views != "" {
		fmt.Sscanf(entry.MediaGroup.Community.Statistics.Views, "%d", &views)
	}

	description := entry.MediaGroup.Description
	// Truncate long descriptions
	if len(description) > 2000 {
		description = description[:2000]
	}

	return VideoInfo{
		VideoID:     entry.VideoID,
		Title:       entry.Title,
		Description: description,
		ChannelID:   entry.ChannelID,
		ChannelName: entry.Author.Name,
		Published:   published,
		Thumbnail:   entry.MediaGroup.Thumbnail.URL,
		Link:        entry.Link.Href,
		Views:       views,
	}
}

// extractChannelIDFromPage fetches a YouTube page and extracts the channel ID
func (f *YouTubeFetcher) extractChannelIDFromPage(pageURL string) (string, error) {
	resp, err := f.Client.Get(pageURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("page returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Look for channel ID in the page
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`"channelId":"(UC[a-zA-Z0-9_-]{22})"`),
		regexp.MustCompile(`"externalId":"(UC[a-zA-Z0-9_-]{22})"`),
		regexp.MustCompile(`/channel/(UC[a-zA-Z0-9_-]{22})`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindSubmatch(body)
		if len(matches) > 1 {
			return string(matches[1]), nil
		}
	}

	return "", fmt.Errorf("channel ID not found in page")
}

// extractChannelIDFromURL extracts channel ID from a channel URL
func extractChannelIDFromURL(rawURL string) string {
	if strings.Contains(rawURL, "/channel/") {
		parts := strings.Split(rawURL, "/channel/")
		if len(parts) > 1 {
			id := strings.Split(parts[1], "/")[0]
			return id
		}
	}
	return ""
}

// IsValidChannelID checks if a string looks like a valid YouTube channel ID
func IsValidChannelID(id string) bool {
	if len(id) != 24 {
		return false
	}
	if !strings.HasPrefix(id, "UC") {
		return false
	}
	matched, _ := regexp.MatchString(`^UC[a-zA-Z0-9_-]{22}$`, id)
	return matched
}

// IsValidVideoID checks if a string looks like a valid YouTube video ID
func IsValidVideoID(id string) bool {
	if len(id) != 11 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]{11}$`, id)
	return matched
}

// IsValidPlaylistID checks if a string looks like a valid YouTube playlist ID
func IsValidPlaylistID(id string) bool {
	if len(id) < 2 {
		return false
	}
	// Playlist IDs start with PL, UU, LL, FL, etc.
	prefixes := []string{"PL", "UU", "LL", "FL", "RD", "OL"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

// VideoThumbnails returns all available thumbnail URLs for a video
func VideoThumbnails(videoID string) map[string]string {
	return map[string]string{
		"default":  fmt.Sprintf("https://i.ytimg.com/vi/%s/default.jpg", videoID),
		"medium":   fmt.Sprintf("https://i.ytimg.com/vi/%s/mqdefault.jpg", videoID),
		"high":     fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", videoID),
		"standard": fmt.Sprintf("https://i.ytimg.com/vi/%s/sddefault.jpg", videoID),
		"maxres":   fmt.Sprintf("https://i.ytimg.com/vi/%s/maxresdefault.jpg", videoID),
	}
}
