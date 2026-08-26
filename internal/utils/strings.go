package utils

// FirstNonEmpty 返回第一个非空字符串
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
