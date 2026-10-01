package models

// Neighbor 上下篇导航项。
type Neighbor struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// SiteOptions 站点级设置，对应 options 表。
//
// 站点设置存在数据库而非环境变量，是为了让后台可自助修改，
// 无需重启容器——这是与原实现最大的运维差异。
type SiteOptions struct {
	Title       string `json:"title"`
	Tagline     string `json:"tagline"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Locale      string `json:"locale"`
	Timezone    string `json:"timezone"`
	DateFormat  string `json:"date_format"`
	// 社交与页脚链接
	Links []SiteLink `json:"links"`
	// 功能开关
	CommentsEnabled   bool `json:"comments_enabled"`
	CommentModeration bool `json:"comment_moderation"`
	RegistrationOpen  bool `json:"registration_open"`
	// 每页条数
	PostsPerPage int `json:"posts_per_page"`
	// 文章详情页的元信息展示开关
	ShowAuthor  bool `json:"show_author"`
	ShowDate    bool `json:"show_date"`
	ShowReading bool `json:"show_reading_time"`
	ShowTags    bool `json:"show_tags"`
	// ShowCover 是否显示封面图
	ShowCover bool `json:"show_cover"`
	// 统计
	PostCount    int `json:"post_count"`
	PageCount    int `json:"page_count"`
	CommentCount int `json:"comment_count"`
	TagCount     int `json:"tag_count"`
}

// SiteLink 站点链接（导航/页脚/社交）。
type SiteLink struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
	// 位置：header | footer | social
	Position string `json:"position"`
	Target   string `json:"target"`
	Order    int    `json:"order"`
}

// DefaultSiteOptions 返回默认站点设置。
func DefaultSiteOptions() SiteOptions {
	return SiteOptions{
		Title:             "叙页博客系统",
		Tagline:           "",
		Description:       "",
		Locale:            "zh-CN",
		Timezone:          "Asia/Shanghai",
		DateFormat:        "YYYY-MM-DD",
		Links:             []SiteLink{},
		CommentsEnabled:   true,
		CommentModeration: true,
		RegistrationOpen:  false,
		PostsPerPage:      10,
		ShowAuthor:        true,
		ShowDate:          true,
		ShowReading:       true,
		ShowTags:          true,
		ShowCover:         true,
	}
}

// InstallRequest 安装向导提交。
type InstallRequest struct {
	DBType        string `json:"dbType"`
	DBDSN         string `json:"dbDsn"`
	AdminUsername string `json:"adminUsername"`
	AdminPassword string `json:"adminPassword"`
	AdminEmail    string `json:"adminEmail"`
	SiteURL       string `json:"siteUrl"`
	SiteTitle     string `json:"siteTitle"`
	RedisEnabled  bool   `json:"redisEnabled"`
	RedisURL      string `json:"redisUrl"`
}

// InstallResult 安装结果。
type InstallResult struct {
	OK          bool              `json:"ok"`
	Username    string            `json:"username"`
	SiteTitle   string            `json:"siteTitle"`
	DBType      string            `json:"dbType"`
	EnvSnippet  map[string]string `json:"env"`
	NeedRestart bool              `json:"need_restart"`
}
