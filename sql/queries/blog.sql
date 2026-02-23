-- name: CreateBlogPost :one
INSERT INTO blog_posts (
  id, slug, title, excerpt, content_md, category, status, published_at, created_at, updated_at
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetBlogPostBySlug :one
SELECT * FROM blog_posts
WHERE slug = ? LIMIT 1;

-- name: GetBlogPostByID :one
SELECT * FROM blog_posts
WHERE id = ? LIMIT 1;

-- name: ListPublishedBlogPosts :many
SELECT * FROM blog_posts
WHERE status = 'published' AND published_at <= CURRENT_TIMESTAMP
ORDER BY published_at DESC
LIMIT ? OFFSET ?;

-- name: ListAllBlogPosts :many
SELECT * FROM blog_posts
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: UpdateBlogPost :one
UPDATE blog_posts
SET 
  title = ?,
  slug = ?,
  excerpt = ?,
  content_md = ?,
  category = ?,
  status = ?,
  published_at = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteBlogPost :exec
DELETE FROM blog_posts
WHERE id = ?;
