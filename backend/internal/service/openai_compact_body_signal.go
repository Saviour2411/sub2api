package service

import "github.com/tidwall/gjson"

// HasCompactionTriggerInInput detects the Codex remote compact v2 body signal:
// an input item with type "compaction_trigger". A normal POST /v1/responses
// carrying this signal must be promoted to the compact path before routing.
func HasCompactionTriggerInInput(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	input := gjson.GetBytes(body, "input")
	if !input.Exists() {
		return false
	}
	if input.IsArray() {
		found := false
		input.ForEach(func(_, item gjson.Result) bool {
			if item.Get("type").String() == "compaction_trigger" {
				found = true
				return false
			}
			return true
		})
		return found
	}
	return input.Get("type").String() == "compaction_trigger"
}

func hasCompactionTriggerInInput(body []byte) bool {
	return HasCompactionTriggerInInput(body)
}
