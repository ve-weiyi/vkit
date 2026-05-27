# vkit

Go 通用工具包，提供可复用的技术组件、工具函数和业务插件。

## 安装

```bash
go get github.com/ve-weiyi/vkit
```

## 目录结构

```
.
├── adapter/   # 外部服务适配器
├── x/         # 标准库扩展
├── plugins/   # HTTP 插件
└── gen/       # 代码生成
```

## adapter/ — 外部服务适配器

封装第三方 SDK 和外部 API，提供统一接口。适用于数据库、消息队列、支付、存储等场景。

| 包 | 说明 |
|---|---|
| `adapter/excelx` | Excel 导入导出 (excelize) |
| `adapter/gormx` | GORM 扩展（日志、查询构建器、数据库内省） |
| `adapter/ipx` | IP 地理位置查询 (ip-api / 百度) |
| `adapter/logz` | 日志系统 (zap + lumberjack) |
| `adapter/mail` | SMTP 邮件发送 |
| `adapter/mqx` | 消息队列统一抽象（Kafka / RabbitMQ / Redis Stream） |
| `adapter/nacosx` | Nacos 配置中心 |
| `adapter/oauthx` | OAuth2 第三方登录（QQ、GitHub、Gitee、微博、飞书） |
| `adapter/payx` | 支付网关（支付宝、微信、Stripe） |
| `adapter/smsx` | 短信服务（阿里云、腾讯云） |
| `adapter/storagex` | 对象存储（阿里云 OSS、腾讯云 COS、七牛云 Kodo、本地） |
| `adapter/storex` | KV 存储抽象（Redis、内存），含验证码/Token 子包 |

## x/ — 标准库扩展

标准库的轻量补充，几乎无外部依赖，提供纯函数式工具方法。

| 包 | 说明 |
|---|---|
| `x/colorx` | 终端 ANSI 颜色输出 |
| `x/cryptox` | 加密工具（AES、RSA、ECDSA、bcrypt、MD5、SHA-256） |
| `x/filex` | 文件系统操作（读写、复制、压缩） |
| `x/httpx` | HTTP 客户端构建器 |
| `x/jsonconv` | JSON 序列化与类型转换 |
| `x/jwtx` | JWT 令牌生成与解析 |
| `x/maskx` | 数据脱敏（手机号、邮箱） |
| `x/moneyx` | 金额类型（分/元转换、算术运算） |
| `x/patternx` | 常用正则校验（邮箱、手机号、版本号） |
| `x/randomx` | 随机值生成（UUID、验证码、订单号） |
| `x/slicex` | 泛型切片操作（去重、反转、Join） |
| `x/systemx` | 系统监控（CPU、内存、磁盘） |
| `x/tempx` | 文本模板渲染 |

## plugins/ — HTTP 插件

实现统一 `PluginProvider` 接口的 HTTP 服务插件。

| 包 | 说明 |
|---|---|
| `plugins/ai` | OpenAI API 代理（支持流式响应） |
| `plugins/gsm` | GSM Arena 手机参数抓取 |
| `plugins/knife4j` | Knife4j Swagger 文档 UI |
| `plugins/music` | 网易云音乐 API 代理 |
| `plugins/swagger` | Swagger 文档 UI |

## gen/ — 代码生成

| 包 | 说明 |
|---|---|
| `gen/astx` | 基于 `go/ast` 的代码注入 |
| `gen/dstx` | 基于 `dst` 的代码注入（保留注释） |
| `gen/gormgen` | GORM 模型代码生成 |
| `gen/tmplx` | 模板引擎代码生成 |

## 设计原则

- **adapter**：对接外部 SDK 或网络服务，有第三方依赖
- **x**：标准库的扩展补充，尽量零依赖
- **plugins**：统一接口的 HTTP 插件，可独立挂载路由
- **gen**：代码生成工具，输出 `.go` 文件

## License

MIT
