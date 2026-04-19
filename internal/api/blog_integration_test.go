//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBlogPublicSlug_FiltersUnpublished is the CRIT-02 regression test.
// Drafts and future-scheduled posts must NOT be reachable via the public
// /blog/:slug route. Only posts with status='published' AND
// published_at <= CURRENT_TIMESTAMP may be returned.
func TestBlogPublicSlug_FiltersUnpublished(t *testing.T) {
	app := setupTestApp(t)

	// blog_posts is defined in sql/schema/019_blog_posts.sql but is not yet
	// part of sql/migrations (the test bootstrap only runs migrations). Create
	// the table here so this regression test is self-contained without
	// modifying files outside this fix's scope.
	_, err := db.DB().Exec(`
		CREATE TABLE IF NOT EXISTS blog_posts (
			id TEXT PRIMARY KEY,
			slug TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			excerpt TEXT NOT NULL,
			content_md TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT 'ANNOUNCEMENTS',
			status TEXT NOT NULL DEFAULT 'draft',
			published_at DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`)
	require.NoError(t, err)

	now := time.Now().UTC().Format(time.RFC3339)
	// SQLite's CURRENT_TIMESTAMP returns "YYYY-MM-DD HH:MM:SS" (no T, no Z),
	// and `published_at <= CURRENT_TIMESTAMP` does a string compare. RFC3339
	// values (with the 'T' separator) always sort greater than that, which
	// would falsely place even a "now" published_at in the future. Use the
	// SQLite-native datetime format for published_at, and put the "published"
	// row safely in the past so this test isn't clock-flaky.
	publishedPast := time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02 15:04:05")
	future := time.Now().UTC().AddDate(0, 0, 30).Format("2006-01-02 15:04:05")

	// Seed three blog posts: published-now, draft, scheduled-future.
	// blog_posts schema (sql/schema/019_blog_posts.sql):
	//   id, slug, title, excerpt, content_md, category, status,
	//   published_at, created_at, updated_at
	rows := []struct {
		id, slug, title, status, publishedAt string
	}{
		{"blog_pub_001", "published-post", "Published", "published", publishedPast},
		{"blog_drf_001", "draft-post", "Draft", "draft", ""},
		{"blog_fut_001", "future-post", "Future", "published", future},
	}
	for _, r := range rows {
		var pub interface{}
		if r.publishedAt != "" {
			pub = r.publishedAt
		}
		_, err = db.DB().Exec(
			`INSERT INTO blog_posts
			   (id, slug, title, excerpt, content_md, category, status,
			    published_at, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.id, r.slug, r.title, "excerpt", "body", "ANNOUNCEMENTS", r.status,
			pub, now, now,
		)
		require.NoError(t, err, "seed %s", r.id)
	}

	// Published-now: 200
	req := httptest.NewRequest(http.MethodGet, "/api/v1/blog/published-post", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "published post must be reachable")

	// Draft: 404 (or whatever the public-not-found shape is). Must NOT return the row.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/blog/draft-post", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.NotEqual(t, http.StatusOK, resp.StatusCode, "draft must not be exposed publicly")
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)

	// Future-scheduled: also 404.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/blog/future-post", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.NotEqual(t, http.StatusOK, resp.StatusCode, "future-scheduled post must not be exposed publicly")
}
