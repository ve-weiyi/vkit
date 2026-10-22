{{ template "file_header" . }}
// Register{{ .Name }}Routes 注册 {{ lower .Name }} 模块路由。
func Register{{ .Name }}Routes(r *gin.Engine) {
	g := r.Group("{{ .BasePath }}")
{{- range .Methods }}
{{ template "method_line" . }}{{- end }}
}
