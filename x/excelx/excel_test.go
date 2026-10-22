package excelx

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExportImportRoundTrip 导出两 sheet 的样例后回读，校验标题、行内容与行数。
func TestExportImportRoundTrip(t *testing.T) {
	file := filepath.Join(t.TempDir(), "sample.xlsx")

	e := NewExcelExporter()
	for _, sheet := range []string{"Sheet1", "Sheet2"} {
		if err := e.NewActiveSheet(sheet); err != nil {
			t.Fatalf("NewActiveSheet(%s): %v", sheet, err)
		}
		if err := e.SetSheetTitle([]any{"姓名", "年龄", "性别"}); err != nil {
			t.Fatalf("SetSheetTitle(%s): %v", sheet, err)
		}
		if err := e.AddRowValue([]any{"张三", 18, "男"}); err != nil {
			t.Fatalf("AddRowValue(%s): %v", sheet, err)
		}
	}
	if err := e.ExportFile(file); err != nil {
		t.Fatalf("ExportFile: %v", err)
	}

	i := NewExcelImporter()
	if err := i.OpenFile(file); err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if err := i.SetActiveSheet("Sheet1"); err != nil {
		t.Fatalf("SetActiveSheet: %v", err)
	}

	title, err := i.GetSheetTitle()
	if err != nil {
		t.Fatalf("GetSheetTitle: %v", err)
	}
	if len(title) != 3 || title[0] != "姓名" {
		t.Errorf("标题 = %v, want [姓名 年龄 性别]", title)
	}

	row, err := i.GetRowValue(2)
	if err != nil {
		t.Fatalf("GetRowValue(2): %v", err)
	}
	if len(row) != 3 || row[0] != "张三" || row[1] != "18" {
		t.Errorf("第 2 行 = %v, want [张三 18 男]", row)
	}
	if got := i.RowCount(); got != 2 {
		t.Errorf("RowCount = %d, want 2", got)
	}
}

// TestImportExternalFile 读取 EXCELX_TEST_FILE 指定的 xlsx，未设置则跳过。
func TestImportExternalFile(t *testing.T) {
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
	if err != nil {
		t.Fatalf("GetSheetTitle: %v", err)
	}
	if len(title) == 0 {
		t.Error("标题行不应为空")
	}
	if _, err := i.GetRowValue(2); err != nil {
		t.Errorf("GetRowValue(2): %v", err)
	}
}
