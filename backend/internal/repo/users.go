package repo

import (
	"database/sql"
	"strings"

	"narratpage/internal/auth"
	"narratpage/internal/db"
	"narratpage/internal/dialect"
	"narratpage/internal/models"
)

// Users 用户仓储。
type Users struct {
	DB     *sql.DB
	DBType string
}

// NewUsers 创建用户仓储。
func NewUsers(database *sql.DB, dbType string) *Users {
	return &Users{DB: database, DBType: dbType}
}

const userColumns = `id, username, email, display_name, role, COALESCE(bio, ''),
	avatar_url, active, created_at, last_login_at`

// scanUser 扫描用户行。Scan 的目标顺序必须与 userColumns 严格一致，
// 错位不会报错、只会静默写入错误的字段。
//
// 布尔列统一用 sql.NullBool 承接：PostgreSQL 的 BOOLEAN 扫进 int 会报
// "converting driver.Value type bool to a int"，而 SQLite / MySQL 的
// 0/1 也能被 database/sql 正确转成 bool。
func scanUser(row interface{ Scan(...any) error }) (*models.User, error) {
	var u models.User
	var active sql.NullBool
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.Role,
		&u.Bio, &u.AvatarURL, &active, &u.CreatedAt, &u.LastLoginAt); err != nil {
		return nil, err
	}
	u.Active = active.Valid && active.Bool
	if u.DisplayName == "" {
		u.DisplayName = u.Username
	}
	return &u, nil
}

// ByID 按 ID 取用户。
func (r *Users) ByID(id int64) (*models.User, error) {
	return scanUser(db.QueryRow(r.DB, r.DBType,
		"SELECT "+userColumns+" FROM users WHERE id = ?", id))
}

// ByUsername 按用户名取用户（含密码哈希，仅登录使用）。
//
// 注意两点：
//  1. SELECT 列顺序必须与 Scan 目标逐一对应，错位会静默写入错误字段；
//  2. 布尔列用 sql.NullBool 承接，否则 PostgreSQL 的 BOOLEAN
//     扫进 int 会报类型转换错误。
func (r *Users) ByUsername(username string) (*models.User, string, error) {
	var id int64
	var name, email, display, role, bio, avatar string
	var active sql.NullBool
	var hash string
	var createdAt string
	var lastLogin sql.NullString
	err := db.QueryRow(r.DB, r.DBType,
		"SELECT "+userColumns+", password_hash FROM users WHERE username = ?",
		username).Scan(&id, &name, &email, &display, &role, &bio, &avatar,
		&active, &createdAt, &lastLogin, &hash)
	if err != nil {
		return nil, "", err
	}
	u := &models.User{
		ID: id, Username: name, Email: email, DisplayName: display,
		Role: role, Bio: bio, AvatarURL: avatar, Active: active.Valid && active.Bool,
		CreatedAt: createdAt,
	}
	if lastLogin.Valid {
		u.LastLoginAt = &lastLogin.String
	}
	if u.DisplayName == "" {
		u.DisplayName = username
	}
	return u, hash, nil
}

// List 分页列出用户（后台）。
func (r *Users) List(page, pageSize int, search string) (models.Paged[models.User], error) {
	where := []string{"1 = 1"}
	var args []any
	if search != "" {
		like := "%" + dialectEscape(search) + "%"
		where = append(where, "(username LIKE ? ESCAPE '!' OR display_name LIKE ? ESCAPE '!' OR email LIKE ? ESCAPE '!')")
		args = append(args, like, like, like)
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")

	total, err := db.Count(r.DB, r.DBType, "SELECT COUNT(*) FROM users "+whereSQL, args...)
	if err != nil {
		return models.Paged[models.User]{}, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	query := "SELECT " + userColumns + " FROM users " + whereSQL +
		" ORDER BY id ASC LIMIT " + itoa(pageSize) + " OFFSET " + itoa(offset)
	rows, err := db.Query(r.DB, r.DBType, query, args...)
	if err != nil {
		return models.Paged[models.User]{}, err
	}
	defer rows.Close()
	items := []models.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return models.Paged[models.User]{}, err
		}
		items = append(items, *u)
	}
	return models.NewPaged(items, total, page, pageSize), nil
}

// CountByRole 统计各角色人数。
func (r *Users) CountByRole() (map[string]int, error) {
	rows, err := db.Query(r.DB, r.DBType, "SELECT role, COUNT(*) FROM users GROUP BY role")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var role string
		var n int
		if err := rows.Scan(&role, &n); err != nil {
			return nil, err
		}
		out[role] = n
	}
	return out, nil
}

// Create 新建用户。
func (r *Users) Create(u *models.User, password string) (int64, error) {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return 0, err
	}
	display := u.DisplayName
	if display == "" {
		display = u.Username
	}
	role := u.Role
	if role == "" {
		role = string(auth.RoleAuthor)
	}
	// 布尔列按方言转换：PostgreSQL 需要真正的 bool
	active := Dialect(r.DBType).QuoteBool(u.Active)
	return db.Insert(r.DB, r.DBType, "users",
		[]string{"username", "email", "display_name", "password_hash", "role", "avatar_url", "active", "created_at"},
		[]any{u.Username, u.Email, display, hash, role, u.AvatarURL, active, db.NowISO()})
}

// UpdateProfile 更新个人资料（不含角色与启用状态）。
//
// 与 Update 分开是刻意的：自助改资料不应有提权路径，
// 角色与启用状态只能由具备用户管理权限的入口修改。
func (r *Users) UpdateProfile(id int64, u *models.User) error {
	_, err := db.Update(r.DB, r.DBType, "users",
		map[string]any{
			"email":        u.Email,
			"display_name": u.DisplayName,
			"bio":          u.Bio,
			"avatar_url":   u.AvatarURL,
		}, "id = ?", id)
	return err
}

// Update 更新用户全部可管理字段（含角色与启用状态）。
func (r *Users) Update(id int64, u *models.User) error {
	_, err := db.Update(r.DB, r.DBType, "users", map[string]any{
		"email":        u.Email,
		"display_name": u.DisplayName,
		"role":         u.Role,
		"bio":          u.Bio,
		"avatar_url":   u.AvatarURL,
		"active":       Dialect(r.DBType).QuoteBool(u.Active),
	}, "id = ?", id)
	return err
}

// UpdatePassword 更新密码。
func (r *Users) UpdatePassword(id int64, password string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = db.Update(r.DB, r.DBType, "users",
		map[string]any{"password_hash": hash}, "id = ?", id)
	return err
}

// TouchLogin 记录最近登录时间。
func (r *Users) TouchLogin(id int64) {
	_, _ = db.Update(r.DB, r.DBType, "users",
		map[string]any{"last_login_at": db.NowISO()}, "id = ?", id)
}

// Delete 删除用户。
func (r *Users) Delete(id int64) (int64, error) {
	return db.Delete(r.DB, r.DBType, "users", "id = ?", id)
}

// CountAdmins 统计管理员数量，用于防止删掉最后一个管理员。
func (r *Users) CountAdmins(excludeID int64) (int, error) {
	return db.Count(r.DB, r.DBType,
		"SELECT COUNT(*) FROM users WHERE role = 'admin' AND active = ? AND id != ?",
		1, excludeID)
}

// UsernameExists 报告用户名是否已被占用。
func (r *Users) UsernameExists(username string, exceptID int64) (bool, error) {
	n, err := db.Count(r.DB, r.DBType,
		"SELECT COUNT(*) FROM users WHERE username = ? AND id != ?", username, exceptID)
	return n > 0, err
}

// ---------- 会话 ----------

// RecordSession 登记一次登录会话。
func (r *Users) RecordSession(tokenID string, userID int64, ip, userAgent string) {
	_, _ = db.Insert(r.DB, r.DBType, "sessions",
		[]string{"token_id", "user_id", "ip", "user_agent", "created_at", "last_seen_at"},
		[]any{tokenID, userID, ip, truncate(userAgent, 500), db.NowISO(), db.NowISO()})
}

// SessionAlive 报告会话是否仍然有效（未登出、未过期）。
// SessionAlive 校验会话是否仍有效，并返回该用户**当前**角色。
//
// 签名与 auth.Middleware 的 sessionCheck 对齐。
// 角色从库里取而不是信任 token 里的快照：降权、停用要立刻生效，
// 否则旧 token 在有效期内仍能提权。
func (r *Users) SessionAlive(tokenID string, userID int64) (string, bool) {
	var role string
	err := db.QueryRow(r.DB, r.DBType,
		"SELECT u.role FROM sessions s JOIN users u ON u.id = s.user_id "+
			"WHERE s.token_id = ? AND s.user_id = ? AND u.active = ?",
		tokenID, userID, dialect.Dialect(r.DBType).QuoteBool(true)).Scan(&role)
	return role, err == nil && role != ""
}

// ListSessions 列出某用户的活跃会话。
func (r *Users) ListSessions(userID int64) ([]Session, error) {
	rows, err := db.Query(r.DB, r.DBType,
		"SELECT id, token_id, ip, user_agent, created_at, last_seen_at "+
			"FROM sessions WHERE user_id = ? ORDER BY last_seen_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.TokenID, &s.IP, &s.UserAgent,
			&s.CreatedAt, &s.LastSeenAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// Session 表示一次登录会话。
type Session struct {
	ID         int64  `json:"id"`
	TokenID    string `json:"-"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
}

// RevokeOtherSessions 登出某用户除 keepTokenID 外的全部会话。
func (r *Users) RevokeOtherSessions(userID int64, keepTokenID string) (int64, error) {
	return db.Delete(r.DB, r.DBType, "sessions", "user_id = ? AND token_id != ?", userID, keepTokenID)
}

// RevokeSession 登出单个会话。
func (r *Users) RevokeSession(tokenID string) error {
	_, err := db.Delete(r.DB, r.DBType, "sessions", "token_id = ?", tokenID)
	return err
}

// RevokeAllForUser 登出某用户全部会话。
func (r *Users) RevokeAllForUser(userID int64) (int64, error) {
	return db.Delete(r.DB, r.DBType, "sessions", "user_id = ?", userID)
}

// PruneSessions 清理过期会话（保留最近 keep 条）。
func (r *Users) PruneSessions(keep int) error {
	_, err := db.Exec(r.DB, r.DBType,
		"DELETE FROM sessions WHERE token_id NOT IN "+
			"(SELECT token_id FROM sessions ORDER BY last_seen_at DESC LIMIT ?)", keep)
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
