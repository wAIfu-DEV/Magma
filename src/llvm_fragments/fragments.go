package llvmfragments

import (
	"bytes"
	_ "embed"
	"fmt"
	"strconv"
)

//go:embed utils.ll
var Utils []byte

func RenderUtils(traceSlots uint64) ([]byte, error) {
	if traceSlots == 0 || traceSlots > 1024 || traceSlots&(traceSlots-1) != 0 {
		return nil, fmt.Errorf("invalid error trace slot count %d", traceSlots)
	}
	warning := fmt.Sprintf("  ... trace truncated: diagnostic storage was reused or traversal reached --error-trace-slots=%d\\0A\\00", traceSlots)
	out := bytes.Clone(Utils)
	replacements := map[string]string{
		"{{TRACE_SLOTS}}":         strconv.FormatUint(traceSlots, 10),
		"{{TRACE_MASK}}":          strconv.FormatUint(traceSlots-1, 10),
		"{{TRACE_ARENA_PADDING}}": strconv.FormatUint((64-(traceSlots*16)%64)%64, 10),
		"{{TRACE_WARNING_LEN}}":   strconv.Itoa(len(warning) - 4),
		"{{TRACE_WARNING}}":       warning,
	}
	for from, to := range replacements {
		out = bytes.ReplaceAll(out, []byte(from), []byte(to))
	}
	return out, nil
}
