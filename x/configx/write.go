package configx

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// -------------------------- 配置写入功能 --------------------------
// 将任意结构体数据写入文件，支持 JSON/YAML 格式
func WriteConfigAs(filename string, target any) error {
	if target == nil {
		return errors.New("target cannot be nil")
	}

	// 根据文件后缀判断序列化格式
	var (
		content []byte
		err     error
	)
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		// 带缩进的 JSON 格式，便于阅读
		content, err = json.MarshalIndent(target, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to serialize JSON: %w", err)
		}
	case ".yaml", ".yml":
		content, err = yaml.Marshal(target)
		if err != nil {
			return fmt.Errorf("failed to serialize YAML: %w", err)
		}
	default:
		return fmt.Errorf("unsupported file format: %s (only .json/.yaml/.yml supported)", ext)
	}

	// 写入文件（覆盖模式，权限 0644）
	if err := os.WriteFile(filename, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
