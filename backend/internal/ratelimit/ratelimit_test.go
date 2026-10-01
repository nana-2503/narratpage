package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func req(remoteAddr, xff, xri string) *http.Request {
	r := httptest.NewRequest("POST", "/api/auth/login", nil)
	r.RemoteAddr = remoteAddr
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	if xri != "" {
		r.Header.Set("X-Real-IP", xri)
	}
	return r
}

// TestClientIPNoProxy 未启用 trustProxy 时只认 RemoteAddr。
// 这是防伪造的正确默认：XFF 由客户端可控。
func TestClientIPNoProxy(t *testing.T) {
	r := req("10.0.0.5:1234", "1.2.3.4", "5.6.7.8")
	if got := ClientIP(r, false); got != "10.0.0.5" {
		t.Fatalf("期望忽略转发头得到 10.0.0.5，得到 %q", got)
	}
}

// TestClientIPTrustProxy 启用后优先取 X-Real-IP。
//
// 安全要点：X-Forwarded-For 不可作为首选。nginx 的
// $proxy_add_x_forwarded_for 是追加语义，客户端自带的头会留在最前面，
// 取最左跳等于信任客户端，限流可被逐请求伪造 IP 绕过。
func TestClientIPTrustProxy(t *testing.T) {
	cases := []struct {
		name, remote, xff, xri, want string
	}{
		{"优先 X-Real-IP", "10.0.0.5:1234", "1.2.3.4", "5.6.7.8", "5.6.7.8"},
		{"伪造 XFF 无法覆盖 X-Real-IP", "10.0.0.5:1234", "8.8.8.8, 5.6.7.8", "5.6.7.8", "5.6.7.8"},
		{"无 X-Real-IP 时回退 XFF 首跳", "10.0.0.5:1234", "1.2.3.4", "", "1.2.3.4"},
		{"XFF 多跳取首个", "10.0.0.5:1234", "1.2.3.4, 172.17.0.2", "", "1.2.3.4"},
		{"XFF 带空格", "10.0.0.5:1234", "  1.2.3.4 , 9.9.9.9", "", "1.2.3.4"},
		{"无转发头回退 RemoteAddr", "10.0.0.5:1234", "", "", "10.0.0.5"},
		{"IPv6 端口", "[::1]:8080", "", "", "::1"},
		{"X-Real-IP 带空格", "10.0.0.5:1234", "", " 5.6.7.8 ", "5.6.7.8"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClientIP(req(c.remote, c.xff, c.xri), true); got != c.want {
				t.Fatalf("期望 %q，得到 %q", c.want, got)
			}
		})
	}
}

// TestForgedHeaderCannotBypass 伪造 X-Forwarded-For 不得重置限流计数。
//
// 这是实际部署中发现的绕过：nginx 追加 XFF 后，客户端自带的
// 伪造地址排在最前，被后端当成了真实客户端 IP。
func TestForgedHeaderCannotBypass(t *testing.T) {
	l := New(60_000_000_000, 3, "too many", nil, "login").TrustProxy(true)

	// 代理用 X-Real-IP 声明真实客户端；XFF 由客户端伪造并被代理追加
	newReq := func(fake string) *http.Request {
		r := req("10.0.0.5:1234", fake+", 203.0.113.9", "203.0.113.9")
		return r
	}

	allowed := 0
	for i := 0; i < 3; i++ {
		if ok, _, _ := l.allow(newReq("8.8.8." + strconv.Itoa(i))); ok {
			allowed++
		}
	}
	if allowed != 3 {
		t.Fatalf("期望放行 3 次，得到 %d", allowed)
	}

	// 换个伪造 IP 也应继续被限流：真实客户端始终是 203.0.113.9
	if ok, _, _ := l.allow(newReq("1.1.1.1")); ok {
		t.Fatal("伪造 XFF 不应重置限流计数")
	}
}

// TestLimiterPerIP 验证不同 IP 的配额相互独立——
// 这正是代理场景下修复要达到的效果。
func TestLimiterPerIP(t *testing.T) {
	l := New(60_000_000_000, 3, "too many", nil, "login").TrustProxy(true)

	calls := func(ip string) int {
		allowed := 0
		for i := 0; i < 5; i++ {
			ok, _, _ := l.allow(req("10.0.0.5:1234", ip, ""))
			if ok {
				allowed++
			}
		}
		return allowed
	}

	if got := calls("1.1.1.1"); got != 3 {
		t.Fatalf("IP 1.1.1.1 期望放行 3 次，得到 %d", got)
	}
	// 另一个 IP 不应受牵连
	if got := calls("2.2.2.2"); got != 3 {
		t.Fatalf("IP 2.2.2.2 期望独立放行 3 次，得到 %d", got)
	}
	// 第一个 IP 继续被限
	if got := calls("1.1.1.1"); got != 0 {
		t.Fatalf("IP 1.1.1.1 期望已被限流，得到放行 %d 次", got)
	}
}

// TestLimiterSameIPShared 未启用 trustProxy 时，同一代理 IP 共享配额，
// 证明这正是修复前的行为。
func TestLimiterSameIPShared(t *testing.T) {
	l := New(60_000_000_000, 2, "too many", nil, "login")
	allowed := 0
	for i := 0; i < 5; i++ {
		if ok, _, _ := l.allow(req("10.0.0.5:1234", "1.1.1.1", "")); ok {
			allowed++
		}
	}
	if allowed != 2 {
		t.Fatalf("期望共享配额下放行 2 次，得到 %d", allowed)
	}
}

// TestLimiterReset 登录成功后应清空计数。
func TestLimiterReset(t *testing.T) {
	l := New(60_000_000_000, 1, "too many", nil, "login")
	r := req("10.0.0.5:1234", "", "")
	if ok, _, _ := l.allow(r); !ok {
		t.Fatal("首次应放行")
	}
	if ok, _, _ := l.allow(r); ok {
		t.Fatal("第二次应被限流")
	}
	l.Reset(r)
	if ok, _, _ := l.allow(r); !ok {
		t.Fatal("Reset 后应重新放行")
	}
}
