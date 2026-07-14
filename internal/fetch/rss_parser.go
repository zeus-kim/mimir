package fetch

import (
	"encoding/xml"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// FeedItem represents a single article/entry from any feed type
type FeedItem struct {
	Title       string
	Link        string
	Description string
	Content     string
	Author      string
	Published   time.Time
	GUID        string
	Categories  []string
}

// Feed represents a parsed RSS/Atom feed
type Feed struct {
	Title       string
	Link        string
	Description string
	Language    string
	Updated     time.Time
	Items       []FeedItem
	FeedType    string // "rss2", "rss1", "atom"
}

// RSSParser handles parsing of RSS 2.0, RSS 1.0, and Atom feeds
type RSSParser struct {
	client *http.Client
}

// NewRSSParser creates a new RSS parser with default settings
func NewRSSParser() *RSSParser {
	return &RSSParser{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewRSSParserWithClient creates a parser with a custom HTTP client
func NewRSSParserWithClient(client *http.Client) *RSSParser {
	return &RSSParser{client: client}
}

// ParseURL fetches and parses a feed from a URL
func (p *RSSParser) ParseURL(feedURL string) (*Feed, error) {
	req, err := http.NewRequest("GET", feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mimir-mcp/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.Parse(resp.Body)
}

// Parse parses feed content from a reader
func (p *RSSParser) Parse(r io.Reader) (*Feed, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	content := string(data)

	// Detect feed type and parse accordingly
	if strings.Contains(content, "<feed") && strings.Contains(content, "xmlns=\"http://www.w3.org/2005/Atom\"") {
		return p.parseAtom(data)
	}
	if strings.Contains(content, "xmlns=\"http://purl.org/rss/1.0/\"") {
		return p.parseRSS1(data)
	}
	// Default to RSS 2.0
	return p.parseRSS2(data)
}

// RSS 2.0 structures
type rss2Feed struct {
	XMLName xml.Name    `xml:"rss"`
	Channel rss2Channel `xml:"channel"`
}

type rss2Channel struct {
	Title       string     `xml:"title"`
	Link        string     `xml:"link"`
	Description string     `xml:"description"`
	Language    string     `xml:"language"`
	PubDate     string     `xml:"pubDate"`
	LastBuild   string     `xml:"lastBuildDate"`
	Items       []rss2Item `xml:"item"`
}

type rss2Item struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description string   `xml:"description"`
	Author      string   `xml:"author"`
	Creator     string   `xml:"http://purl.org/dc/elements/1.1/ creator"`
	PubDate     string   `xml:"pubDate"`
	GUID        string   `xml:"guid"`
	Categories  []string `xml:"category"`
	Content     string   `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
}

func (p *RSSParser) parseRSS2(data []byte) (*Feed, error) {
	var rss rss2Feed
	if err := xml.Unmarshal(data, &rss); err != nil {
		return nil, err
	}

	ch := rss.Channel
	feed := &Feed{
		Title:       cleanText(ch.Title),
		Link:        ch.Link,
		Description: cleanText(ch.Description),
		Language:    ch.Language,
		FeedType:    "rss2",
	}

	if ch.LastBuild != "" {
		feed.Updated = parseDate(ch.LastBuild)
	} else if ch.PubDate != "" {
		feed.Updated = parseDate(ch.PubDate)
	}

	for _, item := range ch.Items {
		author := item.Author
		if author == "" {
			author = item.Creator
		}

		content := item.Content
		if content == "" {
			content = item.Description
		}

		fi := FeedItem{
			Title:       cleanText(item.Title),
			Link:        item.Link,
			Description: cleanText(item.Description),
			Content:     cleanHTML(content),
			Author:      cleanText(author),
			Published:   parseDate(item.PubDate),
			GUID:        item.GUID,
			Categories:  item.Categories,
		}
		if fi.GUID == "" {
			fi.GUID = fi.Link
		}
		feed.Items = append(feed.Items, fi)
	}

	return feed, nil
}

// RSS 1.0 (RDF) structures
type rss1Feed struct {
	XMLName xml.Name    `xml:"RDF"`
	Channel rss1Channel `xml:"channel"`
	Items   []rss1Item  `xml:"item"`
}

type rss1Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Date        string `xml:"http://purl.org/dc/elements/1.1/ date"`
}

type rss1Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Creator     string `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Date        string `xml:"http://purl.org/dc/elements/1.1/ date"`
	Subject     string `xml:"http://purl.org/dc/elements/1.1/ subject"`
	Content     string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
}

func (p *RSSParser) parseRSS1(data []byte) (*Feed, error) {
	var rdf rss1Feed
	if err := xml.Unmarshal(data, &rdf); err != nil {
		return nil, err
	}

	ch := rdf.Channel
	feed := &Feed{
		Title:       cleanText(ch.Title),
		Link:        ch.Link,
		Description: cleanText(ch.Description),
		Updated:     parseDate(ch.Date),
		FeedType:    "rss1",
	}

	for _, item := range rdf.Items {
		content := item.Content
		if content == "" {
			content = item.Description
		}

		var categories []string
		if item.Subject != "" {
			categories = []string{item.Subject}
		}

		fi := FeedItem{
			Title:       cleanText(item.Title),
			Link:        item.Link,
			Description: cleanText(item.Description),
			Content:     cleanHTML(content),
			Author:      cleanText(item.Creator),
			Published:   parseDate(item.Date),
			GUID:        item.Link,
			Categories:  categories,
		}
		feed.Items = append(feed.Items, fi)
	}

	return feed, nil
}

// Atom structures
type atomFeed struct {
	XMLName  xml.Name    `xml:"feed"`
	Title    string      `xml:"title"`
	Subtitle string      `xml:"subtitle"`
	Links    []atomLink  `xml:"link"`
	Updated  string      `xml:"updated"`
	Entries  []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type atomEntry struct {
	Title     string       `xml:"title"`
	Links     []atomLink   `xml:"link"`
	ID        string       `xml:"id"`
	Updated   string       `xml:"updated"`
	Published string       `xml:"published"`
	Summary   string       `xml:"summary"`
	Content   atomContent  `xml:"content"`
	Authors   []atomAuthor `xml:"author"`
	Category  []struct {
		Term string `xml:"term,attr"`
	} `xml:"category"`
}

type atomContent struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type atomAuthor struct {
	Name  string `xml:"name"`
	Email string `xml:"email"`
}

func (p *RSSParser) parseAtom(data []byte) (*Feed, error) {
	var atom atomFeed
	if err := xml.Unmarshal(data, &atom); err != nil {
		return nil, err
	}

	feed := &Feed{
		Title:       cleanText(atom.Title),
		Description: cleanText(atom.Subtitle),
		Updated:     parseDate(atom.Updated),
		FeedType:    "atom",
	}

	// Find the alternate link (main site URL)
	for _, link := range atom.Links {
		if link.Rel == "alternate" || link.Rel == "" {
			feed.Link = link.Href
			break
		}
	}

	for _, entry := range atom.Entries {
		// Find entry link
		var link string
		for _, l := range entry.Links {
			if l.Rel == "alternate" || l.Rel == "" {
				link = l.Href
				break
			}
		}

		// Get author name
		var author string
		if len(entry.Authors) > 0 {
			author = entry.Authors[0].Name
		}

		// Get categories
		var categories []string
		for _, cat := range entry.Category {
			if cat.Term != "" {
				categories = append(categories, cat.Term)
			}
		}

		// Content
		content := entry.Content.Value
		if content == "" {
			content = entry.Summary
		}

		// Published date
		pubDate := entry.Published
		if pubDate == "" {
			pubDate = entry.Updated
		}

		fi := FeedItem{
			Title:       cleanText(entry.Title),
			Link:        link,
			Description: cleanText(entry.Summary),
			Content:     cleanHTML(content),
			Author:      author,
			Published:   parseDate(pubDate),
			GUID:        entry.ID,
			Categories:  categories,
		}
		feed.Items = append(feed.Items, fi)
	}

	return feed, nil
}

// parseDate handles various date formats found in feeds
func parseDate(dateStr string) time.Time {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return time.Time{}
	}

	// Common date formats in RSS/Atom feeds
	formats := []string{
		// RFC 2822 (RSS 2.0)
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"02 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 -0700",

		// RFC 3339 (Atom)
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05",

		// ISO 8601
		"2006-01-02 15:04:05",
		"2006-01-02",

		// Other common formats
		"January 2, 2006",
		"Jan 2, 2006",
		"02/01/2006",
		"01/02/2006",
		"2006/01/02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t
		}
	}

	return time.Time{}
}

// cleanText removes HTML tags and normalizes whitespace
func cleanText(s string) string {
	s = html.UnescapeString(s)
	s = stripHTML(s)
	s = normalizeWhitespace(s)
	return strings.TrimSpace(s)
}

// cleanHTML cleans HTML content while preserving structure
func cleanHTML(s string) string {
	s = html.UnescapeString(s)
	s = normalizeWhitespace(s)
	return strings.TrimSpace(s)
}

var (
	htmlTagRe    = regexp.MustCompile(`<[^>]*>`)
	whitespaceRe = regexp.MustCompile(`[\s\p{Zs}]+`)
	cdataRe      = regexp.MustCompile(`<!\[CDATA\[(.*?)\]\]>`)
)

// stripHTML removes all HTML tags
func stripHTML(s string) string {
	// Handle CDATA sections
	s = cdataRe.ReplaceAllString(s, "$1")
	// Remove HTML tags
	s = htmlTagRe.ReplaceAllString(s, " ")
	return s
}

// normalizeWhitespace collapses multiple whitespace to single space
func normalizeWhitespace(s string) string {
	return whitespaceRe.ReplaceAllString(s, " ")
}

// DetectFeedType checks content to determine feed type without full parsing
func DetectFeedType(content string) string {
	if strings.Contains(content, "<feed") && strings.Contains(content, "xmlns=\"http://www.w3.org/2005/Atom\"") {
		return "atom"
	}
	if strings.Contains(content, "xmlns=\"http://purl.org/rss/1.0/\"") {
		return "rss1"
	}
	if strings.Contains(content, "<rss") {
		return "rss2"
	}
	if strings.Contains(content, "<channel") || strings.Contains(content, "<item") {
		return "rss2"
	}
	return "unknown"
}

// ValidateFeed checks if content appears to be a valid RSS/Atom feed
func ValidateFeed(content string) bool {
	content = strings.ToLower(content[:min(len(content), 500)])
	return strings.Contains(content, "<rss") ||
		strings.Contains(content, "<feed") ||
		strings.Contains(content, "<channel") ||
		strings.Contains(content, "rdf:rdf")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
