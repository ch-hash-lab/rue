package rue

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Delims represents template delimiters
type Delims struct {
	Left  string
	Right string
}

// HTMLRenderer is a template renderer using html/template
type HTMLRenderer struct {
	templates *template.Template
	funcMap   template.FuncMap
	delims    Delims
}

// NewHTMLRenderer creates a new HTMLRenderer
func NewHTMLRenderer() *HTMLRenderer {
	return &HTMLRenderer{
		delims: Delims{Left: "{{", Right: "}}"},
	}
}

// Render implements the Renderer interface
func (r *HTMLRenderer) Render(w io.Writer, name string, data any) error {
	if r.templates == nil {
		return fmt.Errorf("no templates loaded")
	}
	return r.templates.ExecuteTemplate(w, name, data)
}

// SetFuncMap sets the template function map for the engine
func (e *Engine) SetFuncMap(funcMap template.FuncMap) {
	if e.htmlRenderer == nil {
		e.htmlRenderer = NewHTMLRenderer()
	}
	e.htmlRenderer.funcMap = funcMap
}

// Delims sets the template delimiters for the engine
func (e *Engine) Delims(left, right string) *Engine {
	if e.htmlRenderer == nil {
		e.htmlRenderer = NewHTMLRenderer()
	}
	e.htmlRenderer.delims = Delims{Left: left, Right: right}
	return e
}

// LoadHTMLGlob loads HTML templates from a glob pattern
// Supports multi-level directories with "**" pattern
// Examples:
//   - "templates/*.html" - single level
//   - "templates/**/*.html" - multi-level
//   - "templates/**/*" - all files in all subdirectories
func (e *Engine) LoadHTMLGlob(pattern string) error {
	if e.htmlRenderer == nil {
		e.htmlRenderer = NewHTMLRenderer()
	}

	// Check if pattern contains "**" for recursive matching
	if strings.Contains(pattern, "**") {
		return e.loadHTMLGlobRecursive(pattern)
	}

	// Standard glob pattern
	tmpl := template.New("").Delims(e.htmlRenderer.delims.Left, e.htmlRenderer.delims.Right)
	if e.htmlRenderer.funcMap != nil {
		tmpl = tmpl.Funcs(e.htmlRenderer.funcMap)
	}

	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no files matched pattern: %s", pattern)
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		name := filepath.Base(file)
		_, err = tmpl.New(name).Parse(string(content))
		if err != nil {
			return err
		}
	}

	e.htmlRenderer.templates = tmpl
	e.Renderer = e.htmlRenderer
	return nil
}

// loadHTMLGlobRecursive loads templates recursively from directories
func (e *Engine) loadHTMLGlobRecursive(pattern string) error {
	// Split pattern into base directory and file pattern
	// e.g., "templates/**/*.html" -> base="templates", filePattern="*.html"
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return fmt.Errorf("invalid recursive pattern: %s", pattern)
	}

	baseDir := strings.TrimSuffix(parts[0], string(os.PathSeparator))
	if baseDir == "" {
		baseDir = "."
	}

	filePattern := strings.TrimPrefix(parts[1], string(os.PathSeparator))
	if filePattern == "" {
		filePattern = "*"
	}

	tmpl := template.New("").Delims(e.htmlRenderer.delims.Left, e.htmlRenderer.delims.Right)
	if e.htmlRenderer.funcMap != nil {
		tmpl = tmpl.Funcs(e.htmlRenderer.funcMap)
	}

	var files []string
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Match file pattern
		matched, err := filepath.Match(filePattern, filepath.Base(path))
		if err != nil {
			return err
		}
		if matched {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no files matched pattern: %s", pattern)
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		// Use relative path from base directory as template name
		// This allows templates like "pages/index.html", "layouts/base.html"
		relPath, err := filepath.Rel(baseDir, file)
		if err != nil {
			relPath = filepath.Base(file)
		}
		// Normalize path separators for template names
		name := strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

		_, err = tmpl.New(name).Parse(string(content))
		if err != nil {
			return fmt.Errorf("error parsing template %s: %w", name, err)
		}
	}

	e.htmlRenderer.templates = tmpl
	e.Renderer = e.htmlRenderer
	return nil
}

// LoadHTMLFiles loads specific HTML template files
func (e *Engine) LoadHTMLFiles(files ...string) error {
	if e.htmlRenderer == nil {
		e.htmlRenderer = NewHTMLRenderer()
	}

	if len(files) == 0 {
		return fmt.Errorf("no files specified")
	}

	tmpl := template.New("").Delims(e.htmlRenderer.delims.Left, e.htmlRenderer.delims.Right)
	if e.htmlRenderer.funcMap != nil {
		tmpl = tmpl.Funcs(e.htmlRenderer.funcMap)
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		name := filepath.Base(file)
		_, err = tmpl.New(name).Parse(string(content))
		if err != nil {
			return err
		}
	}

	e.htmlRenderer.templates = tmpl
	e.Renderer = e.htmlRenderer
	return nil
}

// LoadHTMLFS loads HTML templates from an embed.FS or other fs.FS
func (e *Engine) LoadHTMLFS(fsys fs.FS, patterns ...string) error {
	if e.htmlRenderer == nil {
		e.htmlRenderer = NewHTMLRenderer()
	}

	tmpl := template.New("").Delims(e.htmlRenderer.delims.Left, e.htmlRenderer.delims.Right)
	if e.htmlRenderer.funcMap != nil {
		tmpl = tmpl.Funcs(e.htmlRenderer.funcMap)
	}

	var files []string
	for _, pattern := range patterns {
		matches, err := fs.Glob(fsys, pattern)
		if err != nil {
			return err
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files matched patterns")
	}

	for _, file := range files {
		content, err := fs.ReadFile(fsys, file)
		if err != nil {
			return err
		}

		name := filepath.Base(file)
		_, err = tmpl.New(name).Parse(string(content))
		if err != nil {
			return err
		}
	}

	e.htmlRenderer.templates = tmpl
	e.Renderer = e.htmlRenderer
	return nil
}

// LoadHTMLFSRecursive loads HTML templates recursively from an fs.FS
func (e *Engine) LoadHTMLFSRecursive(fsys fs.FS, root string, filePattern string) error {
	if e.htmlRenderer == nil {
		e.htmlRenderer = NewHTMLRenderer()
	}

	tmpl := template.New("").Delims(e.htmlRenderer.delims.Left, e.htmlRenderer.delims.Right)
	if e.htmlRenderer.funcMap != nil {
		tmpl = tmpl.Funcs(e.htmlRenderer.funcMap)
	}

	var files []string
	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		matched, err := filepath.Match(filePattern, filepath.Base(path))
		if err != nil {
			return err
		}
		if matched {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no files matched pattern: %s in %s", filePattern, root)
	}

	for _, file := range files {
		content, err := fs.ReadFile(fsys, file)
		if err != nil {
			return err
		}

		// Use relative path from root as template name
		relPath, err := filepath.Rel(root, file)
		if err != nil {
			relPath = filepath.Base(file)
		}
		name := strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

		_, err = tmpl.New(name).Parse(string(content))
		if err != nil {
			return fmt.Errorf("error parsing template %s: %w", name, err)
		}
	}

	e.htmlRenderer.templates = tmpl
	e.Renderer = e.htmlRenderer
	return nil
}

// SetHTMLTemplate sets a custom template
func (e *Engine) SetHTMLTemplate(tmpl *template.Template) {
	if e.htmlRenderer == nil {
		e.htmlRenderer = NewHTMLRenderer()
	}
	e.htmlRenderer.templates = tmpl
	e.Renderer = e.htmlRenderer
}

// GetHTMLTemplate returns the current HTML template
func (e *Engine) GetHTMLTemplate() *template.Template {
	if e.htmlRenderer == nil {
		return nil
	}
	return e.htmlRenderer.templates
}

// Common template functions
var DefaultFuncMap = template.FuncMap{
	"safe": func(s string) template.HTML {
		return template.HTML(s)
	},
	"safeJS": func(s string) template.JS {
		return template.JS(s)
	},
	"safeCSS": func(s string) template.CSS {
		return template.CSS(s)
	},
	"safeURL": func(s string) template.URL {
		return template.URL(s)
	},
	"upper":     strings.ToUpper,
	"lower":     strings.ToLower,
	"title":     strings.Title,
	"trim":      strings.TrimSpace,
	"join":      strings.Join,
	"split":     strings.Split,
	"contains":  strings.Contains,
	"hasPrefix": strings.HasPrefix,
	"hasSuffix": strings.HasSuffix,
	"replace": func(s, old, new string) string {
		return strings.ReplaceAll(s, old, new)
	},
}
