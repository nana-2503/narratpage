// Package models 定义跨 handler 共享的领域模型与查询辅助。
package models

import "time"

// ContentStatus 内容状态。
type ContentStatus string

const (
	StatusDraft     ContentStatus = "draft"
	StatusPublished ContentStatus = "published"
	StatusPending   ContentStatus = "pending"
	StatusPrivate   ContentStatus = "private"
	StatusTrash     ContentStatus = "trash"
)

// ContentType 内容类型。
type ContentType string

const (
	TypePost       ContentType = "post"
	TypePage       ContentType = "page"
	TypeAttachment ContentType = "attachment"
)

// User 用户。
type User struct {
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	Email       string  `json:"email"`
	DisplayName string  `json:"display_name"`
	Role        string  `json:"role"`
	Bio         string  `json:"bio"`
	AvatarURL   string  `json:"avatar_url"`
	Active      bool    `json:"active"`
	CreatedAt   string  `json:"created_at"`
	LastLoginAt *string `json:"last_login_at"`
	// 仅 /api/users/me 返回
	PostCount int `json:"post_count,omitempty"`
}

// Author 文章作者（列表内联）。
type Author struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
}

// Category 分类/页面分类。
type Category struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	IsPage      int    `json:"is_page"`
	Position    int    `json:"position"`
	PostCount   int    `json:"post_count"`
}

// Tag 标签。
type Tag struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	PostCount   int    `json:"post_count"`
}

// Post 文章/页面。
type Post struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Slug    string `json:"slug"`
	Summary string `json:"summary"`
	// Content 不加 omitempty：正文为空时应序列化为 ""，
	// 前端无需区分「空正文」与「字段缺失」。
	Content      string        `json:"content"`
	CoverURL     string        `json:"cover_url"`
	CategoryID   *int64        `json:"category_id"`
	CategoryName *string       `json:"category_name"`
	CategorySlug *string       `json:"category_slug"`
	Status       ContentStatus `json:"status"`
	Type         ContentType   `json:"type"`
	Sticky       bool          `json:"sticky"`
	Password     string        `json:"-"`
	HasPassword  bool          `json:"has_password"`
	MenuOrder    int           `json:"menu_order"`
	AuthorID     *int64        `json:"author_id"`
	Author       *Author       `json:"author,omitempty"`
	Views        int           `json:"views"`
	Template     string        `json:"template,omitempty"`
	Tags         []Tag         `json:"tags,omitempty"`
	CreatedAt    string        `json:"created_at"`
	UpdatedAt    string        `json:"updated_at"`
	PublishedAt  *string       `json:"published_at"`
	ScheduledAt  *string       `json:"scheduled_at"`
	DeletedAt    *string       `json:"deleted_at"`
}

// Comment 评论。
type Comment struct {
	ID         int64   `json:"id"`
	PostID     int64   `json:"post_id"`
	ParentID   *int64  `json:"parent_id"`
	AuthorID   *int64  `json:"author_id"`
	Author     string  `json:"author"`
	Email      string  `json:"-"`
	Website    string  `json:"website,omitempty"`
	Content    string  `json:"content"`
	Status     string  `json:"status"`
	IsPingback bool    `json:"is_pingback"`
	CreatedAt  string  `json:"created_at"`
	PostTitle  *string `json:"post_title,omitempty"`
	// Replies 为二级回复（仅嵌套模式返回）
	Replies []Comment `json:"replies,omitempty"`
}

// Media 媒体附件。
type Media struct {
	ID         int64  `json:"id"`
	Filename   string `json:"filename"`
	URL        string `json:"url"`
	Mime       string `json:"mime"`
	Size       int64  `json:"size"`
	Title      string `json:"title"`
	AltText    string `json:"alt_text"`
	Caption    string `json:"caption"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	UploadedBy *int64 `json:"uploaded_by"`
	CreatedAt  string `json:"created_at"`
}

// Revision 历史版本。
type Revision struct {
	ID        int64  `json:"id"`
	PostID    int64  `json:"post_id"`
	AuthorID  *int64 `json:"author_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Excerpt   string `json:"excerpt"`
	CreatedAt string `json:"created_at"`
}

// Redirect 重定向规则。
type Redirect struct {
	ID        int64  `json:"id"`
	FromPath  string `json:"from_path"`
	ToPath    string `json:"to_path"`
	Hits      int    `json:"hits"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

// Paged 是所有分页列表的统一响应结构。
type Paged[T any] struct {
	Items      []T `json:"items"`
	Total      int `json:"total"`
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalPages int `json:"totalPages"`
}

// NewPaged 组装分页响应，确保 items 序列化为 [] 而非 null。
func NewPaged[T any](items []T, total, page, pageSize int) Paged[T] {
	if items == nil {
		items = []T{}
	}
	totalPages := 0
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	return Paged[T]{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

// ParseDate 解析时间字符串；空串或非法值返回零值。
func ParseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// Deref 解引用可空字符串。
func Deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
