package api

import (
	"database/sql"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func RegisterBlog(g fiber.Router) {
	g.Get("/blog", listBlogPosts)
	g.Get("/blog/:slug", getBlogPostBySlug)

	g.Get("/admin/blog", middleware.RequireAuth, middleware.RequireAdmin, listAllBlogPostsAdmin)
	g.Post("/admin/blog", middleware.RequireAuth, middleware.RequireAdmin, createBlogPost)
	g.Put("/admin/blog/:id", middleware.RequireAuth, middleware.RequireAdmin, updateBlogPost)
	g.Delete("/admin/blog/:id", middleware.RequireAuth, middleware.RequireAdmin, deleteBlogPost)
}

func listBlogPosts(c *fiber.Ctx) error {
	limit := int64(c.QueryInt("limit", 10))
	offset := int64(c.QueryInt("offset", 0))

	posts, err := db.Queries().ListPublishedBlogPosts(c.Context(), gen.ListPublishedBlogPostsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list blog posts"))
	}
	if posts == nil {
		posts = []gen.BlogPost{}
	}
	return c.JSON(fiber.Map{"data": posts})
}

func getBlogPostBySlug(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "slug required"))
	}

	post, err := db.Queries().GetBlogPostBySlug(c.Context(), slug)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Blog post not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to get blog post"))
	}
	return c.JSON(fiber.Map{"data": post})
}

func listAllBlogPostsAdmin(c *fiber.Ctx) error {
	limit := int64(c.QueryInt("limit", 50))
	offset := int64(c.QueryInt("offset", 0))

	posts, err := db.Queries().ListAllBlogPosts(c.Context(), gen.ListAllBlogPostsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list all blog posts"))
	}
	if posts == nil {
		posts = []gen.BlogPost{}
	}
	return c.JSON(fiber.Map{"data": posts})
}

type blogPostBody struct {
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	Excerpt     string  `json:"excerpt"`
	ContentMd   string  `json:"content_md"`
	Category    string  `json:"category"`
	Status      string  `json:"status"`
	PublishedAt *string `json:"published_at"`
}

func createBlogPost(c *fiber.Ctx) error {
	var body blogPostBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}

	if body.Title == "" || body.Slug == "" || body.ContentMd == "" || body.Excerpt == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "title, slug, excerpt, and content_md are required"))
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	var publishedAt sql.NullTime
	if body.PublishedAt != nil && *body.PublishedAt != "" {
		pt, err := time.Parse(time.RFC3339, *body.PublishedAt)
		if err == nil {
			publishedAt = sql.NullTime{Time: pt, Valid: true}
		}
	} else if body.Status == "published" {
		publishedAt = sql.NullTime{Time: now, Valid: true}
	}

	cat := body.Category
	if cat == "" {
		cat = "ANNOUNCEMENTS"
	}

	status := body.Status
	if status == "" {
		status = "draft"
	}

	post, err := db.Queries().CreateBlogPost(c.Context(), gen.CreateBlogPostParams{
		ID:          id,
		Slug:        body.Slug,
		Title:       body.Title,
		Excerpt:     body.Excerpt,
		ContentMd:   body.ContentMd,
		Category:    cat,
		Status:      status,
		PublishedAt: publishedAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create blog post"))
	}

	return c.Status(201).JSON(fiber.Map{"data": post})
}

func updateBlogPost(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}

	var body blogPostBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}

	if body.Title == "" || body.Slug == "" || body.ContentMd == "" || body.Excerpt == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "title, slug, excerpt, and content_md are required"))
	}

	var publishedAt sql.NullTime
	if body.PublishedAt != nil && *body.PublishedAt != "" {
		pt, err := time.Parse(time.RFC3339, *body.PublishedAt)
		if err == nil {
			publishedAt = sql.NullTime{Time: pt, Valid: true}
		}
	}

	cat := body.Category
	if cat == "" {
		cat = "ANNOUNCEMENTS"
	}

	status := body.Status
	if status == "" {
		status = "draft"
	}

	post, err := db.Queries().UpdateBlogPost(c.Context(), gen.UpdateBlogPostParams{
		Title:       body.Title,
		Slug:        body.Slug,
		Excerpt:     body.Excerpt,
		ContentMd:   body.ContentMd,
		Category:    cat,
		Status:      status,
		PublishedAt: publishedAt,
		ID:          id,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Blog post not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update blog post"))
	}

	return c.JSON(fiber.Map{"data": post})
}

func deleteBlogPost(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}

	err := db.Queries().DeleteBlogPost(c.Context(), id)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to delete blog post"))
	}

	return c.JSON(fiber.Map{"status": "success"})
}
