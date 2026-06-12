package rue

import "testing"

func noopChain() HandlersChain { return HandlersChain{func(c *Context) {}} }

// 冲突矩阵：换结构后必须保持对等的注册期 panic 行为
func TestRouter_ConflictMatrix(t *testing.T) {
	mustPanic := func(name string, fn func()) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic, got none")
				}
			}()
			fn()
		})
	}
	mustNotPanic := func(name string, fn func()) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("unexpected panic: %v", r)
				}
			}()
			fn()
		})
	}

	mustPanic("same-position different param names", func() {
		r := newRouter()
		r.addRoute("GET", "/user/:id", noopChain())
		r.addRoute("GET", "/user/:name", noopChain())
	})
	mustPanic("duplicate registration", func() {
		r := newRouter()
		r.addRoute("GET", "/a/b", noopChain())
		r.addRoute("GET", "/a/b", noopChain())
	})
	mustPanic("empty method", func() {
		r := newRouter()
		r.addRoute("", "/a", noopChain())
	})
	mustPanic("path without leading slash", func() {
		r := newRouter()
		r.addRoute("GET", "a/b", noopChain())
	})
	mustPanic("empty handlers", func() {
		r := newRouter()
		r.addRoute("GET", "/a", HandlersChain{})
	})
	mustPanic("two wildcards in one segment", func() {
		r := newRouter()
		r.addRoute("GET", "/a/:x:y", noopChain())
	})
	mustPanic("unnamed param", func() {
		r := newRouter()
		r.addRoute("GET", "/a/:", noopChain())
	})
	mustPanic("catch-all not at end", func() {
		r := newRouter()
		r.addRoute("GET", "/a/*x/b", noopChain())
	})
	mustNotPanic("static and param siblings coexist", func() {
		r := newRouter()
		r.addRoute("GET", "/user/new", noopChain())
		r.addRoute("GET", "/user/:id", noopChain())
	})
}

// 段内前缀参数（gin 兼容形态）
func TestRouter_PrefixParamSegment(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/user_:id", noopChain())
	var ps Params
	h, pattern, ok := r.getValue("GET", "/user_42", &ps)
	if !ok || h == nil {
		t.Fatal("expected /user_42 to match /user_:id")
	}
	if pattern != "/user_:id" {
		t.Fatalf("pattern = %q, want /user_:id", pattern)
	}
	if v := ps.ByName("id"); v != "42" {
		t.Fatalf("id = %q, want 42", v)
	}
}

// rue.go:153 重定向探测用 nil params 调 getValue
func TestRouter_NilParams(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/x/:id", noopChain())
	h, _, ok := r.getValue("GET", "/x/1", nil)
	if !ok || h == nil {
		t.Fatal("nil params lookup must still match")
	}
}

// 根路由与根级通配
func TestRouter_RootForms(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/", noopChain())
	r.addRoute("GET", "/:x", noopChain())
	r.addRoute("POST", "/*rest", noopChain())

	if _, _, ok := r.getValue("GET", "/", nil); !ok {
		t.Fatal("/ must match")
	}
	var ps Params
	if _, _, ok := r.getValue("GET", "/abc", &ps); !ok || ps.ByName("x") != "abc" {
		t.Fatal("/:x must capture abc")
	}
	ps = ps[:0]
	// 旧实现语义：catch-all 捕获值含前导斜杠（/*rest 对 /a/b/c 捕获 "/a/b/c"）
	if _, _, ok := r.getValue("POST", "/a/b/c", &ps); !ok || ps.ByName("rest") != "/a/b/c" {
		t.Fatalf("/*rest must capture /a/b/c (with leading slash), got %q", ps.ByName("rest"))
	}
}

// 请求路径输入永不 panic（注册期之外无 panic）
func TestRouter_LookupNeverPanics(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/a/:b/c", noopChain())
	r.addRoute("GET", "/files/*path", noopChain())
	inputs := []string{"", "/", "//", "/a", "/a/", "/a//c", "/a/b/c/d", "/%2e%2e", "/files", "/\x00", "/a/b/c"}
	for _, in := range inputs {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("lookup panicked on %q: %v", in, rec)
				}
			}()
			var ps Params
			r.getValue("GET", in, &ps)
		}()
	}
}

// F-001 唯一行为增量：static 分支走死后回退 param 分支
func TestRouter_BacktrackStaticDeadEnd(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/user/new", noopChain())
	r.addRoute("GET", "/user/:id", noopChain())
	var ps Params
	h, pattern, ok := r.getValue("GET", "/user/newx", &ps)
	if !ok || h == nil {
		t.Fatal("/user/newx must fall back to /user/:id")
	}
	if pattern != "/user/:id" || ps.ByName("id") != "newx" {
		t.Fatalf("got pattern=%q id=%q, want /user/:id, newx", pattern, ps.ByName("id"))
	}
}

func TestRouter_BacktrackMultiLevel(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/a/b/c", noopChain())
	r.addRoute("GET", "/a/:x/d", noopChain())
	var ps Params
	_, pattern, ok := r.getValue("GET", "/a/b/d", &ps)
	if !ok || pattern != "/a/:x/d" || ps.ByName("x") != "b" {
		t.Fatalf("/a/b/d must backtrack to /a/:x/d, got ok=%v pattern=%q", ok, pattern)
	}
}

func TestRouter_BacktrackPrefixParamInterplay(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/v1/user_admin/list", noopChain())
	r.addRoute("GET", "/v1/user_:name/info", noopChain())
	var ps Params
	_, pattern, ok := r.getValue("GET", "/v1/user_admin/info", &ps)
	if !ok || pattern != "/v1/user_:name/info" || ps.ByName("name") != "admin" {
		t.Fatalf("must backtrack from static user_admin to prefix-param, got ok=%v pattern=%q", ok, pattern)
	}
}

// 空捕获语义对等（G6 评审发现 3）：非末尾位置捕获可为空，末尾不可
func TestRouter_EmptyCaptureParity(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/a/:b/c", noopChain())
	r.addRoute("GET", "/u_:m/c", noopChain())
	r.addRoute("GET", "/x/:id", noopChain())

	var ps Params
	_, pattern, ok := r.getValue("GET", "/a//c", &ps)
	if !ok || pattern != "/a/:b/c" {
		t.Fatalf("/a//c must match /a/:b/c with empty capture, got ok=%v pattern=%q", ok, pattern)
	}
	if v, found := ps.Get("b"); !found || v != "" {
		t.Fatalf("b = %q (found=%v), want empty string", v, found)
	}

	ps = ps[:0]
	_, pattern, ok = r.getValue("GET", "/u_/c", &ps)
	if !ok || pattern != "/u_:m/c" {
		t.Fatalf("/u_/c must match /u_:m/c with empty capture, got ok=%v pattern=%q", ok, pattern)
	}
	if v, found := ps.Get("m"); !found || v != "" {
		t.Fatalf("m = %q (found=%v), want empty string", v, found)
	}

	// 末尾空段不匹配（与旧实现一致）
	if _, _, ok := r.getValue("GET", "/x/", nil); ok {
		t.Fatal("/x/ must NOT match /x/:id (terminal empty capture)")
	}
}
