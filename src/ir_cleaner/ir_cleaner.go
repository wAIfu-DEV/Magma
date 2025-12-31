package ircleaner

import (
	"bytes"
)

type ClCtx struct {
	Decls map[string][]byte
}

func handleDeclare(ctx *ClCtx, line []byte) {
	lnLen := len(line)
	declName := []byte{}

	feed := false

	for i := range lnLen {
		if line[i] == '@' {
			feed = true
			continue
		}

		if feed {
			b := line[i]

			if b == '(' {
				break
			}
			if b == ' ' || b == '\t' {
				continue
			}
			declName = append(declName, line[i])
		}
	}
	ctx.Decls[string(declName)] = bytes.Clone(line)

	for i := range lnLen - 1 {
		line[i] = ' ' // erase line
	}
}

func processLine(ctx *ClCtx, line []byte) error {
	lnLen := len(line)

	// consume leading whitespace
	for lnLen > 0 {
		b := line[0]
		if b == ' ' || b == '\t' {
			line = line[1:]
			lnLen -= 1
			continue
		}
		break
	}

	if lnLen <= 0 {
		return nil
	}

	if line[0] == 'd' {
		if bytes.HasPrefix(line, []byte("declare ")) {
			handleDeclare(ctx, line)
			return nil
		}
	}

	return nil
}

// For now only needed for declaration deduplication since LLVM is quite
// stingy with them.
func CleanIr(irStr []byte) ([]byte, error) {
	ctx := &ClCtx{
		Decls: map[string][]byte{},
	}

	irLen := len(irStr)
	lineStartIdx := 0

	for i := range irLen {
		b := irStr[i]

		if b == '\n' {
			prevLnIdx := lineStartIdx
			lineStartIdx = i + 1

			e := processLine(ctx, irStr[prevLnIdx:lineStartIdx])
			if e != nil {
				return nil, e
			}
			continue
		}
	}

	irStr = append(irStr, "\n; Deduplicated declarations\n"...)

	for _, d := range ctx.Decls {
		irStr = append(
			irStr,
			d...,
		)
	}

	return irStr, nil
}
