package dstx

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Llongfile)
}

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
		//ImportMetas: []*ImportMeta{
		//	NewImportMete(`jsoniter "github.com/json-iterator/go"`),
		//	NewImportMete(`"go/ast"`),
		//},
		//StructMetas: []*StructMeta{
		//	NewStructMete("Context", `Alias  *dst.Visitor //元素别名`),
		//},
		FuncMetas: []*FuncMeta{
			//&FuncMeta{
			//	FuncName:   "GetName",
			//	FuncPos:    4,
			//	Variables:  []string{"value", "err"},
			//	Symbol:     ":=",
			//	IdentNames: []string{"jsoniter", "AA", "ConfigCompatibleWithStandardLibrary"},
			//	//Parameters: []interface{}{"ss", 11},
			//},
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
	//err = inject.RollBack()
	if err := inject.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}
func TestNewAst(t *testing.T) {
	//NewImportMete(`jsoniter "github.com/json-iterator/go"`)
	//NewStructMete("ApiGroup", `Alias   *dst.Visitor //元素别名`)
	//NewFuncMete("NewApiContext", `json := jsoniter.ConfigCompatibleWithStandardLibrary()`)
	//NewFuncMete("NewApiContext", `return &Context{
	//	visitor: ast.NewIdent("hello"),
	//}`)

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
	}`)

}

func TestType(t *testing.T) {

	strings := []string{"11.0", "\"22\"", "11"}

	for _, str := range strings {
		result, err := inferType(str)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Result: %v (type: %T)\n", result, result)
		}
	}

}
