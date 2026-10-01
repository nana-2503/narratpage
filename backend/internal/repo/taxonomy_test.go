package repo

import "testing"

// TestSlugify 覆盖 slug 生成的各种输入。
//
// 重点回归：中文标题必须生成稳定的、可读的 slug。
// 早期实现用 `r < 0x3000` 排除 CJK，导致中文全部被过滤、
// 只能得到 tag-<时间戳>，每次都不同，进而造成同名标签重复堆积。
func TestSlugify(t *testing.T) {
	cases := []struct {
		name  string
		input string
		// 不比对时间戳兜底，只比对前缀
		prefix string
		want   string
		stable bool // true 表示可精确比对
	}{
		{name: "英文", input: "Hello World", prefix: "post", want: "hello-world", stable: true},
		{name: "英文带符号", input: "Hello, World! 2024", prefix: "post", want: "hello-world-2024", stable: true},
		{name: "中文", input: "前端开发", prefix: "tag", want: "前端开发", stable: true},
		{name: "中英混合", input: "使用 Go 编写后端", prefix: "post", want: "使用-go-编写后端", stable: true},
		{name: "纯中文标签", input: "数据库", prefix: "tag", want: "数据库", stable: true},
		{name: "数字", input: "2024 年总结", prefix: "post", want: "2024-年总结", stable: true},
		{name: "多余连字符", input: "a---b", prefix: "post", want: "a-b", stable: true},
		{name: "首尾空白", input: "  标题  ", prefix: "post", want: "标题", stable: true},
		{name: "纯符号走兜底", input: "!!!", prefix: "post", want: "post-", stable: false},
		{name: "空串走兜底", input: "", prefix: "cat", want: "cat-", stable: false},
		{name: "emoji 走兜底", input: "🎉🎉", prefix: "post", want: "post-", stable: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Slugify(c.input, c.prefix)
			if c.stable {
				if got != c.want {
					t.Fatalf("期望 %q，得到 %q", c.want, got)
				}
				// 稳定性：同样输入必须得到同样输出
				if again := Slugify(c.input, c.prefix); again != got {
					t.Fatalf("slug 不稳定：%q 与 %q", got, again)
				}
			} else {
				if len(got) <= len(c.prefix) || got[:len(c.prefix)] != c.prefix {
					t.Fatalf("期望以 %q 开头的兜底值，得到 %q", c.prefix, got)
				}
			}
		})
	}
}

// TestSlugifyCJKStable 中文输入在多次调用间必须稳定——
// 这是标签不重复的前提。
func TestSlugifyCJKStable(t *testing.T) {
	first := Slugify("前端", "tag")
	for i := 0; i < 20; i++ {
		if got := Slugify("前端", "tag"); got != first {
			t.Fatalf("第 %d 次调用得到 %q，与首次 %q 不一致", i, got, first)
		}
	}
	if first == "tag" || first[:3] == "tag" && len(first) > 6 {
		t.Fatalf("中文 slug 不应退化为时间戳兜底：%q", first)
	}
}
