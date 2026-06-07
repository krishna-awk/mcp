package scanner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Page struct {
	Route    string `json:"route"`
	Title    string `json:"title,omitempty"`
	FilePath string `json:"filePath"`
	Kind     string `json:"kind"`
}

type API struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	FilePath string `json:"filePath"`
	Source   string `json:"source"`
}

type Form struct {
	Action   string   `json:"action"`
	Method   string   `json:"method"`
	Fields   []string `json:"fields"`
	FilePath string   `json:"filePath"`
	IsAuth   bool     `json:"isAuth,omitempty"`
	AuthKind string   `json:"authKind,omitempty"`
}

type AuthInfo struct {
	LoginForms   []Form `json:"loginForms"`
	HasOAuth     bool   `json:"hasOAuth"`
	HasJWT       bool   `json:"hasJWT"`
	HasAPIKey    bool   `json:"hasAPIKey"`
	HasSession   bool   `json:"hasSession"`
	OAuthHints   []string `json:"oauthHints,omitempty"`
	EnvVars      []string `json:"envVars"`
}

type AnalyzeResult struct {
	ProjectPath string   `json:"projectPath"`
	Framework   string   `json:"framework"`
	Pages       []Page   `json:"pages"`
	APIs        []API    `json:"apis"`
	Forms       []Form   `json:"forms"`
	Auth        AuthInfo `json:"auth"`
}

var (
	expressGet    = regexp.MustCompile(`(?m)\b(app|router)\.(get|post|put|patch|delete|head|options)\s*\(\s*['"]([^'"]+)['"]`)
	expressAction = regexp.MustCompile(`(?m)\b(app|router)\.(get|post|put|patch|delete)\s*\(\s*['"][^'"]*['"]\s*,\s*['"]?([A-Za-z_][\w:]*)['"]?`)
	djangoPath    = regexp.MustCompile(`(?m)path\s*\(\s*['"]([^'"]+)['"]`)
	djangoInclude = regexp.MustCompile(`(?m)include\s*\(\s*['"]([^'"]+)['"]`)
	flaskRoute    = regexp.MustCompile(`(?m)@app\.route\s*\(\s*['"]([^'"]+)['"](?:\s*,\s*methods\s*=\s*\[([^\]]+)\])?`)
	fastapiRoute  = regexp.MustCompile(`(?m)@app\.(get|post|put|patch|delete)\s*\(\s*['"]([^'"]+)['"]`)
	honoRoute     = regexp.MustCompile(`(?m)\bapp\.(get|post|put|patch|delete)\s*\(\s*['"]([^'"]+)['"]`)
	formStartRe   = regexp.MustCompile(`(?is)<form\b`)
	attrRe        = regexp.MustCompile(`(?i)\b([a-zA-Z_][\w-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	inputRe       = regexp.MustCompile(`(?i)<input\b[^>]*?\bname\s*=\s*["']([^"']+)["']`)
	textareaRe    = regexp.MustCompile(`(?i)<textarea\b[^>]*?\bname\s*=\s*["']([^"']+)["']`)
	selectRe      = regexp.MustCompile(`(?i)<select\b[^>]*?\bname\s*=\s*["']([^"']+)["']`)
	passwordInput = regexp.MustCompile(`(?i)<input\b[^>]*?\btype\s*=\s*["']password["']`)
	oauthRe       = regexp.MustCompile(`(?i)(google|github|facebook|twitter|apple|okta|auth0|azure[_-]?ad|keycloak)\s*(\.|_)?\s*(oauth|sign[_-]?in|login|sso)`)
	jwtRe         = regexp.MustCompile(`(?i)(jsonwebtoken|jwt\.verify|jwt\.sign|Bearer\s+[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.)`)
	apikeyRe      = regexp.MustCompile(`(?i)(api[_-]?key|x-api-key|x-api-token)`)
	sessionRe     = regexp.MustCompile(`(?i)(express-session|req\.session|flask\.session|django\.contrib\.sessions)`)
)

func Analyze(projectPath string) (*AnalyzeResult, error) {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", abs)
	}

	res := &AnalyzeResult{
		ProjectPath: abs,
		Framework:   detectFramework(abs),
		Pages:       []Page{},
		APIs:        []API{},
		Forms:       []Form{},
		Auth:        AuthInfo{EnvVars: []string{"QA_BASE_URL", "QA_USER", "QA_PASSWORD", "QA_API_TOKEN"}},
	}

	if err := filepath.WalkDir(abs, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == ".next" || name == "dist" || name == "build" || name == "__pycache__" || name == "venv" || name == ".venv" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(abs, p)
		base := strings.ToLower(d.Name())
		switch {
		case strings.HasPrefix(base, "openapi") || strings.HasPrefix(base, "swagger"):
			if apis, src := parseOpenAPI(p); len(apis) > 0 {
				for i := range apis {
					apis[i].FilePath = rel
					apis[i].Source = src
				}
				res.APIs = append(res.APIs, apis...)
			}
		case base == "page.tsx" || base == "page.jsx" || base == "page.js" || base == "page.ts":
			res.Pages = append(res.Pages, Page{Route: nextAppRouterRoute(rel), FilePath: rel, Kind: "next-app-page"})
		case base == "route.ts" || base == "route.js":
			res.APIs = append(res.APIs, API{Method: "ANY", Path: nextAppRouterRoute(rel), FilePath: rel, Source: "next-app-route"})
		case base == "urls.py":
			if apis, _ := parseDjangoURLs(p, abs); len(apis) > 0 {
				for i := range apis {
					apis[i].FilePath = rel
					apis[i].Source = "django"
				}
				res.APIs = append(res.APIs, apis...)
			}
		case strings.HasSuffix(base, ".py"):
			if apis := parsePythonRoutes(p); len(apis) > 0 {
				for i := range apis {
					apis[i].FilePath = rel
					apis[i].Source = "python"
				}
				res.APIs = append(res.APIs, apis...)
			}
		case strings.HasSuffix(base, ".ts") || strings.HasSuffix(base, ".tsx") || strings.HasSuffix(base, ".js") || strings.HasSuffix(base, ".jsx") || strings.HasSuffix(base, ".mjs") || strings.HasSuffix(base, ".cjs"):
			if apis := parseNodeRoutes(p); len(apis) > 0 {
				for i := range apis {
					apis[i].FilePath = rel
					apis[i].Source = "node"
				}
				res.APIs = append(res.APIs, apis...)
			}
		case strings.HasSuffix(base, ".html") || strings.HasSuffix(base, ".htm") || strings.HasSuffix(base, ".vue") || strings.HasSuffix(base, ".svelte") || strings.HasSuffix(base, ".jsx") || strings.HasSuffix(base, ".tsx"):
			if forms := parseForms(p); len(forms) > 0 {
				for i := range forms {
					forms[i].FilePath = rel
					tagFormsAsAuth(&forms[i], res)
				}
				res.Forms = append(res.Forms, forms...)
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}

	detectAuthPatterns(abs, res)
	return res, nil
}

func tagFormsAsAuth(f *Form, res *AnalyzeResult) {
	hasPassword := false
	hasUser := false
	for _, field := range f.Fields {
		low := strings.ToLower(field)
		if low == "password" || low == "passwd" || low == "pwd" || strings.Contains(low, "password") {
			hasPassword = true
		}
		if low == "email" || low == "username" || low == "user" || low == "login" || low == "phone" || strings.Contains(low, "email") {
			hasUser = true
		}
	}
	if hasPassword && hasUser {
		f.IsAuth = true
		f.AuthKind = "password"
		res.Auth.LoginForms = append(res.Auth.LoginForms, *f)
	}
}

func detectAuthPatterns(root string, res *AnalyzeResult) {
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				name := d.Name()
				if name == "node_modules" || name == ".git" || name == "dist" || name == "build" || name == "venv" || name == "__pycache__" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		src := string(data)
		if oauthRe.MatchString(src) {
			res.Auth.HasOAuth = true
			for _, m := range oauthRe.FindAllString(src, -1) {
				provider := strings.SplitN(m, ".", 2)[0]
				provider = strings.SplitN(provider, "_", 2)[0]
				provider = strings.TrimSpace(provider)
				if provider != "" {
					res.Auth.OAuthHints = append(res.Auth.OAuthHints, provider)
				}
			}
		}
		if jwtRe.MatchString(src) {
			res.Auth.HasJWT = true
		}
		if apikeyRe.MatchString(src) {
			res.Auth.HasAPIKey = true
		}
		if sessionRe.MatchString(src) {
			res.Auth.HasSession = true
		}
		return nil
	})
}

func detectFramework(root string) string {
	if fileExists(filepath.Join(root, "next.config.js")) || fileExists(filepath.Join(root, "next.config.ts")) || fileExists(filepath.Join(root, "next.config.mjs")) {
		return "nextjs"
	}
	if fileExists(filepath.Join(root, "manage.py")) {
		return "django"
	}
	if pkg, err := readJSON(filepath.Join(root, "package.json")); err == nil {
		deps := map[string]bool{}
		if v, ok := pkg["dependencies"].(map[string]any); ok {
			for k := range v {
				deps[k] = true
			}
		}
		if v, ok := pkg["devDependencies"].(map[string]any); ok {
			for k := range v {
				deps[k] = true
			}
		}
		switch {
		case deps["express"], deps["@nestjs/core"], deps["fastify"], deps["hono"], deps["koa"]:
			return "node"
		case deps["react-router-dom"], deps["@remix-run/react"], deps["vite"]:
			return "react"
		}
		return "node"
	}
	if fileExists(filepath.Join(root, "requirements.txt")) || fileExists(filepath.Join(root, "pyproject.toml")) {
		return "python"
	}
	return "unknown"
}

func nextAppRouterRoute(rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	end := len(parts)
	for i, p := range parts {
		if p == "page.tsx" || p == "page.jsx" || p == "page.js" || p == "page.ts" || p == "route.ts" || p == "route.js" {
			end = i
			break
		}
	}
	routeParts := parts[:end]
	if len(routeParts) == 0 {
		return "/"
	}
	if routeParts[0] == "src" && len(routeParts) > 1 && routeParts[1] == "app" {
		routeParts = routeParts[2:]
	} else if routeParts[0] == "app" {
		routeParts = routeParts[1:]
	} else if routeParts[0] == "pages" {
		routeParts = routeParts[1:]
	}
	if len(routeParts) == 0 {
		return "/"
	}
	for i, p := range routeParts {
		if strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]") {
			routeParts[i] = ":" + strings.TrimSuffix(strings.TrimPrefix(p, "["), "]")
		} else if strings.HasPrefix(p, "[[") && strings.HasSuffix(p, "]]") {
			routeParts[i] = "*" + strings.TrimSuffix(strings.TrimPrefix(p, "[["), "]]")
		}
	}
	return "/" + strings.Join(routeParts, "/")
}

func parseNodeRoutes(path string) []API {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	src := string(data)
	out := []API{}
	matches := expressGet.FindAllStringSubmatch(src, -1)
	for _, m := range matches {
		out = append(out, API{Method: strings.ToUpper(m[2]), Path: m[3]})
	}
	matches = honoRoute.FindAllStringSubmatch(src, -1)
	for _, m := range matches {
		out = append(out, API{Method: strings.ToUpper(m[1]), Path: m[2]})
	}
	return out
}

func parsePythonRoutes(path string) []API {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	src := string(data)
	out := []API{}
	for _, m := range flaskRoute.FindAllStringSubmatch(src, -1) {
		if m[2] != "" {
			methods := strings.Split(strings.NewReplacer("'", "", `"`, "", " ", "").Replace(m[2]), ",")
			for _, method := range methods {
				if method != "" {
					out = append(out, API{Method: strings.ToUpper(method), Path: m[1]})
				}
			}
		} else {
			out = append(out, API{Method: "GET", Path: m[1]})
		}
	}
	for _, m := range fastapiRoute.FindAllStringSubmatch(src, -1) {
		out = append(out, API{Method: strings.ToUpper(m[1]), Path: m[2]})
	}
	return out
}

func parseDjangoURLs(path, root string) ([]API, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ""
	}
	src := string(data)
	out := []API{}
	for _, m := range djangoPath.FindAllStringSubmatch(src, -1) {
		out = append(out, API{Method: "ANY", Path: m[1]})
	}
	return out, "django-urls"
}

func parseForms(path string) []Form {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	src := string(data)
	out := []Form{}
	idx := 0
	for {
		loc := formStartRe.FindStringIndex(src[idx:])
		if loc == nil {
			break
		}
		start := idx + loc[0]
		closeTag := strings.Index(src[start:], ">")
		if closeTag == -1 {
			break
		}
		tagEnd := start + closeTag
		header := src[start : tagEnd]
		attrs := map[string]string{}
		for _, m := range attrRe.FindAllStringSubmatch(header, -1) {
			name := strings.ToLower(m[1])
			val := m[2]
			if val == "" {
				val = m[3]
			}
			if val == "" {
				val = m[4]
			}
			attrs[name] = val
		}
		action := attrs["action"]
		method := strings.ToUpper(attrs["method"])
		if method == "" {
			method = "GET"
		}
		bodyEnd := strings.Index(strings.ToLower(src[tagEnd:]), "</form>")
		var body string
		if bodyEnd == -1 {
			body = src[tagEnd:]
		} else {
			body = src[tagEnd : tagEnd+bodyEnd]
		}
		fields := []string{}
		for _, m := range inputRe.FindAllStringSubmatch(body, -1) {
			fields = append(fields, m[1])
		}
		for _, m := range textareaRe.FindAllStringSubmatch(body, -1) {
			fields = append(fields, m[1])
		}
		for _, m := range selectRe.FindAllStringSubmatch(body, -1) {
			fields = append(fields, m[1])
		}
		out = append(out, Form{Action: action, Method: method, Fields: fields})
		idx = tagEnd + 1
		if bodyEnd == -1 {
			break
		}
	}
	return out
}

func parseOpenAPI(path string) ([]API, string) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ""
	}
	defer f.Close()
	dec := json.NewDecoder(bufio.NewReader(f))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, ""
	}
	paths, _ := doc["paths"].(map[string]any)
	out := []API{}
	for route, ops := range paths {
		opsMap, _ := ops.(map[string]any)
		for method, op := range opsMap {
			method = strings.ToLower(method)
			if method == "parameters" || method == "summary" || method == "description" {
				continue
			}
			title := ""
			if m, ok := op.(map[string]any); ok {
				if s, ok := m["summary"].(string); ok {
					title = s
				} else if s, ok := m["operationId"].(string); ok {
					title = s
				}
			}
			_ = title
			out = append(out, API{Method: strings.ToUpper(method), Path: route, Source: "openapi"})
		}
	}
	return out, "openapi"
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func readJSON(p string) (map[string]any, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}
