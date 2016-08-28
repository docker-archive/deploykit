package main

func getMap(v interface{}, key string) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		if mm, ok := m[key]; ok {
			if r, ok := mm.(map[string]interface{}); ok {
				return r
			}
		}
	}
	return nil
}
