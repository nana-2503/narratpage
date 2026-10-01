package seed

// 种子数据。仅在首次安装时写入，用于让新站点立刻可浏览。

type seedCategory struct {
	name        string
	slug        string
	description string
}

var seedCategories = []seedCategory{
	{"技术", "tech", "工程实践、工具与实现细节"},
	{"生活", "life", "日常记录与观察"},
	{"随笔", "notes", "不成体系的思考"},
}

type seedTag struct {
	name        string
	slug        string
	description string
}

// seedTags 的 slug 留空：由 repo.Slugify 统一生成。
//
// 早期版本在此手写英文 slug（如 "frontend"），而标签关联走
// ResolveTagIDs → Slugify("前端") 得到的是 "前端"，两者不一致
// 导致每个中文标签被创建两次。
var seedTags = []seedTag{
	{"Go", "", "服务端与并发"},
	{"前端", "", "浏览器端工程"},
	{"数据库", "", "存储与查询"},
	{"随想", "", "不成体系的想法"},
}

type seedPost struct {
	title        string
	slug         string
	summary      string
	content      string
	categorySlug string
	tagNames     []string
}

var seedPosts = []seedPost{
	{
		title:        "为什么选择 SQLite 作为博客数据库",
		slug:         "why-sqlite-for-blog",
		summary:      "单文件、零运维、读多写少的场景下，SQLite 往往是博客系统最务实的后端存储选择。",
		categorySlug: "tech",
		tagNames:     []string{"数据库", "Go"},
		content: `## 单文件的魅力

SQLite 把整个数据库放进一个文件，备份就是拷贝文件，迁移就是挂载卷。对个人博客这种读多写少的场景，它没有连接池调优，没有慢查询监控，运维成本趋近于零。

## 写放大不是问题

博客的写入集中在作者本人：一天几篇文章、几十条评论。WAL 模式下读写不互斥，这个量级绰绰有余。

## 什么时候该换

当出现多实例部署、写入并发超过百 QPS、或需要全文检索排名时，再迁移到 PostgreSQL 也不迟——届时数据导出只是一条 ` + "`.dump`" + ` 的事。

## 这套系统的做法

本项目把三库差异收敛到一个 dialect 包里：占位符、自增写法、类型名、LIKE 转义各自只实现一次。业务代码写同一份 SQL，换库时无需改动。`,
	},
	{
		title:        "Markdown 写作流水线",
		slug:         "markdown-writing-pipeline",
		summary:      "从草稿到发布，一套基于纯文本的写作流程：本地编辑、版本管理、一键发布。",
		categorySlug: "notes",
		tagNames:     []string{"前端"},
		content: `## 纯文本优先

文章用 Markdown 书写，Git 做版本管理，发布只是推送到仓库。不依赖任何专有格式，十年后依然打得开。

## 结构即大纲

先列标题再填内容。H2 是章节，H3 是论点，写作前大纲先成立，文章就不会散。

## 富文本与纯文本的兼容

编辑器用 Tiptap 做所见即所得，但存储始终是 Markdown。这样既能享受富文本的输入体验，又不会把内容锁进专有格式——导出时不需要额外转换。`,
	},
	{
		title:        "重新开始写博客",
		slug:         "restart-blogging",
		summary:      "清理掉收藏夹里的教程，关掉永远在配置的编辑器，先把第一篇发出去。",
		categorySlug: "life",
		tagNames:     []string{"随想"},
		content: `## 完成比完美重要

搭博客的真正风险不是选错技术栈，而是把全部时间花在配置环境上。先让最小版本跑起来，剩下的在路上迭代。

## 写给未来的自己

保持记录的习惯。三年后回看，这些文字就是时间存在的证据。

## 给下一位折腾者

如果你也在选数据库、选编辑器、选部署方式——先选够用的那个。把精力留给内容本身。`,
	},
}

type seedComment struct {
	postSlug string
	author   string
	content  string
	status   string
}

var seedComments = []seedComment{
	{"why-sqlite-for-blog", "读者甲", "写得很好，已经准备把博客迁到 SQLite 了。", "approved"},
	{"markdown-writing-pipeline", "路过乙", "大纲先行这点很认同。", "approved"},
	{"restart-blogging", "游客丙", "占位待审核的一条评论。", "pending"},
	{"restart-blogging", "读者甲", "同感，我已经三年没更新了。", "approved"},
}

type seedPage struct {
	title   string
	slug    string
	content string
}

var seedPages = []seedPage{
	{
		title: "关于",
		slug:  "about",
		content: `## 关于这个站点

这是一个自托管的博客系统，代码可读、依赖极少、部署简单。

内容用 Markdown 书写，保存在你自己控制的数据库里。没有广告，没有追踪，没有推荐算法。`,
	},
	{
		title: "友链",
		slug:  "links",
		content: `## 友情链接

在这里放置你喜欢的站点。`,
	},
}
