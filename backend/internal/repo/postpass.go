package repo

import (
	"strings"

	"narratpage/internal/auth"
	"narratpage/internal/db"
)

// PostPassword 读取按 ID 定位的文章访问密码，返回文章 ID 与存储值
// （bcrypt 哈希或历史明文）。不受可见性约束，仅供解锁处理器内部使用，
// 调用方不得把结果写回响应体。
func (r *Repo) PostPassword(id int64) (int64, string, error) {
	var stored string
	err := db.QueryRow(r.DB, r.DBType,
		r.q("SELECT password FROM posts WHERE id = ?"), id).Scan(&stored)
	return id, stored, err
}

// PostPasswordBySlug 按 slug 取访问密码，受可见性约束。
func (r *Repo) PostPasswordBySlug(slug string, v Viewer) (int64, string, error) {
	clause, args := r.visibleClause(v, "")
	var (
		id     int64
		stored string
	)
	err := db.QueryRow(r.DB, r.DBType,
		r.q("SELECT id, password FROM posts WHERE slug = ? AND type = ? AND deleted_at IS NULL"+clause),
		append([]any{slug, "post"}, args...)...).Scan(&id, &stored)
	return id, stored, err
}

// HashPostPassword 把访问密码转成可入库的哈希。空值表示不保护该文章。
func HashPostPassword(plain string) (string, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return "", nil
	}
	return auth.HashPassword(plain)
}

// SetHashedPassword 将访问密码改写为 bcrypt 形式。
// 用于写入时加密，以及验证历史明文后的就地升级。
func (r *Repo) SetHashedPassword(id int64, plain string) error {
	hash, err := HashPostPassword(plain)
	if err != nil {
		return err
	}
	if hash == "" {
		return nil
	}
	_, err = db.Update(r.DB, r.DBType, "posts", map[string]any{"password": hash}, "id = ?", id)
	return err
}