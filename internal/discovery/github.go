package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHubDiscoverer discovers GitHub repositories and awesome lists
type GitHubDiscoverer struct {
	httpClient *http.Client
	userAgent  string
	apiToken   string // Optional GitHub token for higher rate limits
}

// GitHubRepo represents a GitHub repository
type GitHubRepo struct {
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	Stars       int    `json:"stargazers_count"`
	Forks       int    `json:"forks_count"`
	Language    string `json:"language"`
	Topics      []string `json:"topics"`
	UpdatedAt   string `json:"updated_at"`
}

// GitHubSearchResponse represents the GitHub search API response
type GitHubSearchResponse struct {
	TotalCount int          `json:"total_count"`
	Items      []GitHubRepo `json:"items"`
}

// NewGitHubDiscoverer creates a new GitHub discoverer
func NewGitHubDiscoverer() *GitHubDiscoverer {
	return &GitHubDiscoverer{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		userAgent:  "Mimir/1.0",
	}
}

// NewGitHubDiscovererWithToken creates a GitHub discoverer with API token
func NewGitHubDiscovererWithToken(token string) *GitHubDiscoverer {
	d := NewGitHubDiscoverer()
	d.apiToken = token
	return d
}

func (d *GitHubDiscoverer) Name() string { return "github" }

// Discover finds GitHub repositories and awesome lists for the topic
func (d *GitHubDiscoverer) Discover(topic string, keywords []string, limit int) ([]Source, error) {
	var sources []Source
	seen := make(map[string]bool)

	// Search for awesome lists first (highest quality)
	awesomeSources := d.searchAwesome(topic, limit/2)
	for _, s := range awesomeSources {
		if !seen[s.URL] {
			seen[s.URL] = true
			sources = append(sources, s)
		}
	}

	// Search general repositories
	repoSources := d.searchRepos(topic, keywords, limit/2)
	for _, s := range repoSources {
		if !seen[s.URL] {
			seen[s.URL] = true
			sources = append(sources, s)
		}
	}

	if len(sources) > limit {
		sources = sources[:limit]
	}

	return sources, nil
}

// searchAwesome searches for awesome-* curated lists
func (d *GitHubDiscoverer) searchAwesome(topic string, limit int) []Source {
	var sources []Source

	queries := []string{
		fmt.Sprintf("awesome+%s", url.QueryEscape(topic)),
		fmt.Sprintf("awesome-%s", url.QueryEscape(strings.ToLower(strings.ReplaceAll(topic, " ", "-")))),
	}

	for _, q := range queries {
		apiURL := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=stars&per_page=%d", q, limit)

		repos, err := d.fetchRepos(apiURL)
		if err != nil {
			continue
		}

		for _, repo := range repos {
			// Awesome lists are typically curated markdown files
			sources = append(sources, Source{
				URL:         repo.HTMLURL,
				Title:       repo.FullName,
				Description: truncate(repo.Description, 200),
				Type:        "github",
				Score:       normalizeStars(repo.Stars),
			})

			// Also add releases RSS feed
			releasesURL := fmt.Sprintf("%s/releases.atom", repo.HTMLURL)
			sources = append(sources, Source{
				URL:         releasesURL,
				Title:       fmt.Sprintf("%s Releases", repo.FullName),
				Description: "GitHub releases feed",
				Type:        "rss",
			})
		}
	}

	return sources
}

// searchRepos searches for general repositories
func (d *GitHubDiscoverer) searchRepos(topic string, keywords []string, limit int) []Source {
	var sources []Source

	// Build search queries
	searchTerms := append([]string{topic}, keywords...)
	if len(searchTerms) > 3 {
		searchTerms = searchTerms[:3]
	}

	for _, term := range searchTerms {
		apiURL := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=stars&per_page=%d",
			url.QueryEscape(term), limit/len(searchTerms)+1)

		repos, err := d.fetchRepos(apiURL)
		if err != nil {
			continue
		}

		for _, repo := range repos {
			// Add releases feed
			releasesURL := fmt.Sprintf("%s/releases.atom", repo.HTMLURL)
			sources = append(sources, Source{
				URL:         releasesURL,
				Title:       fmt.Sprintf("%s Releases", repo.FullName),
				Description: truncate(repo.Description, 200),
				Type:        "rss",
				Score:       normalizeStars(repo.Stars),
			})

			// Add commits feed for active repos
			if repo.Stars > 100 {
				commitsURL := fmt.Sprintf("%s/commits.atom", repo.HTMLURL)
				sources = append(sources, Source{
					URL:         commitsURL,
					Title:       fmt.Sprintf("%s Commits", repo.FullName),
					Description: "GitHub commits feed",
					Type:        "rss",
				})
			}
		}
	}

	return sources
}

// fetchRepos fetches repositories from the GitHub API
func (d *GitHubDiscoverer) fetchRepos(apiURL string) ([]GitHubRepo, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", d.userAgent)
	if d.apiToken != "" {
		req.Header.Set("Authorization", "token "+d.apiToken)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var result GitHubSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Items, nil
}

// DiscoverByTopic finds repositories matching specific GitHub topics
func (d *GitHubDiscoverer) DiscoverByTopic(githubTopic string, limit int) ([]Source, error) {
	apiURL := fmt.Sprintf("https://api.github.com/search/repositories?q=topic:%s&sort=stars&per_page=%d",
		url.QueryEscape(githubTopic), limit)

	repos, err := d.fetchRepos(apiURL)
	if err != nil {
		return nil, err
	}

	var sources []Source
	for _, repo := range repos {
		sources = append(sources, Source{
			URL:         fmt.Sprintf("%s/releases.atom", repo.HTMLURL),
			Title:       repo.FullName,
			Description: truncate(repo.Description, 200),
			Type:        "rss",
			Score:       normalizeStars(repo.Stars),
		})
	}

	return sources, nil
}

// DiscoverTrending finds trending repositories
func (d *GitHubDiscoverer) DiscoverTrending(language string, period string, limit int) ([]Source, error) {
	// GitHub doesn't have a trending API, so we search for recently created popular repos
	dateRange := "created:>2024-01-01"
	if period == "weekly" {
		dateRange = "created:>2024-06-01"
	}

	query := dateRange + "+stars:>100"
	if language != "" {
		query += "+language:" + url.QueryEscape(language)
	}

	apiURL := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=stars&order=desc&per_page=%d",
		url.QueryEscape(query), limit)

	repos, err := d.fetchRepos(apiURL)
	if err != nil {
		return nil, err
	}

	var sources []Source
	for _, repo := range repos {
		sources = append(sources, Source{
			URL:         fmt.Sprintf("%s/releases.atom", repo.HTMLURL),
			Title:       repo.FullName,
			Description: truncate(repo.Description, 200),
			Type:        "rss",
			Score:       normalizeStars(repo.Stars),
		})
	}

	return sources, nil
}

// Helper functions

func normalizeStars(stars int) float64 {
	if stars <= 0 {
		return 0.0
	}
	// Log scale normalization: 10k stars = 1.0
	normalized := float64(stars) / 10000.0
	if normalized > 1.0 {
		normalized = 1.0
	}
	return normalized
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
