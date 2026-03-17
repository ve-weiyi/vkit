package patternx

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

func IsValidEmail(email string) bool {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	return regexp.MustCompile(pattern).MatchString(email)
}

func IsValidPhone(photo string) bool {
	return regexp.MustCompile(`^1[3456789]\d{9}$`).MatchString(photo)
}

func IsValidDigit(digit string) bool {
	return regexp.MustCompile(`^\d+$`).MatchString(digit)
}

// Valid version could be V1.0, V1.2.3, or V1.2.3.4
func IsValidVersion(version string) bool {
	return regexp.MustCompile(`^(V)?\d{1,4}(\.\d{1,4}){1,3}$`).MatchString(version)
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
