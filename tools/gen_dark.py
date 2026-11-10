#!/usr/bin/env python3
"""Generates static/css/dark.css from the site's own stylesheets.

Why generated: the site's colours are written directly into ~55 stylesheets. Rather than
maintaining a second hand-written copy, this script reads every stylesheet, and for each rule that
sets a colour it writes a dark twin under  html[data-theme="dark"]  with the colours remapped:

    backgrounds   light surfaces become dark surfaces (page darkest, cards a step lighter)
    text          dark text becomes light, brand greens/oranges become lighter tints
    borders       light lines become dark lines
    shadows       become black shadows

Brand-coloured buttons (dark green with white text) keep their colour.

Run after changing any stylesheet:      python3 tools/gen_dark.py
It reads static/css/**/*.css and writes static/css/dark.css (never edit that file by hand; put
manual fixes in static/css/dark-fixes.css instead, which loads after it).
"""
import colorsys
import glob
import os
import re
import sys

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
CSS_DIR = os.path.join(ROOT, "static", "css")
OUT = os.path.join(CSS_DIR, "dark.css")
SKIP_FILES = {"dark.css", "dark-fixes.css", "design-system.css"}
D = 'html[data-theme="dark"]'

# ---------------------------------------------------------------- colour helpers
NAMED = {"white": (255, 255, 255), "black": (0, 0, 0)}


def parse_color(tok):
    t = tok.strip().lower()
    if t in NAMED:
        return (*NAMED[t], 1.0)
    if t.startswith("#"):
        h = t[1:]
        if len(h) in (3, 4):
            h = "".join(c * 2 for c in h)
        if len(h) == 6:
            return (int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16), 1.0)
        if len(h) == 8:
            return (int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16), int(h[6:8], 16) / 255)
        return None
    m = re.match(r"rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)(?:\s*[,/]\s*([\d.]+%?))?\s*\)$", t)
    if m:
        a = 1.0
        if m.group(4):
            a = float(m.group(4)[:-1]) / 100 if m.group(4).endswith("%") else float(m.group(4))
        return (float(m.group(1)), float(m.group(2)), float(m.group(3)), a)
    return None


def fmt(r, g, b, a=1.0):
    r, g, b = (max(0, min(255, round(x))) for x in (r, g, b))
    if a >= 0.999:
        return "#%02x%02x%02x" % (r, g, b)
    return "rgba(%d,%d,%d,%s)" % (r, g, b, ("%.3f" % a).rstrip("0").rstrip("."))


def hls(r, g, b):
    h, l, s = colorsys.rgb_to_hls(r / 255, g / 255, b / 255)
    return h, l, s


def from_hls(h, l, s):
    r, g, b = colorsys.hls_to_rgb(h, max(0, min(1, l)), max(0, min(1, s)))
    return r * 255, g * 255, b * 255


def lerp(x, x0, x1, y0, y1):
    return y0 + (y1 - y0) * (x - x0) / (x1 - x0)


def map_bg(c):
    r, g, b, a = c
    h, l, s = hls(r, g, b)
    if a < 0.999 and r == g == b == 0:
        return c                                   # black overlays stay
    if l >= 0.995:
        nl = 0.125                                   # white cards
    elif l >= 0.965:
        nl = 0.085                                   # off-white page background
    elif l >= 0.85:
        nl = lerp(l, 0.965, 0.85, 0.16, 0.25)
    elif l >= 0.55:
        nl = lerp(l, 0.85, 0.55, 0.25, 0.42)
    else:
        return c                                     # dark/saturated: brand buttons, heroes
    ns = s * (0.55 if s < 0.9 else 0.5)
    chroma = (max(r, g, b) - min(r, g, b)) / 255
    if chroma < 0.10:                                # white / cream / grey: tint towards the brand green
        h, ns = 0.40, 0.14
    if a < 0.999:                                    # translucent light washes: much quieter
        a = a * 0.45
    return (*from_hls(h, nl, ns), a)


def map_text(c):
    r, g, b, a = c
    h, l, s = hls(r, g, b)
    if l >= 0.85:
        return c                                     # already light (text on dark things)
    nl = 0.95 - 0.85 * l if l < 0.75 else l
    nl = max(nl, 0.55)
    ns = s if s < 0.2 else min(1.0, s * 0.95)
    return (*from_hls(h, nl, ns), a)


def map_border(c):
    r, g, b, a = c
    if r == g == b == 0 and a < 0.999:
        return c
    h, l, s = hls(r, g, b)
    if l >= 0.6:
        nl = min(0.14 + (1 - l) * 0.7, 0.34)
        return (*from_hls(h, nl, s * 0.5), a)
    return c


def map_shadow(c):
    r, g, b, a = c
    if a >= 0.999 and (r, g, b) != (0, 0, 0):
        return c
    return (0, 0, 0, min(0.75, a * 3 + 0.12))


# ---------------------------------------------------------------- value rewriting
COLOR_RE = re.compile(
    r"(url\([^)]*\)|\"[^\"]*\"|'[^']*')|"                       # things to leave alone
    r"(rgba?\([^)]*\)|#[0-9a-fA-F]{3,8}\b|\b(?:white|black)\b)|"  # colours
    r"(var\(\s*(--[\w-]+)\s*(?:,[^)]*)?\))",                    # custom property use
    re.I,
)

ROLE_OF_PROP = {
    "background": "bg", "background-color": "bg", "background-image": "bg",
    "color": "text", "caret-color": "text", "fill": "text", "stroke": "text",
    "text-decoration-color": "text", "-webkit-text-fill-color": "text",
    "border": "border", "border-color": "border", "outline": "border", "outline-color": "border",
    "border-top": "border", "border-right": "border", "border-bottom": "border", "border-left": "border",
    "border-top-color": "border", "border-right-color": "border", "border-bottom-color": "border", "border-left-color": "border",
    "box-shadow": "shadow", "filter": None,
}
MAPPERS = {"bg": map_bg, "text": map_text, "border": map_border, "shadow": map_shadow}

# custom properties that hold colours, found while scanning: name -> parsed colour
VAR_COLORS = {}


def var_role(name):
    n = name.lower()
    if re.search(r"bg|cream|card|surface|soft|paper|panel|tint", n):
        return "bg"
    if re.search(r"line|border|stroke", n):
        return "border"
    if re.search(r"ink|muted|text|fg", n):
        return "text"
    return "accent"          # brand colours: unchanged as backgrounds, lightened when used as text


def rewrite_value(prop, value):
    """Returns the dark version of a declaration value, or None when nothing changes."""
    role = ROLE_OF_PROP.get(prop)
    if prop.startswith("--"):
        vr = var_role(prop)
        role = vr if vr in MAPPERS else None
        parsed = parse_color(value.strip())
        if parsed:
            VAR_COLORS.setdefault(prop, parsed)
        if role is None:
            return None
    if role is None:
        return None
    changed = False

    def sub(m):
        nonlocal changed
        if m.group(1):
            return m.group(1)
        if m.group(2):
            c = parse_color(m.group(2))
            if not c:
                return m.group(2)
            n = MAPPERS[role](c)
            if n != c:
                changed = True
                return fmt(*n)
            return m.group(2)
        name = m.group(4)
        vr = var_role(name)
        if vr == "accent" and role in ("text", "border") and name in VAR_COLORS:
            n = MAPPERS[role](VAR_COLORS[name])
            changed = True
            return fmt(*n)
        return m.group(3)

    out = COLOR_RE.sub(sub, value)
    return out if changed else None


# ---------------------------------------------------------------- CSS parsing
def strip_comments(s):
    return re.sub(r"/\*.*?\*/", "", s, flags=re.S)


def split_top(s, sep):
    """Splits on sep outside (), [] and quotes."""
    out, depth, quote, cur = [], 0, None, []
    for ch in s:
        if quote:
            cur.append(ch)
            if ch == quote:
                quote = None
            continue
        if ch in "\"'":
            quote = ch
        elif ch in "([":
            depth += 1
        elif ch in ")]":
            depth -= 1
        elif ch == sep and depth == 0:
            out.append("".join(cur))
            cur = []
            continue
        cur.append(ch)
    out.append("".join(cur))
    return out


def parse(s, i=0, top=True):
    """Returns (nodes, next_index). Node: ('rule', selector, body) or ('at', header, nodes_or_body)."""
    nodes = []
    n = len(s)
    while i < n:
        while i < n and s[i].isspace():
            i += 1
        if i >= n:
            break
        if s[i] == "}":
            return nodes, i + 1
        # read prelude up to { or ;
        j, depth, quote = i, 0, None
        while j < n:
            ch = s[j]
            if quote:
                if ch == quote:
                    quote = None
            elif ch in "\"'":
                quote = ch
            elif ch == "(":
                depth += 1
            elif ch == ")":
                depth -= 1
            elif depth == 0 and ch in "{;":
                break
            j += 1
        prelude = s[i:j].strip()
        if j >= n:
            break
        if s[j] == ";":
            i = j + 1          # e.g. @import / @charset
            continue
        # block
        if prelude.startswith("@"):
            name = prelude.split()[0].lower()
            if name in ("@media", "@supports", "@layer", "@container"):
                children, i = parse(s, j + 1, False)
                nodes.append(("at", prelude, children))
            else:                # @keyframes, @font-face, ...: skip the block
                depth_b, k = 1, j + 1
                while k < n and depth_b:
                    depth_b += (s[k] == "{") - (s[k] == "}")
                    k += 1
                i = k
        else:
            k = j + 1
            depth_p, quote = 0, None
            while k < n:
                ch = s[k]
                if quote:
                    if ch == quote:
                        quote = None
                elif ch in "\"'":
                    quote = ch
                elif ch == "(":
                    depth_p += 1
                elif ch == ")":
                    depth_p -= 1
                elif ch == "}" and depth_p == 0:
                    break
                k += 1
            nodes.append(("rule", prelude, s[j + 1:k]))
            i = k + 1
    return nodes, i


def dark_selector(sel):
    out = []
    for part in split_top(sel, ","):
        p = part.strip()
        if not p:
            continue
        if p.startswith(":root"):
            p = D + p[5:]
        elif re.match(r"^html\b", p):
            p = D + p[4:]
        else:
            p = D + " " + p
        out.append(p)
    return ",".join(out)


def rule_body_to_dark(body):
    decls = []
    for d in split_top(body, ";"):
        if ":" not in d:
            continue
        prop, value = d.split(":", 1)
        prop, value = prop.strip().lower(), value.strip()
        important = ""
        if re.search(r"!\s*important\s*$", value, re.I):
            important = " !important"
            value = re.sub(r"\s*!\s*important\s*$", "", value, flags=re.I)
        nv = rewrite_value(prop, value)
        if nv is not None:
            decls.append("%s:%s%s" % (prop, nv, important))
    return decls


def collect_vars(nodes):
    """First pass: learn colour values of custom properties so uses can be replaced."""
    for node in nodes:
        if node[0] == "at":
            collect_vars(node[2])
        else:
            for d in split_top(node[2], ";"):
                if ":" in d:
                    p, v = d.split(":", 1)
                    p = p.strip()
                    if p.startswith("--"):
                        c = parse_color(v.strip())
                        if c:
                            VAR_COLORS.setdefault(p, c)


def emit(nodes, out):
    for node in nodes:
        if node[0] == "at":
            head = node[1].lower()
            if "print" in head or "prefers-reduced-motion" in head:
                continue
            inner = []
            emit(node[2], inner)
            if inner:
                out.append(node[1] + "{" + "".join(inner) + "}")
        else:
            decls = rule_body_to_dark(node[2])
            if decls:
                out.append("%s{%s}\n" % (dark_selector(node[1]), ";".join(decls)))


def ordered_files():
    files = [f for f in glob.glob(os.path.join(CSS_DIR, "**", "*.css"), recursive=True) if os.path.basename(f) not in SKIP_FILES]
    def key(f):
        b = os.path.basename(f)
        return (0 if b == "base.css" else 2 if b == "ui.css" else 1, f)
    return sorted(files, key=key)


def main():
    files = ordered_files()
    parsed = []
    for f in files:
        with open(f, encoding="utf-8") as fh:
            nodes, _ = parse(strip_comments(fh.read()))
        parsed.append((f, nodes))
        collect_vars(nodes)
    chunks = []
    for f, nodes in parsed:
        out = []
        emit(nodes, out)
        if out:
            chunks.append("/* %s */\n%s" % (os.path.relpath(f, CSS_DIR), "".join(out)))
    header = ("/* GENERATED by tools/gen_dark.py. Do not edit; run the script again after changing any\n"
              "   stylesheet. Put manual fixes in dark-fixes.css. */\n")
    with open(OUT, "w", encoding="utf-8") as fh:
        fh.write(header + "\n".join(chunks))
    size = os.path.getsize(OUT)
    print("wrote %s (%d KB) from %d files" % (os.path.relpath(OUT, ROOT), size // 1024, len(files)))


if __name__ == "__main__":
    sys.exit(main())
