"""Generates the stateless CJK codec data (pkg/environment/cjk_tables.bin.gz,
written to stdout) by probing the codecs of the Python that runs the pinned
Jinja2: every byte sequence its decoders accept or reject (with the error
span they report) and every code point its encoders accept. Run via
`make cjktables`. `make parity` (and so CI) runs `--check FILE`, which
regenerates the tables and fails unless they are byte-identical to FILE's
decompressed content; comparing before compression keeps the check
independent of the zlib build.

Binary layout (all integers unsigned LEB128, strings length-prefixed):
  stateless codecs: count, then per codec: name, body length, body:
    flags (1 = gb18030 four-byte form)
    patterns: count, each 256 action bytes
              ('.' unit, '>' needs more bytes, '1'-'4' illegal span, 'g'
               gb18030 four-byte form, 'k' euc_kr eight-byte make-up form)
    nodes:    count, each (prefix, pattern index)
    units:    count, each (bytes, decoded string as UTF-8)
    encode:   count, each (code point, bytes)
    pairs:    count, each (code point, code point, bytes)
    ranges:   count, each (four-byte index, code point)  [gb18030 only]
  charset tables (decoders of the stateful codecs, shared between them):
    count, each: width, then count, each (code, decoded string as UTF-8)
  stateful codecs (hz, iso2022_*): count, then per codec: name, body length,
  body:
    tables:   count, each (kind: 0 = G0/G1, 2 = G2; final byte, with 0x80
              for double-byte sets; charset table index)
    encode:   count, each (code point, final byte, code)
    pairs:    count, each (code point, code point, final byte, code)
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


STATEFUL = [
    "hz", "iso2022_jp", "iso2022_jp_1", "iso2022_jp_2", "iso2022_jp_2004", "iso2022_jp_3",
    "iso2022_jp_ext", "iso2022_kr",
]

ESC, SO, SI = 0x1B, 0x0E, 0x0F
ASCII = ord("B")


def decodes(name, data):
    try:
        data.decode(name)
        return True
    except (UnicodeDecodeError, RuntimeError):
        return False


def designations(name):
    """The charsets an iso2022 decoder accepts, found by trying every
    designation escape: (kind, final byte, escape)."""
    found = []
    for f in range(0x40, 0x5B):
        if f != ASCII and decodes(name, bytes([ESC, ord("("), f])):
            found.append((0, f, bytes([ESC, ord("("), f])))
        if decodes(name, bytes([ESC, ord("$"), ord("("), f])):
            found.append((0, f | 0x80, bytes([ESC, ord("$"), ord("("), f])))
        if f != ASCII and decodes(name, bytes([ESC, ord("."), f])):
            found.append((2, f, bytes([ESC, ord("."), f])))
    return found


def charset_table(name, kind, mark, escape):
    """Probes one charset through the codec: {code: decoded text}."""
    table = {}
    if kind == 2:
        for b in range(256):
            try:
                table[b] = (escape + bytes([ESC, ord("N"), b])).decode(name)
            except UnicodeDecodeError:
                pass
            except RuntimeError:
                return None  # CPython has no G2 decoder for this charset
        return 1, table
    if mark & 0x80:
        for a in range(0x20, 0x80):
            for b in range(256):
                try:
                    table[a << 8 | b] = (escape + bytes([a, b])).decode(name)
                except UnicodeDecodeError:
                    pass
        return 2, table
    for a in range(0x20, 0x80):
        try:
            table[a] = (escape + bytes([a])).decode(name)
        except UnicodeDecodeError:
            pass
    return 1, table


def hz_table():
    table = {}
    for a in range(0x80):
        if a == ord("~"):
            continue
        for b in range(256):
            try:
                table[a << 8 | b] = (b"~{" + bytes([a, b])).decode("hz")
            except UnicodeDecodeError:
                pass
    return 2, table


def iso_units(out):
    """Splits iso2022 encoder output into (final byte, code) units."""
    units, g0, g1, shifted, i = [], ASCII, ASCII, False, 0
    while i < len(out):
        b = out[i]
        if b == ESC:
            if out[i + 1] == ord("$"):
                if out[i + 2] in b"()":
                    g, mark, i = out[i + 2], out[i + 3] | 0x80, i + 4
                else:
                    g, mark, i = ord("("), out[i + 2] | 0x80, i + 3
            else:
                g, mark, i = out[i + 1], out[i + 2], i + 3
            if g == ord("("):
                g0 = mark
            else:
                g1 = mark
        elif b in (SO, SI):
            shifted, i = b == SO, i + 1
        else:
            cs = g1 if shifted else g0
            width = 2 if cs & 0x80 else 1
            units.append((cs, out[i:i + width]))
            i += width
    return units


def hz_units(out):
    units, gb, i = [], False, 0
    while i < len(out):
        if out[i:i + 2] == b"~{":
            gb, i = True, i + 2
        elif out[i:i + 2] == b"~}":
            gb, i = False, i + 2
        elif gb:
            units.append((ord("A") | 0x80, out[i:i + 2]))
            i += 2
        else:
            units.append((ASCII, out[i:i + 1]))
            i += 2 if out[i:i + 2] == b"~~" else 1
    return units


def stateful(tables, table_ids):
    def intern(table):
        width, entries = table
        key = (width, tuple(sorted(entries.items())))
        if key not in table_ids:
            table_ids[key] = len(tables)
            tables.append(key)
        return table_ids[key]

    bodies = []
    for name in STATEFUL:
        units = hz_units if name == "hz" else iso_units
        refs = []
        if name == "hz":
            refs.append((0, ord("A") | 0x80, intern(hz_table())))
        else:
            for kind, mark, escape in designations(name):
                table = charset_table(name, kind, mark, escape)
                if table is not None:
                    refs.append((kind, mark, intern(table)))
        encode = []
        for cp in range(0x80, 0x110000):
            if 0xD800 <= cp < 0xE000:
                continue
            try:
                out = chr(cp).encode(name)
            except UnicodeEncodeError:
                continue
            (unit,) = units(out)
            encode.append((cp, unit))
        pairs = []
        heads = {}
        for kind, mark, tid in refs:
            for code, text in tables[tid][1]:
                if len(text) == 2:
                    heads.setdefault(ord(text[0]), set()).add(ord(text[1]))
        for head, mods in sorted(heads.items()):
            # The encoders match the follower on its low 16 bits, and the
            # JIS X 0213 ones hold (head, U+0000) pairs too.
            for mod in sorted(mods | {0}):
                got = units((chr(head) + chr(mod)).encode(name))
                if len(got) == 1:
                    pairs.append((head, mod, got[0]))
        out = io.BytesIO()
        out.write(uleb(len(refs)))
        for kind, mark, tid in refs:
            out.write(uleb(kind) + uleb(mark) + uleb(tid))
        out.write(uleb(len(encode)))
        for cp, (mark, code) in encode:
            out.write(uleb(cp) + uleb(mark) + lstr(code))
        out.write(uleb(len(pairs)))
        for head, mod, (mark, code) in pairs:
            out.write(uleb(head) + uleb(mod) + uleb(mark) + lstr(code))
        bodies.append((name, out.getvalue()))
    return bodies


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
    tables, table_ids = [], {}
    bodies = stateful(tables, table_ids)
    whole.write(uleb(len(tables)))
    for width, entries in tables:
        whole.write(uleb(width) + uleb(len(entries)))
        for code, text in entries:
            whole.write(uleb(code) + lstr(text.encode("utf-8")))
    whole.write(uleb(len(bodies)))
    for name, body in bodies:
        whole.write(lstr(name.encode()) + lstr(body))
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
