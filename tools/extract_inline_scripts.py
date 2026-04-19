#!/usr/bin/env python3
"""Extract inline <script> blocks from a set of HTML pages into per-page
external .js files.

Phase 2.4 (SEC-001 part A): the dashboard pages currently embed their
app logic inside inline <script> blocks. The CSP therefore must allow
'unsafe-inline' in script-src, which provides no real defense against
stored XSS. As the prerequisite to a nonce-based CSP, this tool moves
each inline block out to an external file and rewrites the page to
reference it via <script src="...">.

Idempotent: if a page no longer contains an inline <script> block (only
src= references), it is left alone.

Usage:
    python3 tools/extract_inline_scripts.py <dir> <route_prefix>

Where:
    <dir>           filesystem directory containing HTML files
    <route_prefix>  URL prefix the rewritten <script src="..."> tags use
                    when referencing the extracted JS files

The tool writes external JS files into <dir>/scripts/<basename>.js and
appends a header comment to each so the origin is obvious in DevTools.
"""

from __future__ import annotations
import os
import re
import sys
from pathlib import Path

INLINE_SCRIPT_RE = re.compile(
    r'<script(?![^>]*\bsrc=)([^>]*)>([\s\S]*?)</script>',
    re.MULTILINE,
)


def extract_file(html_path: Path, scripts_dir: Path, route_prefix: str) -> bool:
    src = html_path.read_text()
    matches = list(INLINE_SCRIPT_RE.finditer(src))
    if not matches:
        return False

    rel_name = html_path.stem  # e.g. "index"
    # Settings subdir uses a flatter name; collisions are avoided by
    # taking the path relative to the html_path's parent.
    subdir = html_path.parent.name
    if subdir not in (scripts_dir.parent.name, ""):
        rel_name = f"{subdir}-{rel_name}"

    if len(matches) == 1:
        out_name = f"{rel_name}.js"
        out_path = scripts_dir / out_name
        attrs, body = matches[0].group(1), matches[0].group(2)
        defer = " defer" if "defer" in (attrs or "") else ""
        replacement = f'<script src="{route_prefix}/{out_name}"{defer}></script>'
        new_src = src[:matches[0].start()] + replacement + src[matches[0].end():]
        write_js(out_path, body, html_path.name)
        html_path.write_text(new_src)
        return True

    # Multiple inline blocks: number them in document order.
    new_src_parts = []
    cursor = 0
    for idx, m in enumerate(matches, start=1):
        out_name = f"{rel_name}.{idx}.js"
        out_path = scripts_dir / out_name
        attrs, body = m.group(1), m.group(2)
        defer = " defer" if "defer" in (attrs or "") else ""
        new_src_parts.append(src[cursor:m.start()])
        new_src_parts.append(f'<script src="{route_prefix}/{out_name}"{defer}></script>')
        write_js(out_path, body, html_path.name)
        cursor = m.end()
    new_src_parts.append(src[cursor:])
    html_path.write_text("".join(new_src_parts))
    return True


def write_js(out_path: Path, body: str, source_html: str) -> None:
    out_path.parent.mkdir(parents=True, exist_ok=True)
    header = (
        "// Auto-extracted from " + source_html + "\n"
        "// Phase 2.4 / SEC-001a: inline <script> moved to external file so\n"
        "// the CSP can drop 'unsafe-inline' (Phase 3.1).\n\n"
    )
    body = body.lstrip("\n")
    out_path.write_text(header + body)


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print(__doc__, file=sys.stderr)
        return 2
    dir_path = Path(argv[1]).resolve()
    route_prefix = argv[2].rstrip("/")
    if not dir_path.is_dir():
        print(f"not a directory: {dir_path}", file=sys.stderr)
        return 1
    scripts_dir = dir_path / "scripts"
    changed = 0
    for html_path in sorted(dir_path.rglob("*.html")):
        # Skip files inside a subdirectory we don't intend to handle.
        # All dashboard HTML, including dashboard/settings/*.html, gets
        # extracted; their JS lands in scripts/ at the top dashboard
        # level so route resolution stays simple.
        if extract_file(html_path, scripts_dir, route_prefix):
            changed += 1
            print(f"  extracted {html_path.relative_to(dir_path)}")
    print(f"done: {changed} file(s) modified")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
