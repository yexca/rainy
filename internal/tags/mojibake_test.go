package tags

import (
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// latin1Mojibake encodes s with enc and decodes the bytes as ISO-8859-1, reproducing what a
// tag reader does with legacy-encoded ID3v1 / ID3v2 Latin-1 frames.
func latin1Mojibake(t *testing.T, enc encoding.Encoding, s string) string {
	t.Helper()
	b, err := enc.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatalf("encoding %q: %v", s, err)
	}
	var sb strings.Builder
	for _, c := range b {
		sb.WriteRune(rune(c))
	}
	return sb.String()
}

func cp1252Mojibake(t *testing.T, s string) string {
	t.Helper()
	out, err := charmap.Windows1252.NewDecoder().String(s)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFixMojibakeRepairs(t *testing.T) {
	tests := []struct {
		name, in, want, enc string
	}{
		{"gbk title", latin1Mojibake(t, simplifiedchinese.GBK, "你好世界"), "你好世界", EncodingGBK},
		{"gbk artist", latin1Mojibake(t, simplifiedchinese.GBK, "周杰伦"), "周杰伦", EncodingGBK},
		{"gbk with ascii", latin1Mojibake(t, simplifiedchinese.GBK, "01 夏日微风 (Live)"), "01 夏日微风 (Live)", EncodingGBK},
		{"gbk single char no letters", latin1Mojibake(t, simplifiedchinese.GBK, "雨"), "雨", EncodingGBK},
		{"gbk punctuation", latin1Mojibake(t, simplifiedchinese.GBK, "晚安，夏天"), "晚安，夏天", EncodingGBK},
		{"big5", latin1Mojibake(t, traditionalchinese.Big5, "中文"), "中文", EncodingBig5},
		{"big5 two chars", latin1Mojibake(t, traditionalchinese.Big5, "晴天"), "晴天", EncodingBig5},
		{"shift-jis hiragana", latin1Mojibake(t, japanese.ShiftJIS, "こんにちは"), "こんにちは", EncodingShiftJIS},
		{"shift-jis katakana ascii trail", latin1Mojibake(t, japanese.ShiftJIS, "アイ"), "アイ", EncodingShiftJIS},
		{"shift-jis mixed", latin1Mojibake(t, japanese.ShiftJIS, "雨上がりの空"), "雨上がりの空", EncodingShiftJIS},
		{"gbk ascii inside", latin1Mojibake(t, simplifiedchinese.GBK, "卡拉OK之夜"), "卡拉OK之夜", EncodingGBK},
		{"gbk glued to latin word", latin1Mojibake(t, simplifiedchinese.GBK, "Live版 现场"), "Live版 现场", EncodingGBK},
		{"shift-jis after latin", latin1Mojibake(t, japanese.ShiftJIS, "Mr.Childrenのベスト"), "Mr.Childrenのベスト", EncodingShiftJIS},
		{"utf-8 as latin-1", latin1Mojibake(t, encoding.Nop, "été"), "été", EncodingUTF8},
		{"utf-8 as cp1252", cp1252Mojibake(t, "日本語"), "日本語", EncodingUTF8},
		{"utf-8 cjk as latin-1", latin1Mojibake(t, encoding.Nop, "林雨晴"), "林雨晴", EncodingUTF8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, enc, changed := FixMojibake(tt.in)
			if !changed || got != tt.want || enc != tt.enc {
				t.Fatalf("FixMojibake(%q) = %q, %q, %v; want %q, %q, true", tt.in, got, enc, changed, tt.want, tt.enc)
			}
		})
	}
}

func TestFixMojibakeLeavesCorrectTextAlone(t *testing.T) {
	for _, s := range []string{
		"", "Hello World", "Björk", "Café", "Beyoncé", "Mötley Crüe", "Sigur Rós – Ágætis byrjun",
		"Größe", "Hæð", "naïve", "Ñandú", "ÀÉÎÕÜ", "Motörhead", "Queensrÿche", "Blue Öyster Cult",
		"Æ", "£5 ¥10", "½ ¼", "你好", "夏日微风", "雨上がりの空", "Доброе утро", "Ελληνικά",
		"Pokémon ♪", "Ø", "Émilie Simon", "Jóhann Jóhannsson", "Ólafur Arnalds", "Åsa", "Dvořák",
		"Straße", "déjà vu", "Crème brûlée", "façade", "El Niño", "São Paulo", "Łódź",
		// Adversarial: pairs of accented letters that are valid GBK / Big5 / Shift-JIS
		// double-byte sequences, guillemets, all-caps words, mixed scripts and emoji.
		"Coração, coração", "CORAÇÃO E CORAÇÃO", "Corações partidos, corações", "«Été»",
		"« Été indien »", "SÃO PAULO", "ÉTÉ", "NAÏVE", "SCHÖNE GRÜßE", "Ævintýri", "Þórður",
		"Hólmfríður", "Mañana", "¿Qué?", "¡Olé!", "© 2004 Sony", "½ ×", "Motörhead ♠",
		"Jónsi & Alex – Riceboy Sleeps", "Sigur Rós — Hoppípolla", "Beyoncé 💃", "🎵 Café",
		"周杰伦 Jay Chou", "Jay Chou 周杰伦 - 晴天", "宇多田ヒカル First Love", "YOASOBI「夜に駆ける」",
		"🌧️ 雨の日", "Café del Mar ☕ 2024", "Ölüdeniz", "Ça ira", "Où es-tu?",
		// A lone pair of Latin-1 symbols is punctuation, not a CJK character.
		"«»", "¿¡", "12 ½½", "»»", "°°", "©®", "±°", "— «» —",
	} {
		if got, enc, changed := FixMojibake(s); changed || got != s || enc != "" {
			t.Errorf("FixMojibake(%q) = %q, %q, %v; want unchanged", s, got, enc, changed)
		}
	}
}

// Every pair of accented Latin-1 letters used inside words ("…çã…" twice, as in "Coração,
// coração") decodes to some valid double-byte character in GBK, Big5 or Shift-JIS; none
// may be "repaired".
func TestFixMojibakeAccentPairsInWords(t *testing.T) {
	var letters []rune
	for r := rune(0xC0); r <= 0xFF; r++ {
		if r != 0xD7 && r != 0xF7 {
			letters = append(letters, r)
		}
	}
	for _, a := range letters {
		for _, b := range letters {
			pair := string([]rune{a, b})
			for _, s := range []string{
				"Cora" + pair + "o, cora" + pair + "o",
				"CORA" + pair + "O E CORA" + pair + "O",
				"«" + string(a) + "t" + string(b) + "»",
				"Ab" + pair + " cd" + pair,
			} {
				if got, enc, changed := FixMojibake(s); changed {
					t.Fatalf("FixMojibake(%q) = %q (%s); want unchanged", s, got, enc)
				}
			}
		}
	}
}

func TestFixMojibakeRejectsPartialDecodes(t *testing.T) {
	// A trailing lone high byte cannot be a double-byte character.
	in := latin1Mojibake(t, simplifiedchinese.GBK, "你好") + "é"
	if _, _, changed := FixMojibake(in); changed {
		t.Fatalf("FixMojibake(%q) changed text with a dangling high byte", in)
	}
	// Big5 characters whose trail byte is ASCII (夜 = A9 5D, 雨 = AB 42) could equally be an
	// accented letter followed by ASCII, so they are deliberately not repaired.
	in = latin1Mojibake(t, traditionalchinese.Big5, "城市夜雨")
	if _, _, changed := FixMojibake(in); changed {
		t.Fatalf("FixMojibake(%q) repaired text with ASCII trail bytes", in)
	}
	// A single decoded character next to ASCII letters is too ambiguous.
	in = "Ab" + latin1Mojibake(t, simplifiedchinese.GBK, "雨")
	if _, _, changed := FixMojibake(in); changed {
		t.Fatalf("FixMojibake(%q) changed single-character text with ASCII letters", in)
	}
}

func TestMetadataFixEncoding(t *testing.T) {
	m := &Metadata{
		Title:  latin1Mojibake(t, simplifiedchinese.GBK, "夏日微风"),
		Artist: "Björk",
		Genres: []string{latin1Mojibake(t, simplifiedchinese.GBK, "流行"), "Rock"},
		Raw:    map[string][]string{"TITLE": {latin1Mojibake(t, simplifiedchinese.GBK, "夏日微风")}},
	}
	raw := m.Raw["TITLE"][0]
	m.fixEncoding()
	if m.Title != "夏日微风" || m.Artist != "Björk" || m.Genres[0] != "流行" || m.Genres[1] != "Rock" {
		t.Fatalf("fixEncoding: %+v", m)
	}
	if m.Raw["TITLE"][0] != raw {
		t.Fatal("fixEncoding must not touch Raw")
	}
}

// An upper-case accented letter (a UTF-8 lead byte 0xC2–0xDF in Latin-1) followed by
// Windows-1252 punctuation (a continuation byte 0x80–0xBF) forms a valid 2-byte UTF-8
// sequence; such correct text must not be "repaired" into IPA / Latin Extended-B / NKo /
// Arabic characters glued to Latin words.
func TestFixMojibakeUpperCaseAccentBeforePunctuation(t *testing.T) {
	for _, s := range []string{
		"CAFÉ–BAR", "OLÉ…", "JOSÉ’S BAND", "PASSÉ…", "Fuß“", "SØ…", "CAFÉ—LIVE", "ÉTÉ • 2024",
		"TOUT EST CALMÉ…", "RAÚL’S", "AQUÍ…", "COSÌ…", "PERCHÈ…", "MÍ”", "CAFÉ–BAR & OLÉ…", "NIÑO“", "HÅKAN–LIVE", "Ö—Ö",
	} {
		if got, enc, changed := FixMojibake(s); changed || got != s {
			t.Errorf("FixMojibake(%q) = %q, %q, %v; want unchanged", s, got, enc, changed)
		}
	}
}

func TestFixMojibakeDoubleUTF8Scripts(t *testing.T) {
	for _, want := range []string{
		"Спасибо", "DJ Смаш – Moscow Never Sleeps", "Người lạ ơi", "Sơn Tùng M-TP", "Ελληνικά",
		"שלום", "Beyoncé 💃", "Don’t Stop", "Ștefan Bănică", "Édith Piaf", "Ångström",
		"Mötley Crüe", "Å", "pīnyīn ǎ", "été",
	} {
		in := cp1252C1Mojibake(want)
		if got, enc, changed := FixMojibake(in); !changed || got != want || enc != EncodingUTF8 {
			t.Errorf("FixMojibake(%q) = %q, %q, %v; want %q", in, got, enc, changed, want)
		}
	}
}

// cp1252C1Mojibake decodes the UTF-8 bytes of s as Windows-1252, mapping the five bytes
// that code page leaves undefined (0x81, 0x8D, 0x8F, 0x90, 0x9D) to the C1 controls as
// Windows and most tag editors do.
func cp1252C1Mojibake(s string) string {
	var sb strings.Builder
	for _, b := range []byte(s) {
		r := charmap.Windows1252.DecodeByte(b)
		if r == utf8.RuneError {
			r = rune(b)
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// Exhaustive: an upper-case accented letter whose UTF-8 reading is not a Latin letter,
// followed by any Windows-1252 punctuation / symbol, inside or at the end of a Latin word.
func TestFixMojibakeAccentThenSymbolExhaustive(t *testing.T) {
	for lead := byte(0xC6); lead <= 0xDF; lead++ {
		for cont := 0x80; cont <= 0xBF; cont++ {
			a, p := charmap.Windows1252.DecodeByte(lead), charmap.Windows1252.DecodeByte(byte(cont))
			if p == utf8.RuneError {
				continue
			}
			decoded, _ := utf8.DecodeRune([]byte{lead, byte(cont)})
			if plausibleLatin := (decoded >= 0x1A0 && decoded <= 0x1B0) || (decoded >= 0x1CD && decoded <= 0x1DC) ||
				(decoded >= 0x218 && decoded <= 0x21B) || commonCombiningMark(decoded); plausibleLatin {
				continue // ơ, ư, ǎ, ș, NFD accents: genuine mojibake patterns ("Æ°" = "ư", "eÌ0081" = "é")
			}
			for _, s := range []string{"AB" + string(a) + string(p) + "CD", "CAF" + string(a) + string(p), "AB" + string(a) + string(p) + " x"} {
				if got, _, changed := FixMojibake(s); changed {
					t.Errorf("FixMojibake(%q) = %q; want unchanged", s, got)
				}
			}
		}
	}
}
