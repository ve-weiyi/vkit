package excelx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// exportSample 导出一份两 sheet 的样例到临时目录，返回产物路径。
func exportSample(t *testing.T) string {
	t.Helper()

	e := NewExcelExporter()

	if err := e.NewActiveSheet("Sheet1"); err != nil {
		t.Fatalf("NewActiveSheet: %v", err)
	}
	if err := e.SetSheetTitle([]any{"姓名", "年龄", "性别"}); err != nil {
		t.Fatalf("SetSheetTitle: %v", err)
	}
	if err := e.AddRowValue([]any{"张三", 18, "男"}); err != nil {
		t.Fatalf("AddRowValue: %v", err)
	}
	if err := e.AddRowValue([]any{"李四", 18, "男"}); err != nil {
		t.Fatalf("AddRowValue: %v", err)
	}

	if err := e.NewActiveSheet("Sheet2"); err != nil {
		t.Fatalf("NewActiveSheet: %v", err)
	}
	if err := e.SetSheetTitle([]any{"姓名", "年龄", "性别"}); err != nil {
		t.Fatalf("SetSheetTitle: %v", err)
	}
	if err := e.AddRowValue([]any{"张三", 18, "男"}); err != nil {
		t.Fatalf("AddRowValue: %v", err)
	}
	if err := e.AddRowValue([]any{"李四", 18, "男"}); err != nil {
		t.Fatalf("AddRowValue: %v", err)
	}

	file := filepath.Join(t.TempDir(), "sample.xlsx")
	if err := e.ExportFile(file); err != nil {
		t.Fatalf("ExportFile: %v", err)
	}
	return file
}

// TestExport 导出样例，产物落在临时目录，不污染工作目录。
func TestExport(t *testing.T) {
	t.Log(exportSample(t))
}

// TestImport 导入刚导出的样例并逐行读取。
func TestImport(t *testing.T) {
	i := NewExcelImporter()

	if err := i.OpenFile(exportSample(t)); err != nil {
		t.Fatalf("OpenFile: %v", err)
	}

	if err := i.SetActiveSheet("Sheet1"); err != nil {
		t.Fatalf("SetActiveSheet: %v", err)
	}
	row, err := i.GetRowValue(0)
	t.Log(row, err)
	row, err = i.GetRowValue(1)
	t.Log(row, err)
	row, err = i.GetRowValue(2)
	t.Log(row, err)

	if err := i.SetActiveSheet("Sheet2"); err != nil {
		t.Fatalf("SetActiveSheet: %v", err)
	}
	row, err = i.GetRowValue(1)
	t.Log(row, err)
	row, err = i.GetRowValue(2)
	t.Log(row, err)
}

// TestImport2 读取 EXCELX_TEST_FILE 指定的 xlsx，未设置则跳过。
func TestImport2(t *testing.T) {
	path := os.Getenv("EXCELX_TEST_FILE")
	if path == "" {
		t.Skip("跳过：未设置 EXCELX_TEST_FILE")
	}

	i := NewExcelImporter()

	if err := i.OpenFile(path); err != nil {
		t.Fatalf("OpenFile: %v", err)
	}

	if err := i.SetActiveSheet("Sheet1"); err != nil {
		t.Fatalf("SetActiveSheet: %v", err)
	}

	title, err := i.GetSheetTitle()
	t.Log(title, err)

	row, err := i.GetRowValue(2)
	t.Log(row, err)
}

func TestNewExcelExporter(t *testing.T) {
	f := excelize.NewFile()
	ex := &ExcelExportImpl{
		File: f,
	}

	t.Log(ex.point(1, 1))
}
