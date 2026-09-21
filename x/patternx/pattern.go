package patternx

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// 正则提到包级：写在函数体内会每次调用重新编译
var (
	emailPattern   = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	mobilePattern  = regexp.MustCompile(`^1[3456789]\d{9}$`)
	digitPattern   = regexp.MustCompile(`^\d+$`)
	versionPattern = regexp.MustCompile(`^(V)?\d{1,4}(\.\d{1,4}){1,3}$`)
)

func IsValidEmail(email string) bool {
	return emailPattern.MatchString(email)
}

func IsValidMobile(mobile string) bool {
	return mobilePattern.MatchString(mobile)
}

func IsValidDigit(digit string) bool {
	return digitPattern.MatchString(digit)
}

// Valid version could be V1.0, V1.2.3, or V1.2.3.4
func IsValidVersion(version string) bool {
	return versionPattern.MatchString(version)
}

func CompareVersions(newVersion, oldVersion string) (int, error) {
	newVersion = strings.ToUpper(newVersion)
	oldVersion = strings.ToUpper(oldVersion)
	if !IsValidVersion(oldVersion) || !IsValidVersion(newVersion) {
		return 0, errors.New("invalid version format")
	}

	newVersion = strings.ReplaceAll(newVersion, "V", "")
	oldVersion = strings.ReplaceAll(oldVersion, "V", "")

	newComponents := strings.Split(newVersion, ".")
	oldComponents := strings.Split(oldVersion, ".")

	for i := 0; i < len(newComponents) && i < len(oldComponents); i++ {
		newNum, err := strconv.Atoi(newComponents[i])
		if err != nil {
			return 0, err
		}
		oldNum, err := strconv.Atoi(oldComponents[i])
		if err != nil {
			return 0, err
		}
		if newNum < oldNum {
			return -1, nil
		} else if newNum > oldNum {
			return 1, nil
		}
	}

	if len(newComponents) < len(oldComponents) {
		return -1, nil
	} else if len(newComponents) > len(oldComponents) {
		return 1, nil
	}
	return 0, nil
}
