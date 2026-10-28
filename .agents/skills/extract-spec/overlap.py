# Copy check (EXT-9): shared N-word runs between a source repo and spec Markdown.
# Usage: python3 -I overlap.py <source-dir> <spec-dir>...
import os, re, sys, pathlib
N = 10
word = re.compile(r"[a-z0-9]+")
def shingles(text):
    w = word.findall(text.lower())
    return {" ".join(w[i:i+N]) for i in range(len(w)-N+1)}
specs = {}
for root in sys.argv[2:]:
    for p in pathlib.Path(root).rglob("*.md"):
        specs[str(p)] = shingles(p.read_text(errors="ignore"))
allspec = set().union(*specs.values())
hits = {}
src = pathlib.Path(sys.argv[1])
exts = {".ts", ".tsx", ".js", ".md", ".mdx", ".prisma", ".json", ".yml", ".yaml", ".txt", ".html", ".hbs"}
for dirpath, dirs, files in os.walk(src):
    dirs[:] = [d for d in dirs if d not in {"node_modules", ".git", "dist", "build", "public", "locales"}]
    for f in files:
        p = pathlib.Path(dirpath, f)
        if p.suffix not in exts or p.stat().st_size > 2_000_000:
            continue
        for s in shingles(p.read_text(errors="ignore")) & allspec:
            hits.setdefault(s, set()).add(str(p.relative_to(src)))
print(f"{src.name}: {len(hits)} shared {N}-word runs")
for s, fs in sorted(hits.items())[:40]:
    print(" -", s, "|", sorted(fs)[0])
