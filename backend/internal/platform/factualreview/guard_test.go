package factualreview

import "testing"

func TestGeneratedReviewPlaceholderCannotBeFinalized(t *testing.T) {
	for _, text := range []string{Marker + "质保承诺缺少依据。】", "【事实待核实：\n企业事实待补充。】", "【事实 待核实：响应时间】", "提供[待澄清]年保修", "【待确认】天交付", "[待 填写]"} {
		if CheckText(text) != ErrRequired {
			t.Fatalf("expected review to block %q", text)
		}
	}
	for _, text := range []string{"", "交付期限为合同生效后30天。", "质保期按合同约定，不作无依据承诺。"} {
		if err := CheckText(text); err != nil {
			t.Fatalf("unexpected review block: %v", err)
		}
	}
}

func TestFragmentedEditorContentCannotBypassPlainTextGuard(t *testing.T) {
	content := map[string]any{"content": []any{map[string]any{"content": []any{
		map[string]any{"text": "【事实"}, map[string]any{"text": "待核实："},
	}}}}
	if err := CheckContent(content); err != ErrRequired {
		t.Fatalf("expected block, got %v", err)
	}
}
