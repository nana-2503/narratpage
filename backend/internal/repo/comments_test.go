package repo

import (
	"testing"

	"narratpage/internal/models"
)

func ptr(v int64) *int64 { return &v }

// TestBuildTree 覆盖评论树的几种形态。
func TestBuildTree(t *testing.T) {
	t.Run("空输入", func(t *testing.T) {
		if got := BuildTree(nil); len(got) != 0 {
			t.Fatalf("期望空结果，得到 %d", len(got))
		}
	})

	t.Run("单条顶层", func(t *testing.T) {
		got := BuildTree([]models.Comment{{ID: 1, Author: "a"}})
		if len(got) != 1 || len(got[0].Replies) != 0 {
			t.Fatalf("期望 1 顶层 0 回复，得到 %d/%d", len(got), len(got[0].Replies))
		}
	})

	t.Run("一条回复", func(t *testing.T) {
		got := BuildTree([]models.Comment{
			{ID: 1, Author: "楼主"},
			{ID: 2, Author: "回复者", ParentID: ptr(1)},
		})
		if len(got) != 1 {
			t.Fatalf("期望 1 顶层，得到 %d", len(got))
		}
		if len(got[0].Replies) != 1 {
			t.Fatalf("期望 1 条回复，得到 %d", len(got[0].Replies))
		}
		if got[0].Replies[0].Author != "回复者" {
			t.Fatalf("回复内容错误: %v", got[0].Replies[0])
		}
	})

	t.Run("父级在回复之后出现", func(t *testing.T) {
		// 排序不应影响结果：回复先于父级到达时也要正确挂载
		got := BuildTree([]models.Comment{
			{ID: 2, Author: "回复者", ParentID: ptr(1)},
			{ID: 1, Author: "楼主"},
		})
		if len(got) != 1 || got[0].ID != 1 {
			t.Fatalf("期望 1 条顶层且为父级，得到 %+v", got)
		}
		if len(got[0].Replies) != 1 || got[0].Replies[0].ID != 2 {
			t.Fatalf("回复未正确挂载: %+v", got[0].Replies)
		}
	})

	t.Run("多条回复", func(t *testing.T) {
		got := BuildTree([]models.Comment{
			{ID: 1, Author: "楼主"},
			{ID: 2, Author: "A", ParentID: ptr(1)},
			{ID: 3, Author: "B", ParentID: ptr(1)},
		})
		if len(got) != 1 || len(got[0].Replies) != 2 {
			t.Fatalf("期望 1 顶层 2 回复，得到 %d/%d", len(got), len(got[0].Replies))
		}
	})

	t.Run("父级缺失时提升为顶层", func(t *testing.T) {
		// 父评论被删除或未通过审核时，回复不应凭空消失
		got := BuildTree([]models.Comment{
			{ID: 2, Author: "孤儿回复", ParentID: ptr(999)},
		})
		if len(got) != 1 || got[0].ID != 2 {
			t.Fatalf("孤立回复应提升为顶层，得到 %+v", got)
		}
	})

	t.Run("自引用不构成环", func(t *testing.T) {
		got := BuildTree([]models.Comment{
			{ID: 1, Author: "异常", ParentID: ptr(1)},
		})
		if len(got) != 1 {
			t.Fatalf("期望 1 条，得到 %d", len(got))
		}
		if len(got[0].Replies) != 0 {
			t.Fatal("自引用不应产生嵌套")
		}
	})

	t.Run("多棵子树互不干扰", func(t *testing.T) {
		got := BuildTree([]models.Comment{
			{ID: 1, Author: "A楼主"},
			{ID: 2, Author: "A回复", ParentID: ptr(1)},
			{ID: 3, Author: "B楼主"},
			{ID: 4, Author: "B回复", ParentID: ptr(3)},
		})
		if len(got) != 2 {
			t.Fatalf("期望 2 棵子树，得到 %d", len(got))
		}
		for _, root := range got {
			if len(root.Replies) != 1 {
				t.Fatalf("子树 %d 的回复数异常: %d", root.ID, len(root.Replies))
			}
		}
	})
}
