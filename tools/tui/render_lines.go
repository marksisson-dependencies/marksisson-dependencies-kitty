package tui

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"kitty/tools/tui/loop"
	"kitty/tools/utils/style"
	"kitty/tools/wcswidth"
)

var _ = fmt.Print

const KittyInternalHyperlinkProtocol = "kitty-ih"

func InternalHyperlink(text, id string) string {
	return fmt.Sprintf("\x1b]8;;%s:%s\x1b\\%s\x1b]8;;\x1b\\", KittyInternalHyperlinkProtocol, id, text)
}

type RenderLines struct {
	WrapOptions style.WrapOptions
}

var hyperlink_pat = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile("\x1b]8;([^;]*);.*?(\x1b\\\\|\a)")
})

// Render lines in the specified rectangle. If width > 0 then lines are wrapped
// to fit in the width. A string containing rendered lines with escape codes to
// move cursor is returned. Any internal hyperlinks are added to the
// MouseState.
func (r RenderLines) InRectangle(
	lines []string, start_x, start_y, width, height int, mouse_state *MouseState,
) (all_rendered bool, final_y int, ans string) {
	end_y := start_y + height - 1
	if end_y < start_y {
		return len(lines) == 0, start_y, ""
	}
	x, y := start_x, start_y
	buf := strings.Builder{}
	buf.Grow(len(lines) * max(1, width) * 3)
	move_cursor := func(x, y int) { buf.WriteString(fmt.Sprintf(loop.MoveCursorToTemplate, y+1, x+1)) }
	var hyperlink_state struct {
		action           string
		start_x, start_y int
	}

	start_hyperlink := func(action string) {
		hyperlink_state.action = action
		hyperlink_state.start_x, hyperlink_state.start_y = x, y
	}

	add_chunk := func(text string) {
		if text != "" {
			buf.WriteString(text)
			x += wcswidth.Stringwidth(text)
		}
	}

	commit_hyperlink := func() {
		mouse_state.AddCellRegion(hyperlink_state.action, hyperlink_state.start_x, hyperlink_state.start_y, x, y)
		hyperlink_state.action = ``
	}

	add_hyperlink := func(id, url string) {
		is_closer := id == "" && url == ""
		if is_closer {
			if hyperlink_state.action != "" {
				commit_hyperlink()
			} else {
				buf.WriteString("\x1b]8;;\x1b\\")
			}
		} else {
			if hyperlink_state.action != "" {
				commit_hyperlink()
			}
			if strings.HasPrefix(url, KittyInternalHyperlinkProtocol+":") {
				start_hyperlink(url[len(KittyInternalHyperlinkProtocol)+1:])
			} else {
				buf.WriteString(fmt.Sprintf("\x1b]8;%s;%s\x1b\\", id, url))
			}
		}

	}

	add_line := func(line string) {
		x = start_x
		indices := hyperlink_pat().FindAllStringSubmatchIndex(line, -1)
		start := 0
		for _, index := range indices {
			full_hyperlink_start, full_hyperlink_end := index[0], index[1]
			add_chunk(line[start:full_hyperlink_start])
			start = full_hyperlink_end
			add_hyperlink(line[index[2]:index[3]], line[index[4]:index[5]])
		}
		add_chunk(line[start:])
	}

	all_rendered = true
	for _, line := range lines {
		lines := []string{line}
		if width > 0 {
			lines = style.WrapTextAsLines(line, width, r.WrapOptions)
		}
		for _, line := range lines {
			if y > end_y {
				all_rendered = false
				goto end
			}
			move_cursor(start_x, y)
			add_line(line)
			y += 1
		}
	}
end:
	commit_hyperlink()
	return all_rendered, y, buf.String()
}