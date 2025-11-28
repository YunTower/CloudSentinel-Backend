package repositories

// stringsToInterfaceSlice 将字符串切片转换为 interface{} 切片
func stringsToInterfaceSlice(strs []string) []interface{} {
	if len(strs) == 0 {
		return nil
	}
	result := make([]interface{}, len(strs))
	for i, s := range strs {
		result[i] = s
	}
	return result
}

