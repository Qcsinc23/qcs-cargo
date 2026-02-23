-- +goose Up
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
);

-- +goose Down
DROP TABLE IF EXISTS blog_posts;
