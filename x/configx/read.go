package configx

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// -------------------------- 配置读取功能 --------------------------
func ReadConfigFrom(filename string, target any) error {
	// 校验目标必须是指针（否则无法修改原始数据）
	if target == nil {
		return errors.New("target cannot be nil")
	}
	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return errors.New("target must be a non-nil pointer")
	}

	// 读取文件内容
	if filename == "" {
		return errors.New("filename cannot be nil")
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// 根据文件后缀判断解析格式
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		if err := json.Unmarshal(content, target); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(content, target); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}
	default:
		return fmt.Errorf("unsupported file format: %s (only .json/.yaml/.yml supported)", ext)
	}

	return nil
}
