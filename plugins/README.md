# Plugin - Go HTTP 插件集合

一个轻量级的 Go 插件集合，提供多种开箱即用的 HTTP 服务功能。所有插件都实现了统一的 `PluginProvider` 接口，可以快速集成到任何 Go Web 项目中。

## 特性

- 🔌 **统一接口设计** - 所有插件实现相同的 `PluginProvider` 接口
- 🚀 **开箱即用** - 无需复杂配置，快速集成
- 📦 **模块化设计** - 按需引入所需插件
- 🎯 **RESTful API** - 标准的 HTTP 接口设计
- 🛠️ **易于扩展** - 简单的插件开发模式

## 插件列表

### 1. Music Plugin - 音乐服务插件

提供网易云音乐数据查询服务。

**功能：**

- 搜索歌曲
- 获取歌曲详情
- 获取歌曲播放链接
- 获取歌词
- 获取专辑信息
- 获取歌手信息
- 获取歌单信息

**API 端点：**

```
GET /music/search?keyword=xxx      # 搜索歌曲
GET /music/song?id=xxx             # 获取歌曲详情
GET /music/song/link?id=xxx        # 获取歌曲播放链接
GET /music/lyric?id=xxx            # 获取歌词
GET /music/album?id=xxx            # 获取专辑信息
GET /music/artist?id=xxx           # 获取歌手信息
GET /music/playlist?id=xxx         # 获取歌单信息
```

### 2. GSM Plugin - 手机规格查询插件

爬取 GSMArena 网站数据，提供手机品牌和型号信息查询服务。

**功能：**

- 获取所有手机品牌列表
- 根据品牌获取设备列表（支持分页）
- 获取设备详细规格信息

**API 端点：**

```
GET /gsm/brands                           # 获取所有品牌
GET /gsm/devices?slug=xxx&page=1          # 获取设备列表
GET /gsm/specification?slug=xxx           # 获取设备规格
```

### 3. AI Plugin - AI 对话服务插件

提供 OpenAI 兼容的对话接口代理服务。

**功能：**

- OpenAI Chat Completions API 代理
- 支持流式响应
- 自定义 API 配置

**API 端点：**

```
POST /ai/chat/completions              # 对话接口
```

### 4. Knife4j Plugin - API 文档插件

提供 Knife4j 风格的 Swagger API 文档界面。

**功能：**

- 美观的 API 文档界面
- 支持 Swagger 2.0 规范
- 在线 API 测试

**访问地址：**

```
GET /doc.html                          # 文档首页
```

### 5. Swagger Plugin - Swagger 文档插件

提供标准的 Swagger UI 文档界面。

**功能：**

- 标准 Swagger UI
- 支持 OpenAPI 规范
- API 在线调试

## 快速开始

### 安装

```bash
go get github.com/ve-weiyi/plugin
```

### 基本使用

```go
package main

import (
    "log"
    "net/http"
    
    "github.com/ve-weiyi/plugin/music"
    "github.com/ve-weiyi/plugin/gsm"
    "github.com/ve-weiyi/plugin/ai"
    "github.com/openai/openai-go/v3/option"
)

func main() {
    // 注册音乐插件
    http.HandleFunc("/music/", music.NewMusicPlugin().Handler("/music/"))
    
    // 注册 GSM 插件
    http.HandleFunc("/gsm/", gsm.NewGsmPlugin().Handler("/gsm/"))
    
    // 注册 AI 插件
    aiPlugin := ai.NewAiPlugin(
        option.WithAPIKey("your-api-key"),
        option.WithBaseURL("https://api.openai.com/v1"),
    )
    http.HandleFunc("/ai/", aiPlugin.Handler("/ai/"))
    
    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### 使用示例

#### Music Plugin 示例

```bash
# 搜索歌曲
curl "http://localhost:8080/music/search?keyword=周杰伦"

# 获取歌曲详情
curl "http://localhost:8080/music/song?id=123456"

# 获取歌曲播放链接
curl "http://localhost:8080/music/song/link?id=123456"
```

#### GSM Plugin 示例

```bash
# 获取所有品牌
curl "http://localhost:8080/gsm/brands"

# 获取三星手机列表
curl "http://localhost:8080/gsm/devices?slug=samsung-phones-9&page=1"

# 获取设备规格
curl "http://localhost:8080/gsm/specification?slug=samsung_galaxy_s24-12345"
```

#### AI Plugin 示例

```bash
# 发送对话请求
curl -X POST "http://localhost:8080/ai/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

## 插件接口

所有插件都实现了 `PluginProvider` 接口：

```go
type PluginProvider interface {
    Handler(prefix string) http.HandlerFunc
}
```

### 参数说明

- `prefix`: URL 路径前缀，用于路由匹配

### 返回值

- `http.HandlerFunc`: 标准的 HTTP 处理函数

## 开发自定义插件

### 1. 创建插件结构

```go
package myplugin

import (
    "encoding/json"
    "net/http"
    "strings"
)

type MyPlugin struct {
    // 插件配置字段
}

func NewMyPlugin() *MyPlugin {
    return &MyPlugin{}
}
```

### 2. 实现 Handler 方法

```go
func (p *MyPlugin) Handler(prefix string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 解析路径
        path := strings.TrimPrefix(r.URL.Path, prefix)
        path = strings.TrimPrefix(path, "/")
        
        var data interface{}
        var err error
        
        // 路由处理
        switch path {
        case "endpoint1":
            data, err = p.handleEndpoint1(r)
        case "endpoint2":
            data, err = p.handleEndpoint2(r)
        default:
            http.NotFound(w, r)
            return
        }
        
        // 错误处理
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        
        // 返回 JSON 响应
        body, _ := json.Marshal(data)
        w.Header().Set("Content-Type", "application/json")
        w.Write(body)
    }
}
```

### 3. 注册插件

```go
http.HandleFunc("/myplugin/", myplugin.NewMyPlugin().Handler("/myplugin/"))
```

## 项目结构

```
plugin/
├── ai/                    # AI 对话服务插件
│   ├── chat/             # OpenAI 代理实现
│   └── ai.go             # 插件入口
├── gsm/                   # 手机规格查询插件
│   ├── service/          # 业务服务层
│   │   ├── brand/        # 品牌服务
│   │   ├── device/       # 设备服务
│   │   └── specification/ # 规格服务
│   ├── util/             # 工具类
│   └── gsm.go            # 插件入口
├── music/                 # 音乐服务插件
│   ├── netease/          # 网易云音乐实现
│   └── music.go          # 插件入口
├── knife4j/               # Knife4j 文档插件
│   ├── static/           # 静态资源
│   └── knife4j.go        # 插件入口
├── swagger/               # Swagger 文档插件
│   └── swagger.go        # 插件入口
├── plugin.go              # 插件接口定义
└── README.md              # 项目文档
```

## 依赖项

主要依赖：

- `github.com/PuerkitoBio/goquery` - HTML 解析
- `github.com/openai/openai-go/v3` - OpenAI SDK
- `github.com/swaggo/http-swagger` - Swagger 支持
- `gorm.io/gorm` - ORM 框架

完整依赖列表请查看 [go.mod](go.mod)

## 技术栈

- Go 1.25+
- HTTP 标准库
- RESTful API 设计
- JSON 数据格式

## 最佳实践

1. **错误处理**: 所有插件都应该妥善处理错误并返回适当的 HTTP 状态码
2. **日志记录**: 建议在关键操作处添加日志记录
3. **参数验证**: 在处理请求前验证必需参数
4. **响应格式**: 统一使用 JSON 格式返回数据
5. **路径处理**: 使用 `strings.TrimPrefix` 正确处理路径前缀

## 注意事项

- GSM Plugin 依赖外部网站数据，请遵守相关网站的使用条款
- AI Plugin 需要配置有效的 OpenAI API Key
- Music Plugin 仅供学习使用，请勿用于商业用途
- 建议在生产环境中添加适当的限流和缓存机制

## 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 联系方式

- 项目地址: https://github.com/ve-weiyi/plugin
- 问题反馈: https://github.com/ve-weiyi/plugin/issues

## 更新日志

### v1.0.0 (2024)

- ✨ 初始版本发布
- ✨ 实现 Music Plugin
- ✨ 实现 GSM Plugin
- ✨ 实现 AI Plugin
- ✨ 实现 Knife4j Plugin
- ✨ 实现 Swagger Plugin
- 📝 完善项目文档

---

**Made with ❤️ by ve-weiyi**
