package service

import (
	"strings"
	"testing"
)

var responseModelRewriteBenchmarkSink string

func BenchmarkResponseModelRewriteSSE(b *testing.B) {
	svc := &OpenAIGatewayService{}
	for _, tt := range []struct {
		name, payload, from, to string
	}{
		{"未映射且型号一致", `{"model":"deepseek-flash","choices":[{"delta":{"content":"hello"}}]}`, "deepseek-flash", "deepseek-flash"},
		{"未映射且型号不同", `{"model":"deepseek-v4.1-flash-1017","choices":[{"delta":{"content":"hello"}}]}`, "deepseek-flash", "deepseek-flash"},
		{"已有映射且型号不同", `{"model":"deepseek-v4.1-flash-1017","choices":[{"delta":{"content":"hello"}}]}`, "deepseek-flash", "public"},
		{"无型号文本增量", `{"type":"response.output_text.delta","delta":"hello"}`, "deepseek-flash", "deepseek-flash"},
		{"嵌套型号", `{"type":"response.completed","response":{"model":"deepseek-v4.1-flash-1017","output":[]}}`, "deepseek-flash", "deepseek-flash"},
		{"较大工具参数增量", `{"model":"deepseek-v4.1-flash-1017","choices":[{"delta":{"tool_calls":[{"function":{"arguments":"` + strings.Repeat("x", 4096) + `"}}]}}]}`, "deepseek-flash", "deepseek-flash"},
	} {
		b.Run(tt.name, func(b *testing.B) {
			line := "data: " + tt.payload
			b.ReportAllocs()
			b.SetBytes(int64(len(line)))
			for b.Loop() {
				responseModelRewriteBenchmarkSink = svc.replaceModelInSSELine(line, tt.from, tt.to)
			}
		})
	}
}
