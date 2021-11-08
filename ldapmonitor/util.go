package ldapmonitor

func getKeysOfMaps(m map[string]map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func getKeysOfMap(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func union(lists ...[]string) []string {
	tempMap := map[string]int{}
	for _, list := range lists {
		for _, str := range list {
			tempMap[str] = 1
		}
	}

	keys := make([]string, 0, len(tempMap))
	for k := range tempMap {
		keys = append(keys, k)
	}
	return keys
}
