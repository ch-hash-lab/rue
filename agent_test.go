package rue

import (
	"net/http"
	"net/url"
	"testing"
)

func TestAgent_Get(t *testing.T) {
	engine := New()
	engine.GET("/hello", func(c *Context) {
		c.Text(http.StatusOK, "Hello, World!")
	})

	agent := NewAgent(engine)
	resp := agent.Get("/hello")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if resp.String() != "Hello, World!" {
		t.Errorf("Body = %s, want 'Hello, World!'", resp.String())
	}
}

func TestAgent_Post(t *testing.T) {
	engine := New()
	engine.POST("/echo", func(c *Context) {
		var data map[string]string
		c.BindJSON(&data)
		c.JSON(http.StatusOK, data)
	})

	agent := NewAgent(engine)
	resp := agent.PostJSON("/echo", map[string]string{"message": "hello"})

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]string
	if err := resp.JSON(&result); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}

	if result["message"] != "hello" {
		t.Errorf("message = %s, want hello", result["message"])
	}
}

func TestAgent_PostForm(t *testing.T) {
	engine := New()
	engine.POST("/form", func(c *Context) {
		name := c.PostForm("name")
		c.Text(http.StatusOK, "Hello, "+name)
	})

	agent := NewAgent(engine)
	resp := agent.PostForm("/form", url.Values{"name": {"John"}})

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if resp.String() != "Hello, John" {
		t.Errorf("Body = %s, want 'Hello, John'", resp.String())
	}
}

func TestAgent_Put(t *testing.T) {
	engine := New()
	engine.PUT("/update", func(c *Context) {
		c.Text(http.StatusOK, "Updated")
	})

	agent := NewAgent(engine)
	resp := agent.Put("/update", nil)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAgent_PutJSON(t *testing.T) {
	engine := New()
	engine.PUT("/update", func(c *Context) {
		var data map[string]string
		c.BindJSON(&data)
		c.JSON(http.StatusOK, data)
	})

	agent := NewAgent(engine)
	resp := agent.PutJSON("/update", map[string]string{"id": "123"})

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAgent_Patch(t *testing.T) {
	engine := New()
	engine.PATCH("/patch", func(c *Context) {
		c.Text(http.StatusOK, "Patched")
	})

	agent := NewAgent(engine)
	resp := agent.Patch("/patch", nil)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAgent_Delete(t *testing.T) {
	engine := New()
	engine.DELETE("/delete/:id", func(c *Context) {
		id := c.Param("id")
		c.Text(http.StatusOK, "Deleted "+id)
	})

	agent := NewAgent(engine)
	resp := agent.Delete("/delete/123")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if resp.String() != "Deleted 123" {
		t.Errorf("Body = %s, want 'Deleted 123'", resp.String())
	}
}

func TestAgent_Head(t *testing.T) {
	engine := New()
	engine.HEAD("/head", func(c *Context) {
		c.SetHeader("X-Custom", "value")
		c.Status(http.StatusOK)
	})

	agent := NewAgent(engine)
	resp := agent.Head("/head")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if resp.Headers.Get("X-Custom") != "value" {
		t.Errorf("X-Custom = %s, want value", resp.Headers.Get("X-Custom"))
	}
}

func TestAgent_Options(t *testing.T) {
	engine := New()
	engine.OPTIONS("/options", func(c *Context) {
		c.SetHeader("Allow", "GET, POST, OPTIONS")
		c.Status(http.StatusOK)
	})

	agent := NewAgent(engine)
	resp := agent.Options("/options")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAgent_Headers(t *testing.T) {
	engine := New()
	engine.GET("/headers", func(c *Context) {
		auth := c.Header("Authorization")
		c.Text(http.StatusOK, auth)
	})

	agent := NewAgent(engine)
	agent.SetHeader("Authorization", "Bearer token123")

	resp := agent.Get("/headers")

	if resp.String() != "Bearer token123" {
		t.Errorf("Body = %s, want 'Bearer token123'", resp.String())
	}
}

func TestAgent_AddHeader(t *testing.T) {
	engine := New()
	engine.GET("/headers", func(c *Context) {
		values := c.Request.Header["X-Custom"]
		c.JSON(http.StatusOK, values)
	})

	agent := NewAgent(engine)
	agent.AddHeader("X-Custom", "value1")
	agent.AddHeader("X-Custom", "value2")

	resp := agent.Get("/headers")

	var values []string
	resp.JSON(&values)

	if len(values) != 2 {
		t.Errorf("Header values count = %d, want 2", len(values))
	}
}

func TestAgent_ClearHeaders(t *testing.T) {
	agent := NewAgent(New())
	agent.SetHeader("X-Custom", "value")
	agent.ClearHeaders()

	if len(agent.headers) != 0 {
		t.Error("Headers should be cleared")
	}
}

func TestAgent_Cookies(t *testing.T) {
	engine := New()
	engine.GET("/cookies", func(c *Context) {
		session, _ := c.Cookie("session")
		c.Text(http.StatusOK, session)
	})

	agent := NewAgent(engine)
	agent.SetCookie(&http.Cookie{Name: "session", Value: "abc123"})

	resp := agent.Get("/cookies")

	if resp.String() != "abc123" {
		t.Errorf("Body = %s, want 'abc123'", resp.String())
	}
}

func TestAgent_ClearCookies(t *testing.T) {
	agent := NewAgent(New())
	agent.SetCookie(&http.Cookie{Name: "test", Value: "value"})
	agent.ClearCookies()

	if len(agent.cookies) != 0 {
		t.Error("Cookies should be cleared")
	}
}

func TestAgent_ResponseCookies(t *testing.T) {
	engine := New()
	engine.GET("/setcookie", func(c *Context) {
		c.SetCookie("session", "xyz789", 3600, "/", "", false, true)
		c.Text(http.StatusOK, "OK")
	})

	agent := NewAgent(engine)
	resp := agent.Get("/setcookie")

	if len(resp.Cookies) == 0 {
		t.Error("Response should have cookies")
	}

	found := false
	for _, cookie := range resp.Cookies {
		if cookie.Name == "session" && cookie.Value == "xyz789" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Session cookie not found in response")
	}
}

func TestAgent_RequestBuilder(t *testing.T) {
	engine := New()
	engine.GET("/search", func(c *Context) {
		q := c.Query("q")
		page := c.Query("page")
		auth := c.Header("Authorization")
		c.JSON(http.StatusOK, H{
			"q":    q,
			"page": page,
			"auth": auth,
		})
	})

	agent := NewAgent(engine)
	resp := agent.NewRequest("GET", "/search").
		Query("q", "test").
		Query("page", "1").
		Header("Authorization", "Bearer token").
		Send()

	var result map[string]string
	resp.JSON(&result)

	if result["q"] != "test" {
		t.Errorf("q = %s, want test", result["q"])
	}
	if result["page"] != "1" {
		t.Errorf("page = %s, want 1", result["page"])
	}
	if result["auth"] != "Bearer token" {
		t.Errorf("auth = %s, want 'Bearer token'", result["auth"])
	}
}

func TestAgent_RequestBuilder_JSON(t *testing.T) {
	engine := New()
	engine.POST("/data", func(c *Context) {
		var data map[string]string
		c.BindJSON(&data)
		c.JSON(http.StatusOK, data)
	})

	agent := NewAgent(engine)
	resp := agent.NewRequest("POST", "/data").
		JSON(map[string]string{"key": "value"}).
		Send()

	var result map[string]string
	resp.JSON(&result)

	if result["key"] != "value" {
		t.Errorf("key = %s, want value", result["key"])
	}
}

func TestAgent_RequestBuilder_Form(t *testing.T) {
	engine := New()
	engine.POST("/form", func(c *Context) {
		name := c.PostForm("name")
		c.Text(http.StatusOK, name)
	})

	agent := NewAgent(engine)
	resp := agent.NewRequest("POST", "/form").
		Form(url.Values{"name": {"Alice"}}).
		Send()

	if resp.String() != "Alice" {
		t.Errorf("Body = %s, want Alice", resp.String())
	}
}

func TestAgent_ChainedCalls(t *testing.T) {
	engine := New()
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	agent := NewAgent(engine).
		SetHeader("X-Custom", "value").
		SetCookie(&http.Cookie{Name: "test", Value: "123"})

	resp := agent.Get("/test")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAgentResponse_String(t *testing.T) {
	resp := &AgentResponse{
		Body: []byte("Hello"),
	}

	if resp.String() != "Hello" {
		t.Errorf("String() = %s, want Hello", resp.String())
	}
}

func TestAgentResponse_JSON(t *testing.T) {
	resp := &AgentResponse{
		Body: []byte(`{"key":"value"}`),
	}

	var result map[string]string
	if err := resp.JSON(&result); err != nil {
		t.Fatalf("JSON error: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("key = %s, want value", result["key"])
	}
}

func TestAgent_NotFound(t *testing.T) {
	engine := New()

	agent := NewAgent(engine)
	resp := agent.Get("/nonexistent")

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
