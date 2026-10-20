"""Generates the stateless CJK codec data (pkg/environment/cjk_tables.bin.gz,
written to stdout) by probing the codecs of the Python that runs the pinned
Jinja2: every byte sequence its decoders accept or reject (with the error
span they report) and every code point its encoders accept. Run via
`make cjktables`. `make parity` (and so CI) runs `--check FILE`, which
regenerates the tables and fails unless they are byte-identical to FILE's
decompressed content; comparing before compression keeps the check
independent of the zlib build.

Binary layout (all integers unsigned LEB128, strings length-prefixed):
  count, then per codec: name, body length, body:
    flags (1 = gb18030 four-byte form)
    patterns: count, each 256 action bytes
              ('.' unit, '>' needs more bytes, '1'-'4' illegal span, 'g'
               gb18030 four-byte form, 'k' euc_kr eight-byte make-up form)
    nodes:    count, each (prefix, pattern index)
    units:    count, each (bytes, decoded string as UTF-8)
    encode:   count, each (code point, bytes)
    pairs:    count, each (code point, code point, bytes)
    ranges:   count, each (four-byte index, code point)  [gb18030 only]
"""

import gzip
import io
import sys

CODECS = [
    "big5", "big5hkscs", "cp932", "cp949", "cp950", "euc_jis_2004", "euc_jisx0213",
    "euc_jp", "euc_kr", "gb18030", "gb2312", "gbk", "johab", "shift_jis",
    "shift_jis_2004", "shift_jisx0213",
]

def uleb(n):
    out = bytearray()
    while True:
        b = n & 0x7F
        n >>= 7
        if n:
            out.append(b | 0x80)
        else:
            out.append(b)
            return bytes(out)


def lstr(b):
    return uleb(len(b)) + b


def probe(name, seq):
    try:
        return ("ok", seq.decode(name))
    except UnicodeDecodeError as e:
        return ("err", e.start, e.end, e.reason)


def decode_trie(name, gb18030):
    patterns, pattern_ids, nodes, units = [], {}, [], []
    queue = [b""]
    while queue:
        prefix = queue.pop(0)
        actions = bytearray()
        for b in range(256):
            seq = prefix + bytes([b])
            # CPython reads these forms whole before validating them, so
            # they are ported rather than probed byte by byte.
            if gb18030 and len(prefix) == 1 and 0x30 <= b <= 0x39:
                actions.append(ord("g"))
                continue
            if name == "euc_kr" and prefix == b"\xa4" and b == 0xD4:
                actions.append(ord("k"))
                continue
            r = probe(name, seq)
            if r[0] == "ok":
                actions.append(ord("."))
                units.append((seq, r[1]))
            elif r[3] == "incomplete multibyte sequence" and r[1] == 0 and r[2] == len(seq):
                actions.append(ord(">"))
                queue.append(seq)
            else:
                assert r[1] == 0 and r[3] == "illegal multibyte sequence", (name, seq, r)
                actions.append(ord(str(r[2] - r[1])))
        key = bytes(actions)
        if key not in pattern_ids:
            pattern_ids[key] = len(patterns)
            patterns.append(key)
        nodes.append((prefix, pattern_ids[key]))
    return patterns, nodes, units


def gb18030_ranges():
    # Four-byte sequences 81 30 81 30 .. 84 39 FE 39 map a linear index to
    # BMP code points in runs; record where each run starts.
    ranges = []
    prev = None
    for lseq in range(39420):
        c1, rest = divmod(lseq, 12600)
        c2, rest = divmod(rest, 1260)
        c3, c4 = divmod(rest, 10)
        seq = bytes([0x81 + c1, 0x30 + c2, 0x81 + c3, 0x30 + c4])
        cp = ord(seq.decode("gb18030"))
        if prev is None or cp != prev + 1:
            ranges.append((lseq, cp))
        prev = cp
    return ranges


def generate():
    whole = io.BytesIO()
    whole.write(uleb(len(CODECS)))
    for name in CODECS:
        out = io.BytesIO()
        gb = name == "gb18030"
        patterns, nodes, units = decode_trie(name, gb)
        encode, pairs = [], []
        for cp in range(0x110000):
            if 0xD800 <= cp < 0xE000:
                continue
            try:
                b = chr(cp).encode(name)
            except UnicodeEncodeError:
                continue
            if gb and len(b) == 4:
                continue  # computed from the ranges
            encode.append((cp, b))
        for seq, text in units:
            if len(text) == 2:
                pairs.append((ord(text[0]), ord(text[1]), text.encode(name)))
        # The JIS X 0213 encoders also hold (head, U+0000) pairs, which
        # swallow a following NUL (or anything the 16-bit match truncates
        # to it).
        for head in {p[0] for p in pairs}:
            alone = chr(head).encode(name)
            if (chr(head) + "\x00").encode(name) == alone:
                pairs.append((head, 0, alone))
        # pkg/environment packs these into 32-bit keys.
        assert max(len(seq) for seq, _ in units) <= 3 and max(len(p) for p, _ in nodes) <= 2
        out.write(uleb(1 if gb else 0))
        out.write(uleb(len(patterns)))
        for p in patterns:
            out.write(p)
        out.write(uleb(len(nodes)))
        for prefix, pid in nodes:
            out.write(lstr(prefix) + uleb(pid))
        out.write(uleb(len(units)))
        for seq, text in units:
            out.write(lstr(seq) + lstr(text.encode("utf-8")))
        out.write(uleb(len(encode)))
        for cp, b in encode:
            out.write(uleb(cp) + lstr(b))
        out.write(uleb(len(pairs)))
        for a, b, code in sorted(set(pairs)):
            out.write(uleb(a) + uleb(b) + lstr(code))
        ranges = gb18030_ranges() if gb else []
        out.write(uleb(len(ranges)))
        for lseq, cp in ranges:
            out.write(uleb(lseq) + uleb(cp))
        whole.write(lstr(name.encode()) + lstr(out.getvalue()))
    return whole.getvalue()


def main():
    if sys.argv[1:2] == ["--check"]:
        with gzip.open(sys.argv[2]) as f:
            if f.read() != generate():
                print(f"{sys.argv[2]} is stale or was modified; run 'make cjktables'", file=sys.stderr)
                return 1
        return 0
    sys.stdout.buffer.write(gzip.compress(generate(), compresslevel=9, mtime=0))
    return 0


if __name__ == "__main__":
    sys.exit(main())
