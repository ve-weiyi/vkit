package astx

import (
	"os"
	"path/filepath"
	"testing"
)

// fixturePath 把夹具拷到临时目录，避免注入直接改写源码树；夹具缺失则跳过。
func fixturePath(t *testing.T) string {
	t.Helper()

	const src = "../ast/test/test.go"
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
	inject := AstInjectionMeta{
		FilePath: fixturePath(t),
		ImportMetas: []*ImportMeta{
			{
				ImportAlias:   "jsoniter",
				ImportPackage: "github.com/json-iterator/go",
			},
			{
				ImportAlias:   "",
				ImportPackage: "go/ast",
			},
		},
		StructMetas: []*StructMeta{
			{
				FindStructName: "ApiGroup",
				FieldName:      "visitor",
				FieldType:      "ast",
				FieldComment:   "Visitor",
			},
		},
		FuncMetas: []*FuncMeta{
			{
				FuncName:   "GetName",
				FuncPos:    3,
				Variables:  []string{"value"},
				Symbol:     ":=",
				IdentNames: []string{"jsoniter", "AA", "ConfigCompatibleWithStandardLibrary"},
			},
		},
	}

	if err := inject.Inject(); err != nil {
		t.Fatalf("Inject: %v", err)
	}
}
