// 演示程序：标准库 text/template 的用法，go run ./cmd/template 直接运行。
//
// 依次演示四件事：
//  1. 字符串模板 + 结构体数据（template.New().Parse）
//  2. 模板文件（embed + template.ParseFS），含 {{define}}/{{template}} 跨文件复用
//  3. 自定义函数（Funcs）—— 除标准库包装外，还有一个手写的 indent
//  4. 渲染结果落盘到 ./runtime/_template（目录名以 _ 开头，go 工具链会跳过它）
//
// 模板也可以放在磁盘上用 template.ParseFiles / ParseGlob 加载；这里用 embed
// 是为了让模板不依赖工作目录（产物路径仍是相对当前目录的）。
//
// 两条约定：
//   - 写死的常量模板用 template.Must（解析失败只可能是程序 bug，启动时就该炸）；
//     运行时才知道的模板必须显式接 err。
//   - 生成 Go 代码时不要把模板里的空白抠到完美 —— 模板只写内容，缩进交给 gofmt
//     （见 formatGo）。{{- -}} 一多，模板就没法读了。
package main

import (
	"embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

//go:embed tpl/*.tpl
var tplFS embed.FS

// outDir 以 _ 开头：go 工具链会忽略下划线/点开头的目录，
// 否则产出的是 .go 文件，会被 go build ./... 当成包去编译而报错
// （本仓 adapter/mqx/*/_examples 用的是同一招）。
const outDir = "./runtime/_template"

// 字符串模板是写死的常量，解析失败只可能是程序 bug，用 Must 在启动时暴露。
const commitTpl = `{{ .Title }}
{{ indent "  " .Body }}
  —— {{ upper .Author }} 于 {{ .At.Format "2006-01-02" }} 提交{{ if .Tags }} [{{ join .Tags ", " }}]{{ end }}`

type method struct {
	Name    string
	HTTP    string
	Path    string
	Comment string
	Params  []string
}

type service struct {
	Name     string
	Version  string
	BasePath string
	Methods  []method
}

func main() {
	svc := service{
		Name:     "BlogService",
		Version:  "v1",
		BasePath: "/blog",
		Methods: []method{
			{Name: "ListPosts", HTTP: "GET", Path: "/posts", Comment: "列出文章", Params: []string{"page", "size"}},
			{Name: "GetPost", HTTP: "GET", Path: "/posts/:id", Comment: "文章详情"},
			{Name: "CreatePost", HTTP: "POST", Path: "/posts", Comment: "新建文章"},
		},
	}

	section("1. 字符串模板 + 结构体数据（含手写函数 indent）")
	renderStringTemplate()

	section("2+3. 模板文件 + 自定义函数 + 跨文件子模板")
	rendered := renderFileTemplate(svc)

	section("4. 渲染结果落盘")
	name := "router_gen.go"
	path, err := writeOutput(name, rendered)
	if err != nil {
		fmt.Println("写文件失败:", err)
		os.Exit(1)
	}
	fmt.Println("已写入:", path)
}

// renderStringTemplate 演示最小用法：解析字符串模板 + 执行，数据可以是结构体。
func renderStringTemplate() {
	tpl := template.Must(template.New("commit").Funcs(funcMap()).Parse(commitTpl))

	data := struct {
		Title  string
		Body   string
		Author string
		At     time.Time
		Tags   []string
	}{
		Title:  "feat: 支持文章草稿",
		Body:   "草稿只有作者本人可见，\n发布后才进入公开列表。",
		Author: "weiyi",
		At:     time.Date(2026, 9, 21, 10, 30, 0, 0, time.UTC),
		Tags:   []string{"blog", "draft"},
	}

	var buf strings.Builder
	// 执行结果取决于数据，必须接 err —— 例如数据缺字段、函数入参类型不符
	if err := tpl.Execute(&buf, data); err != nil {
		fmt.Println("渲染失败:", err)
		os.Exit(1)
	}

	fmt.Println(buf.String())
}

// renderFileTemplate 演示从模板文件加载：ParseFS 把 tpl/ 下所有文件读进同一个模板集合，
// {{define}} 定义的子模板在集合内共享，因此 router.tpl 可以直接 template 调用
// doc.tpl 里定义的 file_header / method_line。
func renderFileTemplate(svc service) string {
	tpl, err := template.New("").Funcs(funcMap()).ParseFS(tplFS, "tpl/*.tpl")
	if err != nil {
		fmt.Println("解析模板失败:", err)
		os.Exit(1)
	}

	// 执行时要按文件名指定入口：ParseFS 之后模板名是文件名，不是 "router"
	var buf strings.Builder
	if err := tpl.ExecuteTemplate(&buf, "router.tpl", svc); err != nil {
		fmt.Println("渲染失败:", err)
		os.Exit(1)
	}

	formatted, err := formatGo(buf.String())
	if err != nil {
		fmt.Println("格式化失败:", err)
		os.Exit(1)
	}

	fmt.Print(formatted)
	return formatted
}

// formatGo 用 gofmt 规整模板产出的 Go 代码。
// 模板里只写内容、不抠缩进：{{- -}} 用多了模板就没法读，缩进与空行交给这里统一处理。
func formatGo(src string) (string, error) {
	out, err := format.Source([]byte(src))
	if err != nil {
		return "", fmt.Errorf("模板产出的不是合法 Go 代码: %w", err)
	}
	return string(out), nil
}

// funcMap 注册模板函数：标准库函数直接挂，业务逻辑自己写。
func funcMap() template.FuncMap {
	return template.FuncMap{
		"upper":  strings.ToUpper,
		"lower":  strings.ToLower,
		"join":   strings.Join,
		"indent": indent,
	}
}

// indent 给多行文本的每一行加前缀，生成代码时用来对齐。
func indent(prefix, text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func writeOutput(name, content string) (string, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, name)
	if err := os.WriteFile(path, []byte(content), 0640); err != nil {
		return "", err
	}
	return path, nil
}

func section(title string) {
	fmt.Printf("\n===== %s =====\n", title)
}
