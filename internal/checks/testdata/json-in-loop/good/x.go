package x

import "encoding/json"

func all(items []any) ([]byte, error) { return json.Marshal(items) }
