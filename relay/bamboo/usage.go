package bamboo

import "github.com/QuantumNous/new-api/dto"

// accumulateReasoning 把 thinking delta 的 token 数累计到 Usage 的 reasoning 字段。
//
// 注意（复审修正）：CompletionTokenDetails 是【值类型 struct】（dto/openai_response.go:232），
// 不能 == nil 判断，也不能取址赋值；直接访问其字段即可。
// bamboo 的 StreamEvent 在 thinking delta 中携带，由 doStreamRelay 循环调用本函数累计。
func accumulateReasoning(usage *dto.Usage, delta int) {
	usage.CompletionTokenDetails.ReasoningTokens += delta
}
