package x

import "encoding/json"

func each(items []any) {
	for _, it := range items {
		json.Marshal(it)
	}
}
