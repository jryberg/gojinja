package environment

import "testing"

// Expected values come from the Python that runs the Jinja2 pinned in
// tools/parity/requirements.txt, including CPython's quirks: pair
// lookups on the follower's low 16 bits, euc_kr make-up sequences,
// gb18030's four-byte forms and one-way mappings.
func TestCJKCodecs(t *testing.T) {
	cases := []struct{ op, codec, errors, in, want, err string }{
		{"enc", "big5", "strict", "\u4e2d\u6587\u7e41\u9ad4", "\xa4\xa4\xa4\xe5\xc1\x63\xc5\xe9", ""},
		{"enc", "big5", "strict", "\u20ac", "", "'big5' codec can't encode character '\\u20ac' in position 0: illegal multibyte sequence"},
		{"enc", "big5", "xmlcharrefreplace", "\u4e2d\u6587\u7e41\u9ad4\U0001f600\u0e01x", "\xa4\xa4\xa4\xe5\xc1\x63\xc5\xe9\x26\x23\x31\x32\x38\x35\x31\x32\x3b\x26\x23\x33\x35\x38\x35\x3b\x78", ""},
		{"dec", "big5", "strict", "\xa4\xa4\xa4\xe5\xc1\x63\xc5\xe9", "\u4e2d\u6587\u7e41\u9ad4", ""},
		{"dec", "big5", "strict", "\xa4\xa4\xa4\xe5\xc1\x63\xc5", "", "'big5' codec can't decode byte 0xc5 in position 6: incomplete multibyte sequence"},
		{"dec", "big5", "replace", "\xff\xff\xa4\xa4\xa4\xe5\xc1\x63\xc5\xe9\x80", "\ufffd\ufffd\u4e2d\u6587\u7e41\u9ad4\ufffd", ""},
		{"dec", "big5", "backslashreplace", "\x81\x20\xfe\xa4\xa4\xa4", "\\x81 \\xfe\u4e2d\\xa4", ""},
		{"enc", "big5hkscs", "strict", "\u00ca\u0304\u00ca\u030c\u00ea", "\x88\x62\x88\x64\x88\xa7", ""},
		{"enc", "big5hkscs", "strict", "\u00ca\U00010304", "\x88\x62", ""},
		{"enc", "big5hkscs", "strict", "\U00020021", "\x9c\x71", ""},
		{"enc", "big5hkscs", "backslashreplace", "\u00ca\u0304\u00ca\u030c\u00ea\U0001f600\u0e01x", "\x88\x62\x88\x64\x88\xa7\x5c\x55\x30\x30\x30\x31\x66\x36\x30\x30\x5c\x75\x30\x65\x30\x31\x78", ""},
		{"dec", "big5hkscs", "strict", "\x88\x62\x88\x64\x88\xa7", "\u00ca\u0304\u00ca\u030c\u00ea", ""},
		{"dec", "big5hkscs", "strict", "\x88\x62\x88\x64\x88", "", "'big5hkscs' codec can't decode byte 0x88 in position 4: incomplete multibyte sequence"},
		{"dec", "big5hkscs", "replace", "\xff\xff\x88\x62\x88\x64\x88\xa7\x80", "\ufffd\ufffd\u00ca\u0304\u00ca\u030c\u00ea\ufffd", ""},
		{"dec", "big5hkscs", "backslashreplace", "\x81\x20\xfe\x88\x62\x88", "\\x81 \\xfe\u00ca\u0304\\x88", ""},
		{"enc", "cp932", "strict", "\uff5e\uff0d\uffe2\u00a2", "\x81\x60\x81\x7c\x81\xca\x81\x91", ""},
		{"enc", "cp932", "strict", "\uff71\uff72\uff73\u2460", "\xb1\xb2\xb3\x87\x40", ""},
		{"enc", "cp932", "namereplace", "\uff5e\uff0d\uffe2\u00a2\U0001f600\u0e01x", "\x81\x60\x81\x7c\x81\xca\x81\x91\x5c\x4e\x7b\x47\x52\x49\x4e\x4e\x49\x4e\x47\x20\x46\x41\x43\x45\x7d\x5c\x4e\x7b\x54\x48\x41\x49\x20\x43\x48\x41\x52\x41\x43\x54\x45\x52\x20\x4b\x4f\x20\x4b\x41\x49\x7d\x78", ""},
		{"dec", "cp932", "strict", "\x81\x60\x81\x7c\x81\xca\x81\x91", "\uff5e\uff0d\uffe2\uffe0", ""},
		{"dec", "cp932", "strict", "\x81\x60\x81\x7c\x81\xca\x81", "", "'cp932' codec can't decode byte 0x81 in position 6: incomplete multibyte sequence"},
		{"dec", "cp932", "replace", "\xff\xff\x81\x60\x81\x7c\x81\xca\x81\x91\x80", "\uf8f3\uf8f3\uff5e\uff0d\uffe2\uffe0\u0080", ""},
		{"dec", "cp932", "backslashreplace", "\x81\x20\xfe\x81\x60\x81", "\\x81 \uf8f2\uff5e\\x81", ""},
		{"enc", "cp949", "strict", "\ud55c\uad6d\uc5b4\ub620", "\xc7\xd1\xb1\xb9\xbe\xee\x8c\x63", ""},
		{"enc", "cp949", "strict", "\uac02", "\x81\x41", ""},
		{"enc", "cp949", "replace", "\ud55c\uad6d\uc5b4\ub620\U0001f600\u0e01x", "\xc7\xd1\xb1\xb9\xbe\xee\x8c\x63\x3f\x3f\x78", ""},
		{"dec", "cp949", "strict", "\xc7\xd1\xb1\xb9\xbe\xee\x8c\x63", "\ud55c\uad6d\uc5b4\ub620", ""},
		{"dec", "cp949", "strict", "\xc7\xd1\xb1\xb9\xbe\xee\x8c", "", "'cp949' codec can't decode byte 0x8c in position 6: incomplete multibyte sequence"},
		{"dec", "cp949", "replace", "\xff\xff\xc7\xd1\xb1\xb9\xbe\xee\x8c\x63\x80", "\ufffd\ufffd\ud55c\uad6d\uc5b4\ub620\ufffd", ""},
		{"dec", "cp949", "backslashreplace", "\x81\x20\xfe\xc7\xd1\xb1", "\\x81 \\xfe\ud55c\\xb1", ""},
		{"enc", "cp950", "strict", "\u4e2d\u6587\u20ac", "\xa4\xa4\xa4\xe5\xa3\xe1", ""},
		{"enc", "cp950", "replace", "\u4e2d\u6587\u20ac\U0001f600\u0e01x", "\xa4\xa4\xa4\xe5\xa3\xe1\x3f\x3f\x78", ""},
		{"dec", "cp950", "strict", "\xa4\xa4\xa4\xe5\xa3\xe1", "\u4e2d\u6587\u20ac", ""},
		{"dec", "cp950", "strict", "\xa4\xa4\xa4\xe5\xa3", "", "'cp950' codec can't decode byte 0xa3 in position 4: incomplete multibyte sequence"},
		{"dec", "cp950", "replace", "\xff\xff\xa4\xa4\xa4\xe5\xa3\xe1\x80", "\ufffd\ufffd\u4e2d\u6587\u20ac\ufffd", ""},
		{"dec", "cp950", "backslashreplace", "\x81\x20\xfe\xa4\xa4\xa4", "\\x81 \\xfe\u4e2d\\xa4", ""},
		{"enc", "euc_jis_2004", "strict", "\u304b\u309a\u31f7\u309a\U00020089", "\xa4\xf7\xa6\xf8\x8f\xa1\xa1", ""},
		{"enc", "euc_jis_2004", "strict", "\u304b\u0000", "\xa4\xab", ""},
		{"enc", "euc_jis_2004", "strict", "\u304b\U00020000", "\xa4\xab", ""},
		{"enc", "euc_jis_2004", "ignore", "\u304b\u309a\u31f7\u309a\U00020089\U0001f600\u0e01x", "\xa4\xf7\xa6\xf8\x8f\xa1\xa1\x78", ""},
		{"dec", "euc_jis_2004", "strict", "\xa4\xf7\xa6\xf8\x8f\xa1\xa1", "\u304b\u309a\u31f7\u309a\U00020089", ""},
		{"dec", "euc_jis_2004", "strict", "\xa4\xf7\xa6\xf8\x8f\xa1", "", "'euc_jis_2004' codec can't decode bytes in position 4-5: incomplete multibyte sequence"},
		{"dec", "euc_jis_2004", "replace", "\xff\xff\xa4\xf7\xa6\xf8\x8f\xa1\xa1\x80", "\ufffd\ufffd\u304b\u309a\u31f7\u309a\U00020089\ufffd", ""},
		{"dec", "euc_jis_2004", "backslashreplace", "\x81\x20\xfe\xa4\xf7\xa6", "\\x81 \uf9d0\u6f2a", ""},
		{"enc", "euc_jisx0213", "strict", "\u304d\u309a\u00ca\u0304", "\xa4\xf8\xa9\xc1\xab\xdb", ""},
		{"enc", "euc_jisx0213", "replace", "\u304d\u309a\u00ca\u0304\U0001f600\u0e01x", "\xa4\xf8\xa9\xc1\xab\xdb\x3f\x3f\x78", ""},
		{"dec", "euc_jisx0213", "strict", "\xa4\xf8\xa9\xc1\xab\xdb", "\u304d\u309a\u00ca\u0304", ""},
		{"dec", "euc_jisx0213", "strict", "\xa4\xf8\xa9\xc1\xab", "", "'euc_jisx0213' codec can't decode byte 0xab in position 4: incomplete multibyte sequence"},
		{"dec", "euc_jisx0213", "replace", "\xff\xff\xa4\xf8\xa9\xc1\xab\xdb\x80", "\ufffd\ufffd\u304d\u309a\u00ca\u0304\ufffd", ""},
		{"dec", "euc_jisx0213", "backslashreplace", "\x81\x20\xfe\xa4\xf8\xa9", "\\x81 \uf9d0\u742b", ""},
		{"enc", "euc_jp", "strict", "\u65e5\u672c\u8a9e\u00a5\u203e", "\xc6\xfc\xcb\xdc\xb8\xec\x5c\x7e", ""},
		{"enc", "euc_jp", "strict", "\u00a1\u00a4", "\x8f\xa2\xc2\x8f\xa2\xf0", ""},
		{"enc", "euc_jp", "xmlcharrefreplace", "\u65e5\u672c\u8a9e\u00a5\u203e\U0001f600\u0e01x", "\xc6\xfc\xcb\xdc\xb8\xec\x5c\x7e\x26\x23\x31\x32\x38\x35\x31\x32\x3b\x26\x23\x33\x35\x38\x35\x3b\x78", ""},
		{"dec", "euc_jp", "strict", "\xc6\xfc\xcb\xdc\xb8\xec\x5c\x7e", "\u65e5\u672c\u8a9e\\~", ""},
		{"dec", "euc_jp", "strict", "\xc6\xfc\xcb\xdc\xb8\xec\x5c", "\u65e5\u672c\u8a9e\\", ""},
		{"dec", "euc_jp", "replace", "\xff\xff\xc6\xfc\xcb\xdc\xb8\xec\x5c\x7e\x80", "\ufffd\ufffd\u65e5\u672c\u8a9e\\~\ufffd", ""},
		{"dec", "euc_jp", "backslashreplace", "\x81\x20\xfe\xc6\xfc\xcb", "\\x81 \\xfe\u65e5\\xcb", ""},
		{"enc", "euc_kr", "strict", "\ud55c\uad6d\uc5b4", "\xc7\xd1\xb1\xb9\xbe\xee", ""},
		{"enc", "euc_kr", "strict", "\ub620", "\xa4\xd4\xa4\xa8\xa4\xc7\xa4\xb1", ""},
		{"enc", "euc_kr", "strict", "\uac02", "\xa4\xd4\xa4\xa1\xa4\xbf\xa4\xa2", ""},
		{"enc", "euc_kr", "ignore", "\ud55c\uad6d\uc5b4\U0001f600\u0e01x", "\xc7\xd1\xb1\xb9\xbe\xee\x78", ""},
		{"dec", "euc_kr", "strict", "\xc7\xd1\xb1\xb9\xbe\xee", "\ud55c\uad6d\uc5b4", ""},
		{"dec", "euc_kr", "strict", "\xc7\xd1\xb1\xb9\xbe", "", "'euc_kr' codec can't decode byte 0xbe in position 4: incomplete multibyte sequence"},
		{"dec", "euc_kr", "replace", "\xff\xff\xc7\xd1\xb1\xb9\xbe\xee\x80", "\ufffd\ufffd\ud55c\uad6d\uc5b4\ufffd", ""},
		{"dec", "euc_kr", "backslashreplace", "\x81\x20\xfe\xc7\xd1\xb1", "\\x81 \\xfe\ud55c\\xb1", ""},
		{"enc", "gb18030", "strict", "\u4e2d\u6587\u20ac\u0080\U0001f600\ue5e5", "\xd6\xd0\xce\xc4\xa2\xe3\x81\x30\x81\x30\x94\x39\xfc\x36\xa3\xa0", ""},
		{"enc", "gb18030", "strict", "\uffff\U0010ffff", "\x84\x31\xa4\x39\xe3\x32\x9a\x35", ""},
		{"enc", "gb18030", "replace", "\u4e2d\u6587\u20ac\u0080\U0001f600\ue5e5\U0001f600\u0e01x", "\xd6\xd0\xce\xc4\xa2\xe3\x81\x30\x81\x30\x94\x39\xfc\x36\xa3\xa0\x94\x39\xfc\x36\x81\x32\xce\x39\x78", ""},
		{"dec", "gb18030", "strict", "\xd6\xd0\xce\xc4\xa2\xe3\x81\x30\x81\x30\x94\x39\xfc\x36\xa3\xa0", "\u4e2d\u6587\u20ac\u0080\U0001f600\ue5e5", ""},
		{"dec", "gb18030", "strict", "\xd6\xd0\xce\xc4\xa2\xe3\x81\x30\x81\x30\x94\x39\xfc\x36\xa3", "", "'gb18030' codec can't decode byte 0xa3 in position 14: incomplete multibyte sequence"},
		{"dec", "gb18030", "replace", "\xff\xff\xd6\xd0\xce\xc4\xa2\xe3\x81\x30\x81\x30\x94\x39\xfc\x36\xa3\xa0\x80", "\ufffd\ufffd\u4e2d\u6587\u20ac\u0080\U0001f600\ue5e5\ufffd", ""},
		{"dec", "gb18030", "backslashreplace", "\x81\x20\xfe\xd6\xd0\xce", "\\x81 \ue49d\u5f62", ""},
		{"enc", "gb2312", "strict", "\u4e2d\u6587", "\xd6\xd0\xce\xc4", ""},
		{"enc", "gb2312", "strict", "\u20ac", "", "'gb2312' codec can't encode character '\\u20ac' in position 0: illegal multibyte sequence"},
		{"enc", "gb2312", "ignore", "\u4e2d\u6587\U0001f600\u0e01x", "\xd6\xd0\xce\xc4\x78", ""},
		{"dec", "gb2312", "strict", "\xd6\xd0\xce\xc4", "\u4e2d\u6587", ""},
		{"dec", "gb2312", "strict", "\xd6\xd0\xce", "", "'gb2312' codec can't decode byte 0xce in position 2: incomplete multibyte sequence"},
		{"dec", "gb2312", "replace", "\xff\xff\xd6\xd0\xce\xc4\x80", "\ufffd\ufffd\u4e2d\u6587\ufffd", ""},
		{"dec", "gb2312", "backslashreplace", "\x81\x20\xfe\xd6\xd0\xce", "\\x81 \\xfe\u4e2d\\xce", ""},
		{"enc", "gbk", "strict", "\u4e2d\u6587\u20ac", "", "'gbk' codec can't encode character '\\u20ac' in position 2: illegal multibyte sequence"},
		{"enc", "gbk", "backslashreplace", "\u4e2d\u6587\u20ac\U0001f600\u0e01x", "\xd6\xd0\xce\xc4\x5c\x75\x32\x30\x61\x63\x5c\x55\x30\x30\x30\x31\x66\x36\x30\x30\x5c\x75\x30\x65\x30\x31\x78", ""},
		{"dec", "gbk", "strict", "\xd6\xd0\xce\xc4", "\u4e2d\u6587", ""},
		{"dec", "gbk", "strict", "\xd6\xd0\xce", "", "'gbk' codec can't decode byte 0xce in position 2: incomplete multibyte sequence"},
		{"dec", "gbk", "replace", "\xff\xff\xd6\xd0\xce\xc4\x80", "\ufffd\ufffd\u4e2d\u6587\ufffd", ""},
		{"dec", "gbk", "backslashreplace", "\x81\x20\xfe\xd6\xd0\xce", "\\x81 \\xfe\u4e2d\\xce", ""},
		{"enc", "johab", "strict", "\ud55c\uad6d\uc5b4\ub620", "\xd0\x65\x8a\x82\xb4\xe1\x99\xb1", ""},
		{"enc", "johab", "replace", "\ud55c\uad6d\uc5b4\ub620\U0001f600\u0e01x", "\xd0\x65\x8a\x82\xb4\xe1\x99\xb1\x3f\x3f\x78", ""},
		{"dec", "johab", "strict", "\xd0\x65\x8a\x82\xb4\xe1\x99\xb1", "\ud55c\uad6d\uc5b4\ub620", ""},
		{"dec", "johab", "strict", "\xd0\x65\x8a\x82\xb4\xe1\x99", "", "'johab' codec can't decode byte 0x99 in position 6: incomplete multibyte sequence"},
		{"dec", "johab", "replace", "\xff\xff\xd0\x65\x8a\x82\xb4\xe1\x99\xb1\x80", "\ufffd\ufffd\ud55c\uad6d\uc5b4\ub620\ufffd", ""},
		{"dec", "johab", "backslashreplace", "\x81\x20\xfe\xd0\x65\x8a", "\\x81 \\xfe\ud55c\\x8a", ""},
		{"enc", "shift_jis", "strict", "\u65e5\u672c\u8a9e\u00a5\u203e\uff5e", "", "'shift_jis' codec can't encode character '\\uff5e' in position 5: illegal multibyte sequence"},
		{"enc", "shift_jis", "strict", "\\~", "\x5c\x7e", ""},
		{"enc", "shift_jis", "replace", "\u65e5\u672c\u8a9e\u00a5\u203e\uff5e\U0001f600\u0e01x", "\x93\xfa\x96\x7b\x8c\xea\x5c\x7e\x3f\x3f\x3f\x78", ""},
		{"dec", "shift_jis", "strict", "\x93\xfa\x96\x7b\x8c\xea\x5c\x7e", "\u65e5\u672c\u8a9e\\~", ""},
		{"dec", "shift_jis", "strict", "\x93\xfa\x96\x7b\x8c\xea\x5c", "\u65e5\u672c\u8a9e\\", ""},
		{"dec", "shift_jis", "replace", "\xff\xff\x93\xfa\x96\x7b\x8c\xea\x5c\x7e\x80", "\ufffd\ufffd\u65e5\u672c\u8a9e\\~\ufffd", ""},
		{"dec", "shift_jis", "backslashreplace", "\x81\x20\xfe\x93\xfa\x96", "\\x81 \\xfe\u65e5\\x96", ""},
		{"enc", "shift_jis_2004", "strict", "\u304b\u309a\U0002000b", "\x82\xf5\x87\xa0", ""},
		{"enc", "shift_jis_2004", "strict", "\u304b\U0001309a", "\x82\xf5", ""},
		{"enc", "shift_jis_2004", "namereplace", "\u304b\u309a\U0002000b\U0001f600\u0e01x", "\x82\xf5\x87\xa0\x81\x5f\x4e\x7b\x47\x52\x49\x4e\x4e\x49\x4e\x47\x20\x46\x41\x43\x45\x7d\x81\x5f\x4e\x7b\x54\x48\x41\x49\x20\x43\x48\x41\x52\x41\x43\x54\x45\x52\x20\x4b\x4f\x20\x4b\x41\x49\x7d\x78", ""},
		{"dec", "shift_jis_2004", "strict", "\x82\xf5\x87\xa0", "\u304b\u309a\U0002000b", ""},
		{"dec", "shift_jis_2004", "strict", "\x82\xf5\x87", "", "'shift_jis_2004' codec can't decode byte 0x87 in position 2: incomplete multibyte sequence"},
		{"dec", "shift_jis_2004", "replace", "\xff\xff\x82\xf5\x87\xa0\x80", "\ufffd\ufffd\u304b\u309a\U0002000b\ufffd", ""},
		{"dec", "shift_jis_2004", "backslashreplace", "\x81\x20\xfe\x82\xf5\x87", "\\x81 \\xfe\u304b\u309a\\x87", ""},
		{"enc", "shift_jisx0213", "strict", "\u31f7\u309a", "\x83\xf6", ""},
		{"enc", "shift_jisx0213", "namereplace", "\u31f7\u309a\U0001f600\u0e01x", "\x83\xf6\x81\x5f\x4e\x7b\x47\x52\x49\x4e\x4e\x49\x4e\x47\x20\x46\x41\x43\x45\x7d\x81\x5f\x4e\x7b\x54\x48\x41\x49\x20\x43\x48\x41\x52\x41\x43\x54\x45\x52\x20\x4b\x4f\x20\x4b\x41\x49\x7d\x78", ""},
		{"dec", "shift_jisx0213", "strict", "\x83\xf6", "\u31f7\u309a", ""},
		{"dec", "shift_jisx0213", "strict", "\x83", "", "'shift_jisx0213' codec can't decode byte 0x83 in position 0: incomplete multibyte sequence"},
		{"dec", "shift_jisx0213", "replace", "\xff\xff\x83\xf6\x80", "\ufffd\ufffd\u31f7\u309a\ufffd", ""},
		{"dec", "shift_jisx0213", "backslashreplace", "\x81\x20\xfe\x83\xf6", "\\x81 \\xfe\u31f7\u309a", ""},
		{"dec", "euc_kr", "strict", "\xa4\xd4\xa4\xa1\xa4\xbf\xa4\xd4", "\uac00", ""},
		{"dec", "euc_kr", "replace", "\xa4\xd4\xa4\xa1\xa4\xbf\xa4\xd4", "\uac00", ""},
		{"dec", "euc_kr", "strict", "\xa4\xd4\xa4\xa3\xa4\xbf\xa4\xd4\x78", "", "'euc_kr' codec can't decode byte 0xa4 in position 0: illegal multibyte sequence"},
		{"dec", "euc_kr", "replace", "\xa4\xd4\xa4\xa3\xa4\xbf\xa4\xd4\x78", "\ufffd\u6e21\uff04\uc5d8\ufffdx", ""},
		{"dec", "euc_kr", "strict", "\xa4\xd4\xa4", "", "'euc_kr' codec can't decode bytes in position 0-2: incomplete multibyte sequence"},
		{"dec", "euc_kr", "replace", "\xa4\xd4\xa4", "\ufffd", ""},
		{"dec", "gb18030", "strict", "\x81\x30\x81\x30\x84\x31\xa4\x39\x90\x30\x81\x30\xe3\x32\x9a\x35", "\u0080\uffff\U00010000\U0010ffff", ""},
		{"dec", "gb18030", "replace", "\x81\x30\x81\x30\x84\x31\xa4\x39\x90\x30\x81\x30\xe3\x32\x9a\x35", "\u0080\uffff\U00010000\U0010ffff", ""},
		{"dec", "gb18030", "strict", "\xe3\x32\x9a\x36", "", "'gb18030' codec can't decode byte 0xe3 in position 0: illegal multibyte sequence"},
		{"dec", "gb18030", "replace", "\xe3\x32\x9a\x36", "\ufffd2\ufffd", ""},
		{"dec", "gb18030", "strict", "\x84\x31\xa5\x30", "", "'gb18030' codec can't decode byte 0x84 in position 0: illegal multibyte sequence"},
		{"dec", "gb18030", "replace", "\x84\x31\xa5\x30", "\ufffd1\ufffd", ""},
		{"dec", "gb18030", "strict", "\x81\x30\x81", "", "'gb18030' codec can't decode bytes in position 0-2: incomplete multibyte sequence"},
		{"dec", "gb18030", "replace", "\x81\x30\x81", "\ufffd", ""},
		{"dec", "euc_jp", "strict", "\x8f\xa2\xaf\x8f\xa2", "", "'euc_jp' codec can't decode bytes in position 3-4: incomplete multibyte sequence"},
		{"dec", "euc_jp", "replace", "\x8f\xa2\xaf\x8f\xa2", "\u02d8\ufffd", ""},
		{"dec", "euc_jis_2004", "strict", "\xa4\xf7\x8f\xfe\xf6", "\u304b\u309a\U0002a6b2", ""},
		{"dec", "euc_jis_2004", "replace", "\xa4\xf7\x8f\xfe\xf6", "\u304b\u309a\U0002a6b2", ""},
		{"dec", "shift_jis", "strict", "\x80\xa0\xfd", "", "'shift_jis' codec can't decode byte 0x80 in position 0: illegal multibyte sequence"},
		{"dec", "shift_jis", "replace", "\x80\xa0\xfd", "\ufffd\ufffd\ufffd", ""},
		{"dec", "big5hkscs", "strict", "\x88\x62\x88\x64\x88\xa5", "\u00ca\u0304\u00ca\u030c\u00ea\u030c", ""},
		{"dec", "big5hkscs", "replace", "\x88\x62\x88\x64\x88\xa5", "\u00ca\u0304\u00ca\u030c\u00ea\u030c", ""},
		{"enc", "sjis", "strict", "\u4e2d", "\x92\x86", ""},
		{"enc", "ms_kanji", "strict", "\u4e2d", "\x92\x86", ""},
		{"enc", "eucjp", "strict", "\u4e2d", "\xc3\xe6", ""},
		{"enc", "ms932", "strict", "\u4e2d", "\x92\x86", ""},
		{"enc", "big5-hkscs", "strict", "\u4e2d", "\xa4\xa4", ""},
		{"enc", "cp936", "strict", "\u4e2d", "\xd6\xd0", ""},
		{"enc", "uhc", "strict", "\u4e2d", "\xf1\xe9", ""},
		{"enc", "ks_c-5601", "strict", "\u4e2d", "\xf1\xe9", ""},
		{"enc", "csbig5", "strict", "\u4e2d", "\xa4\xa4", ""},
		{"enc", "s_jis_2004", "strict", "\u4e2d", "\x92\x86", ""},
		{"enc", "eucjisx0213", "strict", "\u4e2d", "\xc3\xe6", ""},
		{"enc", "chinese", "strict", "\u4e2d", "\xd6\xd0", ""},
		{"enc", "gb-18030", "strict", "\u4e2d", "", "unknown encoding: gb-18030"},
		{"enc", "Shift-JIS", "strict", "\u4e2d", "\x92\x86", ""},
	}
	for _, c := range cases {
		var got string
		var err error
		if c.op == "enc" {
			var b []byte
			b, err = pyEncode(c.in, c.codec, c.errors)
			got = string(b)
		} else {
			got, err = pyDecode([]byte(c.in), c.codec, c.errors)
		}
		switch {
		case c.err != "" && (err == nil || err.Error() != c.err):
			t.Errorf("%s %s(%q, %s): got %q, %v; want error %q", c.op, c.codec, c.in, c.errors, got, err, c.err)
		case c.err == "" && err != nil:
			t.Errorf("%s %s(%q, %s): unexpected error %v", c.op, c.codec, c.in, c.errors, err)
		case c.err == "" && got != c.want:
			t.Errorf("%s %s(%q, %s) = %q, want %q", c.op, c.codec, c.in, c.errors, got, c.want)
		}
	}
}

// TestCJKTablesParse builds every codec from the embedded tables.
func TestCJKTablesParse(t *testing.T) {
	cjkOnce.Do(indexCJK)
	if len(cjkBodies) != 16 {
		t.Fatalf("got %d CJK codecs, want 16", len(cjkBodies))
	}
	for name := range cjkBodies {
		c, ok := cjkLookup(name)
		if !ok || len(c.units) == 0 || len(c.encode) == 0 {
			t.Errorf("%s: empty or missing tables", name)
		}
	}
}
