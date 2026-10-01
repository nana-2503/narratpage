package repo

import (
	"database/sql"
	"strings"

	"narratpage/internal/models"
)

// Comments 评论仓储。
type Comments struct {
	DB     *sql.DB
	DBType string
}

func NewComments(database *sql.DB, dbType string) *Comments {
	return &Comments{DB: database, DBType: dbType}
}

const commentColumns = `cm.id, cm.post_id, cm.parent_id, cm.author_id, cm.author,
	cm.email, cm.website, cm.content, cm.status, cm.is_pingback, cm.created_at`

// scanComment 扫描评论行。布尔列用 sql.NullBool 承接，
// 原因同 scanUser：PostgreSQL 的 BOOLEAN 扫进 int 会类型报错。
func scanComment(row interface{ Scan(...any) error }) (*models.Comment, error) {
	var c models.Comment
	var pingback sql.NullBool
	if err := row.Scan(&c.ID, &c.PostID, &c.ParentID, &c.AuthorID, &c.Author,
		&c.Email, &c.Website, &c.Content, &c.Status, &pingback, &c.CreatedAt); err != nil {
		return nil, err
	}
	c.IsPingback = pingback.Valid && pingback.Bool
	return &c, nil
}

// CommentQuery 评论查询条件。
type CommentQuery struct {
	PostID int64
	Status string
	Search string
	Page   int
	// Limit 为 0 时不分页（后台审核列表默认一次取回）
	Limit int
}

// List 查询评论。公开访问仅返回已通过（除非管理员显式指定状态）。
func (r *Comments) List(q CommentQuery, v Viewer) (models.Paged[models.Comment], error) {
	where := []string{"1 = 1"}
	var args []any

	if q.PostID > 0 {
		where = append(where, "cm.post_id = ?")
		args = append(args, q.PostID)
	}
	// 状态可见性：管理员可看任意状态，其他角色仅看已通过
	if v.IsAdmin {
		if q.Status != "" {
			where = append(where, "cm.status = ?")
			args = append(args, q.Status)
		}
	} else {
		where = append(where, "cm.status = 'approved'")
		// 回复只挂在已通过的父评论下，避免通过父评论看到被拒回复
		where = append(where, "(cm.parent_id IS NULL OR EXISTS "+
			"(SELECT 1 FROM comments p WHERE p.id = cm.parent_id AND p.status = 'approved'))")
	}
	if q.Search != "" {
		like := "%" + dialectEscape(q.Search) + "%"
		where = append(where, "(cm.author LIKE ? ESCAPE '!' OR cm.content LIKE ? ESCAPE '!')")
		args = append(args, like, like)
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")

	total, err := dbCount(r.DB, r.DBType,
		"SELECT COUNT(*) FROM comments cm "+whereSQL, args...)
	if err != nil {
		return models.Paged[models.Comment]{}, err
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := 0
	if q.Limit <= 0 {
		page := q.Page
		if page < 1 {
			page = 1
		}
		offset = (page - 1) * limit
	}
	rows, err := dbQuery(r.DB, r.DBType,
		"SELECT "+commentColumns+" FROM comments cm "+whereSQL+
			" ORDER BY cm.created_at ASC, cm.id ASC LIMIT "+itoa(limit)+" OFFSET "+itoa(offset),
		args...)
	if err != nil {
		return models.Paged[models.Comment]{}, err
	}
	defer rows.Close()

	items := []models.Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return models.Paged[models.Comment]{}, err
		}
		items = append(items, *c)
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	if q.Limit > 0 {
		page = 1
	}
	return models.NewPaged(items, total, page, limit), nil
}

// ByID 取单条评论。
func (r *Comments) ByID(id int64) (*models.Comment, error) {
	return scanComment(dbQueryRow(r.DB, r.DBType,
		"SELECT "+commentColumns+" FROM comments cm WHERE cm.id = ?", id))
}

// Create 新增评论。ipHash 为提交方 IP 的加盐哈希，
// 供反垃圾冷却判定使用，不保留原始地址。
func (r *Comments) Create(c *models.Comment, ipHash string) (int64, error) {
	return dbInsert(r.DB, r.DBType, "comments",
		[]string{"post_id", "parent_id", "author_id", "author", "email", "website",
			"content", "status", "is_pingback", "ip_hash", "created_at"},
		[]any{c.PostID, c.ParentID, c.AuthorID, c.Author, c.Email, c.Website,
			c.Content, c.Status, Dialect(r.DBType).QuoteBool(c.IsPingback), ipHash, nowISO()})
}

// UpdateStatus 更新审核状态。
func (r *Comments) UpdateStatus(id int64, status string) (int64, error) {
	return dbUpdate(r.DB, r.DBType, "comments",
		map[string]any{"status": status}, "id = ?", id)
}

// BulkUpdateStatus 批量更新状态。
func (r *Comments) BulkUpdateStatus(ids []int64, status string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")
	args := make([]any, 0, len(ids)+1)
	args = append(args, status)
	for _, id := range ids {
		args = append(args, id)
	}
	res, err := dbExec(r.DB, r.DBType,
		"UPDATE comments SET status = ? WHERE id IN ("+ph+")", args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Delete 删除评论。
func (r *Comments) Delete(id int64) (int64, error) {
	return dbDelete(r.DB, r.DBType, "comments", "id = ?", id)
}

// BulkDelete 批量删除。
func (r *Comments) BulkDelete(ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	res, err := dbExec(r.DB, r.DBType,
		"DELETE FROM comments WHERE id IN ("+ph+")", args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CountByPost 统计某文章已通过的评论数。
func (r *Comments) CountByPost(postID int64) int {
	n, _ := dbCount(r.DB, r.DBType,
		"SELECT COUNT(*) FROM comments WHERE post_id = ? AND status = 'approved'", postID)
	return n
}

// CountPending 待审核评论数（后台角标）。
func (r *Comments) CountPending() int {
	n, _ := dbCount(r.DB, r.DBType,
		"SELECT COUNT(*) FROM comments WHERE status = 'pending'")
	return n
}

// WithPostTitles 为后台列表补上文章标题。
func (r *Comments) WithPostTitles(items []models.Comment) ([]models.Comment, error) {
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]int64, 0, len(items))
	for _, c := range items {
		ids = append(ids, c.PostID)
	}
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := dbQuery(r.DB, r.DBType,
		"SELECT id, title FROM posts WHERE id IN ("+ph+")", args...)
	if err != nil {
		return items, err
	}
	defer rows.Close()
	titles := map[int64]string{}
	for rows.Next() {
		var id int64
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			return items, err
		}
		titles[id] = title
	}
	for i := range items {
		if title, ok := titles[items[i].PostID]; ok {
			t := title
			items[i].PostTitle = &t
		}
	}
	return items, nil
}

// BuildTree 将平铺评论组装为两级树（顶层 + replies）。
//
// 父评论不在结果集内时（例如父级被拒绝但回复已通过），
// 该回复会被提升为顶层，避免整条回复丢失。
func BuildTree(items []models.Comment) []models.Comment {
	working := make([]models.Comment, len(items))
	copy(working, items)

	// isReply 单独记录「已被挂到某父级下」，而不是改写 ParentID：
	// ParentID 是要返回给前端的字段，就地清空会丢失真实父级信息。
	byID := make(map[int64]int, len(working))
	isReply := make([]bool, len(working))
	for i := range working {
		working[i].Replies = nil
		byID[working[i].ID] = i
	}

	// 先把回复挂到父级上。顺序无关紧要：父级无论出现在
	// 切片中的哪一位，都通过索引定位到同一个槽位。
	for i := range working {
		if working[i].ParentID == nil {
			continue
		}
		pi, ok := byID[*working[i].ParentID]
		if !ok || pi == i {
			// 父级不在结果集内（被删除/未过审）时，
			// 该回复提升为顶层，避免内容凭空丢失
			continue
		}
		working[pi].Replies = append(working[pi].Replies, working[i])
		isReply[i] = true
	}

	roots := make([]models.Comment, 0, len(working))
	for i := range working {
		if !isReply[i] {
			roots = append(roots, working[i])
		}
	}
	return roots
}
