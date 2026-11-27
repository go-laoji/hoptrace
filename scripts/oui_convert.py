import csv, re, sys
src, dst = sys.argv[1], sys.argv[2]
out = []
with open(src, newline="", encoding="utf-8") as f:
    r = csv.reader(f)
    next(r)
    for row in r:
        if len(row) < 3 or row[0] != "MA-L":
            continue
        name = re.sub(r"\s+", " ", row[2]).strip()
        out.append(row[1].upper() + "\t" + name)
out.sort()
open(dst, "w", encoding="utf-8").write("\n".join(out) + "\n")
print(len(out))
