package rue

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHTMLRenderer_Render(t *testing.T) {
	renderer := NewHTMLRenderer()
	renderer.templates = template.Must(template.New("test").Parse("<h1>{{.Title}}</h1>"))

	var buf bytes.Buffer
	err := renderer.Render(&buf, "test", H{"Title": "Hello"})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := "<h1>Hello</h1>"
	if buf.String() != expected {
		t.Errorf("Render output = %s, want %s", buf.String(), expected)
	}
}

func TestHTMLRenderer_NoTemplates(t *testing.T) {
	renderer := NewHTMLRenderer()

	var buf bytes.Buffer
	err := renderer.Render(&buf, "test", nil)
	if err == nil {
		t.Error("Render should fail when no templates loaded")
	}
}

func TestEngine_SetFuncMap(t *testing.T) {
	engine := New()
	engine.SetFuncMap(template.FuncMap{
		"upper": func(s string) string { return s },
	})

	if engine.htmlRenderer == nil {
		t.Error("SetFuncMap should create htmlRenderer")
	}
	if engine.htmlRenderer.funcMap == nil {
		t.Error("SetFuncMap should set funcMap")
	}
}

func TestEngine_Delims(t *testing.T) {
	engine := New()
	engine.Delims("[[", "]]")

	if engine.htmlRenderer == nil {
		t.Error("Delims should create htmlRenderer")
	}
	if engine.htmlRenderer.delims.Left != "[[" {
		t.Errorf("Left delim = %s, want [[", engine.htmlRenderer.delims.Left)
	}
	if engine.htmlRenderer.delims.Right != "]]" {
		t.Errorf("Right delim = %s, want ]]", engine.htmlRenderer.delims.Right)
	}
}

func TestEngine_LoadHTMLGlob(t *testing.T) {
	// Create temp directory with templates
	tmpDir, err := os.MkdirTemp("", "templates")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create template files
	indexContent := `<html><body>{{.Title}}</body></html>`
	err = os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}

	aboutContent := `<html><body>About {{.Name}}</body></html>`
	err = os.WriteFile(filepath.Join(tmpDir, "about.html"), []byte(aboutContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}

	engine := New()
	err = engine.LoadHTMLGlob(filepath.Join(tmpDir, "*.html"))
	if err != nil {
		t.Fatalf("LoadHTMLGlob failed: %v", err)
	}

	if engine.Renderer == nil {
		t.Error("Renderer should be set after LoadHTMLGlob")
	}
}

func TestEngine_LoadHTMLGlob_Recursive(t *testing.T) {
	// Create temp directory with nested templates
	tmpDir, err := os.MkdirTemp("", "templates")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectories
	pagesDir := filepath.Join(tmpDir, "pages")
	layoutsDir := filepath.Join(tmpDir, "layouts")
	os.MkdirAll(pagesDir, 0o755)
	os.MkdirAll(layoutsDir, 0o755)

	// Create template files
	os.WriteFile(filepath.Join(pagesDir, "index.html"), []byte(`<h1>{{.Title}}</h1>`), 0o644)
	os.WriteFile(filepath.Join(pagesDir, "about.html"), []byte(`<h1>About</h1>`), 0o644)
	os.WriteFile(filepath.Join(layoutsDir, "base.html"), []byte(`<html>{{.Content}}</html>`), 0o644)

	engine := New()
	err = engine.LoadHTMLGlob(filepath.Join(tmpDir, "**/*.html"))
	if err != nil {
		t.Fatalf("LoadHTMLGlob recursive failed: %v", err)
	}

	// Check that templates are loaded with relative paths
	tmpl := engine.GetHTMLTemplate()
	if tmpl == nil {
		t.Fatal("Template should not be nil")
	}

	// Should be able to find templates by relative path
	if tmpl.Lookup("pages/index.html") == nil {
		t.Error("Should find pages/index.html template")
	}
	if tmpl.Lookup("layouts/base.html") == nil {
		t.Error("Should find layouts/base.html template")
	}
}

func TestEngine_LoadHTMLFiles(t *testing.T) {
	// Create temp directory with templates
	tmpDir, err := os.MkdirTemp("", "templates")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create template files
	file1 := filepath.Join(tmpDir, "index.html")
	file2 := filepath.Join(tmpDir, "about.html")
	os.WriteFile(file1, []byte(`<h1>{{.Title}}</h1>`), 0o644)
	os.WriteFile(file2, []byte(`<h1>About</h1>`), 0o644)

	engine := New()
	err = engine.LoadHTMLFiles(file1, file2)
	if err != nil {
		t.Fatalf("LoadHTMLFiles failed: %v", err)
	}

	if engine.Renderer == nil {
		t.Error("Renderer should be set after LoadHTMLFiles")
	}
}

func TestEngine_LoadHTMLFiles_NoFiles(t *testing.T) {
	engine := New()
	err := engine.LoadHTMLFiles()
	if err == nil {
		t.Error("LoadHTMLFiles should fail with no files")
	}
}

func TestEngine_SetHTMLTemplate(t *testing.T) {
	engine := New()
	tmpl := template.Must(template.New("test").Parse("<h1>{{.Title}}</h1>"))
	engine.SetHTMLTemplate(tmpl)

	if engine.Renderer == nil {
		t.Error("Renderer should be set after SetHTMLTemplate")
	}
	if engine.GetHTMLTemplate() != tmpl {
		t.Error("GetHTMLTemplate should return the set template")
	}
}

func TestContext_HTML(t *testing.T) {
	// Create temp directory with templates
	tmpDir, err := os.MkdirTemp("", "templates")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create template file
	os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(`<h1>{{.Title}}</h1>`), 0o644)

	engine := New()
	err = engine.LoadHTMLGlob(filepath.Join(tmpDir, "*.html"))
	if err != nil {
		t.Fatalf("LoadHTMLGlob failed: %v", err)
	}

	engine.GET("/", func(c *Context) {
		c.HTML(http.StatusOK, "index.html", H{"Title": "Hello"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %s, want text/html; charset=utf-8", contentType)
	}

	body := w.Body.String()
	if body != "<h1>Hello</h1>" {
		t.Errorf("Body = %s, want <h1>Hello</h1>", body)
	}
}

func TestContext_HTML_NoRenderer(t *testing.T) {
	engine := New()
	engine.GET("/", func(c *Context) {
		err := c.HTML(http.StatusOK, "index.html", nil)
		if err == nil {
			t.Error("HTML should fail when no renderer configured")
		}
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	engine.ServeHTTP(w, req)
}

func TestDefaultFuncMap(t *testing.T) {
	// Test safe function
	if safe, ok := DefaultFuncMap["safe"]; ok {
		result := safe.(func(string) template.HTML)("<script>")
		if string(result) != "<script>" {
			t.Error("safe function should return HTML as-is")
		}
	} else {
		t.Error("DefaultFuncMap should contain safe function")
	}

	// Test upper function
	if upper, ok := DefaultFuncMap["upper"]; ok {
		result := upper.(func(string) string)("hello")
		if result != "HELLO" {
			t.Errorf("upper function = %s, want HELLO", result)
		}
	} else {
		t.Error("DefaultFuncMap should contain upper function")
	}

	// Test lower function
	if lower, ok := DefaultFuncMap["lower"]; ok {
		result := lower.(func(string) string)("HELLO")
		if result != "hello" {
			t.Errorf("lower function = %s, want hello", result)
		}
	} else {
		t.Error("DefaultFuncMap should contain lower function")
	}
}

func TestEngine_CustomDelims(t *testing.T) {
	// Create temp directory with templates
	tmpDir, err := os.MkdirTemp("", "templates")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create template file with custom delimiters
	os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(`<h1>[[.Title]]</h1>`), 0o644)

	engine := New()
	engine.Delims("[[", "]]")
	err = engine.LoadHTMLGlob(filepath.Join(tmpDir, "*.html"))
	if err != nil {
		t.Fatalf("LoadHTMLGlob failed: %v", err)
	}

	engine.GET("/", func(c *Context) {
		c.HTML(http.StatusOK, "index.html", H{"Title": "Custom"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	if body != "<h1>Custom</h1>" {
		t.Errorf("Body = %s, want <h1>Custom</h1>", body)
	}
}

func TestEngine_FuncMap(t *testing.T) {
	// Create temp directory with templates
	tmpDir, err := os.MkdirTemp("", "templates")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create template file using custom function
	os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(`<h1>{{upper .Title}}</h1>`), 0o644)

	engine := New()
	engine.SetFuncMap(template.FuncMap{
		"upper": func(s string) string {
			return "UPPER:" + s
		},
	})
	err = engine.LoadHTMLGlob(filepath.Join(tmpDir, "*.html"))
	if err != nil {
		t.Fatalf("LoadHTMLGlob failed: %v", err)
	}

	engine.GET("/", func(c *Context) {
		c.HTML(http.StatusOK, "index.html", H{"Title": "test"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	if body != "<h1>UPPER:test</h1>" {
		t.Errorf("Body = %s, want <h1>UPPER:test</h1>", body)
	}
}
