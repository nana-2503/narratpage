package repo

import (
	"narratpage/internal/dialect"
)

// dialectEscape 转发到方言包的 LIKE 转义实现。
func dialectEscape(v string) string { return dialect.EscapeLike(v) }
