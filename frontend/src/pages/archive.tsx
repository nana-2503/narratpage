import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import { useAsync } from '@/hooks/use-async'
import { SiteHeader } from '@/components/site-header'
import { SiteFooter } from '@/components/site-footer'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

/** 归档页：按年月、分类、标签、作者聚合 */
export default function Archive() {
  const { data, error, loading } = useAsync(() => api.archive(), [])

  // 月份数据形如 "2026-09"，转换为「2026 年 9 月」
  const months = useMemo(
    () =>
      (data?.months ?? []).map((m) => {
        const [y, mo] = m.month.split('-')
        return {
          key: m.month,
          label: `${y} 年 ${Number(mo)} 月`,
          count: m.count,
        }
      }),
    [data?.months],
  )

  const activeMonths = (data?.years ?? []).map((y) => y.slug)

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto max-w-3xl flex-1 px-4 pb-16 pt-6">
        <Link to="/" className="text-xs text-muted-foreground hover:text-foreground">
          ← 返回
        </Link>
        <h1 className="mt-3 text-xl font-semibold tracking-tight">归档</h1>

        {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}
        {loading && !data && (
          <p className="mt-4 text-sm text-muted-foreground">加载中</p>
        )}

        {data && (
          <>
            <p className="mt-1 text-sm text-muted-foreground tabular-nums">
              共 {data.total} 篇已发布
            </p>

            <Section title="按月份">
              <ul className="flex flex-wrap gap-1.5">
                {months.length === 0 && <Empty />}
                {months.map((m) => (
                  <li key={m.key}>
                    <Link to={`/?year=${m.key.slice(0, 4)}&month=${m.key.slice(5, 7)}`}>
                      <Badge variant="outline" className="font-normal tabular-nums">
                        {m.label} · {m.count}
                      </Badge>
                    </Link>
                  </li>
                ))}
              </ul>
            </Section>

            <Section title="按年份">
              <ul className="flex flex-wrap gap-1.5">
                {activeMonths.length === 0 && <Empty />}
                {data.years.map((y) => (
                  <li key={y.slug}>
                    <Link to={`/?year=${y.slug}`}>
                      <Button variant="outline" size="sm" className="tabular-nums">
                        {y.slug} · {y.count}
                      </Button>
                    </Link>
                  </li>
                ))}
              </ul>
            </Section>

            <Section title="按分类">
              <ul className="flex flex-wrap gap-1.5">
                {data.categories.length === 0 && <Empty />}
                {data.categories.map((c) => (
                  <li key={c.id}>
                    <Link to={`/category/${c.slug}`}>
                      <Badge variant="secondary" className="font-normal tabular-nums">
                        {c.name} · {c.post_count}
                      </Badge>
                    </Link>
                  </li>
                ))}
              </ul>
            </Section>

            <Section title="按标签">
              <ul className="flex flex-wrap gap-1.5">
                {data.tags.length === 0 && <Empty />}
                {data.tags.map((t) => (
                  <li key={t.id}>
                    <Link to={`/tag/${t.slug}`}>
                      <Badge variant="outline" className="font-normal tabular-nums">
                        {t.name} · {t.post_count}
                      </Badge>
                    </Link>
                  </li>
                ))}
              </ul>
            </Section>

            <Section title="按作者">
              <ul className="flex flex-wrap gap-1.5">
                {data.authors.length === 0 && <Empty />}
                {data.authors.map((a) => (
                  <li key={a.id}>
                    <Link to={`/author/${a.id}`}>
                      <Button variant="outline" size="sm" className="tabular-nums">
                        {a.display_name} · {a.post_count}
                      </Button>
                    </Link>
                  </li>
                ))}
              </ul>
            </Section>
          </>
        )}
      </main>
      <SiteFooter />
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mt-8">
      <h2 className="text-xs font-medium text-muted-foreground">{title}</h2>
      <div className="mt-2">{children}</div>
    </section>
  )
}

function Empty() {
  return <li className="text-sm text-muted-foreground">暂无</li>
}
