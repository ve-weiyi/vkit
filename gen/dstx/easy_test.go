package dstx

import (
	"os"
	"path/filepath"
	"testing"
)

// fixturePath 把夹具拷到临时目录，避免注入直接改写源码树；夹具缺失则跳过。
func fixturePath(t *testing.T) string {
	t.Helper()

	const src = "./test/test.go"
	data, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("跳过：夹具 %s 不存在 (%v)", src, err)
	}

	dst := filepath.Join(t.TempDir(), "test.go")
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatalf("写入临时夹具: %v", err)
	}
	return dst
}

func TestInject(t *testing.T) {
	inject := AstInjectMeta{
		FilePath: fixturePath(t),
		FuncMetas: []*FuncMeta{
			NewFuncMete("NewApiContext", `json := jsoniter.ConfigCompatibleWithStandardLibrary()`),
			NewFuncMete("NewApiContext", `return &Context{
			visit: ast.NewIdent("hello"),
			}`),
		},
		DeclMetas: []*DeclMeta{
			NewDeclMeta(`
	// 初始化 Menu 路由信息
	// publicRouter 公开路由，不登录就可以访问
	// loginRouter  登录路由，登录后才可以访问
	func (s *MenuRouter) InitMenuRouter(publicRouter *gin.RouterGroup, loginRouter *gin.RouterGroup) {
		s.InitMenuBasicRouter(publicRouter, loginRouter)
		var handler = s.svcCtx.MenuController

		{
			loginRouter.POST("menus", handler.GetMenus) // 获取Menu列表
		}
	}`),
		},
	}
	inject.Walk()
	if err := inject.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}
