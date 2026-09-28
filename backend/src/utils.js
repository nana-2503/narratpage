// 共享工具函数

/**
 * 将标题/名称转换为 URL 友好的 slug。
 * 纯符号输入时退化为 `${fallbackPrefix}-<时间戳>` 保证唯一可写。
 */
export function slugify(text, fallbackPrefix) {
  const base = String(text ?? '')
    .toLowerCase()
    .replace(/[^a-z0-9\u4e00-\u9fa5]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return base || `${fallbackPrefix}-${Date.now().toString(36)}`;
}

/**
 * 转义 SQL LIKE 模式中的通配符（% _ \），
 * 避免用户搜索输入改变匹配语义（如 "%" 匹配全部）。
 * 配合 ESCAPE '\' 子句使用。
 */
export function escapeLike(value) {
  return String(value).replace(/[\\%_]/g, (ch) => `\\${ch}`);
}

/** 转义 XML 文本节点（RSS 输出用） */
export function xmlEscape(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;');
}
