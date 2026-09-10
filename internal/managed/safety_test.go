package managed

import "testing"

func TestAnalyzeInputFindsOneVMIDAndPreservesOriginal(t *testing.T) {
	in, err := AnalyzeInput("  麻烦查询 VM 3052 不是还在运行吗  ")
	if err != nil { t.Fatal(err) }
	if in.Original != "麻烦查询 VM 3052 不是还在运行吗" || in.VMID != 3052 || !in.HasNegation { t.Fatalf("input=%+v", in) }
}

func TestAnalyzeInputRejectsUnsafeStructures(t *testing.T) {
	for _, text := range []string{
		"查询虚拟机状态", "启动 VM 0", "启动 VM 3052 和 VM 3053", "如果 VM 3052 停了就启动", "启动所有 VM 3052", "先启动 VM 3052 然后重启", "关闭并重启 VM 3052",
	} {
		t.Run(text, func(t *testing.T) { if _, err := AnalyzeInput(text); err == nil { t.Fatal("want error") } })
	}
}

func TestAnalyzeInputAllowsNegatedReadQuestionForModelDecision(t *testing.T) {
	in, err := AnalyzeInput("VM 3052 不是还在运行吗？")
	if err != nil || !in.HasNegation || in.VMID != 3052 { t.Fatalf("input=%+v err=%v", in, err) }
}
