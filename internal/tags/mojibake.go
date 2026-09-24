package tags

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// Encoding names reported by FixMojibake.
const (
	EncodingUTF8     = "utf-8"     // UTF-8 bytes that were decoded as Latin-1 / Windows-1252
	EncodingGBK      = "gbk"       // GBK / GB2312 (simplified Chinese)
	EncodingBig5     = "big5"      // Big5 (traditional Chinese)
	EncodingShiftJIS = "shift_jis" // Shift-JIS (Japanese)
)

// minLegacyScore is the minimum average plausibility (see pairScore) a legacy CJK decoding
// needs to be accepted: on average every character must at least come from the standard
// (non-extension) area of its code page.
const minLegacyScore = 1.0

// FixMojibake repairs text that was decoded with the wrong code page. It is deliberately
// strict and conservative — a false positive would corrupt correct metadata:
//
//   - UTF-8 decoded as Latin-1/Windows-1252 ("æ—¥æœ¬" → "日本"): every rune maps back to a
//     single byte, those bytes are valid UTF-8 containing a multi-byte sequence and every
//     decoded character is plausible (see plausibleDecodedRune: no control, IPA, Latin
//     Extended-B except Vietnamese / pinyin / Romanian letters, rare combining marks, or
//     Greek / Cyrillic / Hebrew / Arabic glued to a Latin letter), so that an accented
//     capital followed by punctuation ("CAFÉ–BAR", "OLÉ…") is left alone.
//   - GBK, Big5 or Shift-JIS decoded as Latin-1 ("ÄãºÃ" → "你好"): only when every rune is
//     ≤ U+00FF, there are at least two high bytes, every high byte is part of a valid
//     double-byte character (no replacement characters, no half-width katakana, no 4-byte
//     GB18030 sequences), the number of ASCII characters is unchanged — an ASCII byte may
//     only be consumed as a trail byte after a C1 control lead byte (0x81–0x9F), which never
//     occurs in genuine Latin-1 text — every decoded character is CJK / kana / CJK
//     punctuation, and the result contains CJK or kana. A single decoded character is only
//     accepted when the text has no ASCII letters ("Größe" stays untouched) and it is not
//     made of two Latin-1 symbols ("«»", "12 ½½"), and decodings
//     where every character stands alone next to an ASCII letter — the shape of accented
//     Latin words such as "Coração, coração" or "«Été»" — are rejected. When several
//     code pages qualify, the most plausible one (common characters, standard code-page
//     areas, kana) wins.
//
// So "Björk", "Café" and "Beyoncé" are never changed. It returns the fixed text, the
// detected encoding (EncodingUTF8, EncodingGBK, EncodingBig5, EncodingShiftJIS) and whether
// anything changed.
func FixMojibake(s string) (fixed string, encoding string, changed bool) {
	if s == "" || isASCII(s) {
		return s, "", false
	}
	if f, ok := fixDoubleUTF8(s); ok {
		return f, EncodingUTF8, true
	}
	if f, enc, ok := fixLegacyCJK(s); ok {
		return f, enc, true
	}
	return s, "", false
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// fixDoubleUTF8 undoes UTF-8 → Latin-1/Windows-1252 → UTF-8 double encoding.
func fixDoubleUTF8(s string) (string, bool) {
	buf := make([]byte, 0, len(s))
	high := 0
	for _, r := range s {
		switch {
		case r == utf8.RuneError:
			return "", false
		case r <= 0xFF:
			buf = append(buf, byte(r))
		default:
			b, ok := charmap.Windows1252.EncodeRune(r)
			if !ok {
				return "", false
			}
			buf = append(buf, b)
		}
		if r >= 0x80 {
			high++
		}
	}
	if high < 2 || !utf8.Valid(buf) {
		return "", false
	}
	out := string(buf)
	if out == s || isASCII(out) {
		return "", false
	}
	runes := []rune(out)
	for i, r := range runes {
		if r == utf8.RuneError || (unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t') {
			return "", false
		}
		if r < utf8.RuneSelf {
			continue
		}
		prev, next := rune(0), rune(0)
		if i > 0 {
			prev = runes[i-1]
		}
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		if !plausibleDecodedRune(r, prev, next) {
			return "", false
		}
	}
	return out, true
}

// plausibleDecodedRune reports whether r, a non-ASCII character produced by undoing a
// double UTF-8 encoding, is something real text contains. An upper-case accented Latin-1
// letter (bytes 0xC2–0xDF are UTF-8 lead bytes) followed by Windows-1252 punctuation
// (0x80–0xBF are continuation bytes) — "CAFÉ–BAR", "OLÉ…", "JOSÉ’S", "Fuß“" — is valid
// UTF-8 too, but decodes to IPA / Latin Extended-B / NKo / Arabic characters glued to a
// Latin word, which genuine mojibake of real text never produces.
func plausibleDecodedRune(r, prev, next rune) bool {
	switch {
	case r >= 0xA0 && r <= 0x17F: // Latin-1 Supplement, Latin Extended-A
		return true
	case r == 0x1A0, r == 0x1A1, r == 0x1AF, r == 0x1B0, // Vietnamese horns (ơ, ư)
		r >= 0x1CD && r <= 0x1DC, // pinyin tone marks (ǎ, ǐ, ǒ, ǔ, ǖ…)
		r >= 0x218 && r <= 0x21B: // Romanian comma below (ș, ț)
		return true
	case r < 0x300: // other Latin Extended-B, IPA, spacing modifier letters
		return false
	case r <= 0x36F: // combining diacritical marks: only the usual accents of NFD text
		return commonCombiningMark(r) && unicode.IsLetter(prev)
	case r < 0x800:
		// Greek, Cyrillic, Armenian, Hebrew, Arabic — but never glued to a Latin letter
		// ("SØ¨" is not "Sب"); Syriac, Thaana, NKo and format characters never.
		if !unicode.In(r, unicode.Greek, unicode.Cyrillic, unicode.Armenian, unicode.Hebrew, unicode.Arabic) {
			return false
		}
		if unicode.IsMark(r) { // script-specific points / harakat: only after a letter of that script
			return prev >= 0x370 && unicode.IsLetter(prev)
		}
		if !unicode.IsLetter(r) && !unicode.IsPunct(r) && !unicode.IsDigit(r) {
			return false
		}
		return !isASCIILetterRune(prev) && !isASCIILetterRune(next)
	}
	// Three- and four-byte sequences (CJK, kana, Hangul, punctuation, emoji …) need a
	// lower-case accented letter followed by two symbols to occur by accident.
	return unicode.IsGraphic(r) && !unicode.Is(unicode.Co, r)
}

func isASCIILetterRune(r rune) bool { return r < utf8.RuneSelf && isASCIILetter(byte(r)) }

// commonCombiningMark reports whether r is one of the combining accents decomposed (NFD)
// Latin / Vietnamese text uses: grave, acute, circumflex, tilde, macron, breve, dot above,
// diaeresis, hook above, ring, double acute, caron, double grave, inverted breve, horn,
// dot / diaeresis / ring / comma below, cedilla, ogonek, circumflex / breve / tilde /
// macron below. Rarer marks (overline "COSÌ…", Greek ypogegrammeni "AQUÍ…") are what an
// accented capital followed by punctuation decodes to.
func commonCombiningMark(r rune) bool {
	switch r {
	case 0x300, 0x301, 0x302, 0x303, 0x304, 0x306, 0x307, 0x308, 0x309, 0x30A, 0x30B, 0x30C, 0x30F,
		0x311, 0x31B, 0x323, 0x324, 0x325, 0x326, 0x327, 0x328, 0x32D, 0x32E, 0x330, 0x331:
		return true
	}
	return false
}

type legacyCodec struct {
	name string
	enc  encoding.Encoding
}

// Candidates in tie-break order.
var legacyCodecs = []legacyCodec{
	{EncodingGBK, simplifiedchinese.GBK},
	{EncodingShiftJIS, japanese.ShiftJIS},
	{EncodingBig5, traditionalchinese.Big5},
}

// fixLegacyCJK tries GBK, Shift-JIS and Big5 on the Latin-1 bytes of s.
func fixLegacyCJK(s string) (string, string, bool) {
	raw := make([]byte, 0, len(s))
	high, highLetters, asciiLetters := 0, 0, false
	for _, r := range s {
		if r > 0xFF || r == utf8.RuneError {
			return "", "", false
		}
		if r >= 0x80 {
			high++
			if r >= 0xC0 && r != 0xD7 && r != 0xF7 {
				highLetters++
			}
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			asciiLetters = true
		}
		raw = append(raw, byte(r))
	}
	if high < 2 {
		return "", "", false
	}
	bestText, bestName, bestScore := "", "", 0.0
	for _, c := range legacyCodecs {
		text, score, chars, ok := decodeLegacy(raw, c)
		if !ok || score < minLegacyScore {
			continue
		}
		// A single decoded character is too ambiguous next to ASCII letters ("Größe"), or
		// when it is made of two Latin-1 symbols ("«»", "¿¡", "12 ½½").
		if chars < 2 && (asciiLetters || highLetters == 0) {
			continue
		}
		if score > bestScore {
			bestText, bestName, bestScore = text, c.name, score
		}
	}
	if bestName == "" {
		return "", "", false
	}
	return bestText, bestName, true
}

// decodeLegacy decodes raw as double-byte text in codec c. It returns the text, the average
// plausibility score of the double-byte characters, their number and whether the decoding
// satisfies every strictness rule of FixMojibake.
func decodeLegacy(raw []byte, c legacyCodec) (string, float64, int, bool) {
	dec := c.enc.NewDecoder()
	var sb strings.Builder
	total, chars := 0.0, 0
	hasCJK := false
	units := make([]unitKind, 0, len(raw))
	for i := 0; i < len(raw); {
		b := raw[i]
		if b < 0x80 {
			sb.WriteByte(b)
			if isASCIILetter(b) {
				units = append(units, unitLetter)
			} else {
				units = append(units, unitOther)
			}
			i++
			continue
		}
		if i+1 >= len(raw) {
			return "", 0, 0, false
		}
		t := raw[i+1]
		// An ASCII trail byte is only acceptable after a C1-control lead byte.
		if t < 0x80 && b > 0x9F {
			return "", 0, 0, false
		}
		out, err := dec.Bytes([]byte{b, t})
		if err != nil {
			return "", 0, 0, false
		}
		r, size := utf8.DecodeRune(out)
		if size != len(out) || r == utf8.RuneError || !allowedCJKRune(r) {
			return "", 0, 0, false
		}
		if isCJKOrKana(r) {
			hasCJK = true
		}
		total += pairScore(c.name, b, t, r)
		chars++
		units = append(units, unitDouble)
		sb.WriteRune(r)
		i += 2
	}
	if chars == 0 || !hasCJK || latinWordShape(units) {
		return "", 0, 0, false
	}
	return sb.String(), total / float64(chars), chars, true
}

// unitKind classifies the units of a legacy decoding: an ASCII letter, another ASCII
// character or a double-byte character.
type unitKind uint8

const (
	unitLetter unitKind = iota
	unitOther
	unitDouble
)

func isASCIILetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }

// latinWordShape reports whether a decoding has the shape of accented Latin words rather
// than of CJK text: every double-byte character stands alone (no two in a row) and touches
// an ASCII letter. Pairs of accented letters inside words ("Coração, coração",
// "CORAÇÃO E CORAÇÃO") and accented letters next to guillemets ("«Été»") decode to valid
// GBK / Big5 characters, but genuine CJK mojibake has runs of several characters or
// characters that are separated from Latin words.
func latinWordShape(units []unitKind) bool {
	for i, u := range units {
		if u != unitDouble {
			continue
		}
		prevDouble := i > 0 && units[i-1] == unitDouble
		nextDouble := i+1 < len(units) && units[i+1] == unitDouble
		if prevDouble || nextDouble {
			return false
		}
		prevLetter := i > 0 && units[i-1] == unitLetter
		nextLetter := i+1 < len(units) && units[i+1] == unitLetter
		if !prevLetter && !nextLetter {
			return false
		}
	}
	return true
}

// allowedCJKRune reports whether r may appear in a repaired string: CJK ideographs, kana,
// bopomofo, CJK / full-width punctuation and common general punctuation. Half-width
// katakana, Latin, Greek, Cyrillic, box drawing and private-use characters are rejected.
func allowedCJKRune(r rune) bool {
	switch {
	case r >= 0xFF61 && r <= 0xFF9F: // half-width katakana
		return false
	case isCJKOrKana(r):
		return true
	case r >= 0x3000 && r <= 0x303F: // CJK symbols and punctuation
		return true
	case r >= 0xFF01 && r <= 0xFF5E, r >= 0xFFE0 && r <= 0xFFE6: // full-width forms
		return true
	case r >= 0x2010 && r <= 0x2027, r >= 0x2030 && r <= 0x203B: // dashes, quotes, ellipsis, ‰, ※
		return true
	case r == 0x00B7, r == 0x00D7, r == 0x00F7, r == 0x30FB, r == 0x2605, r == 0x2606, r == 0x266A:
		return true
	case unicode.Is(unicode.Bopomofo, r):
		return true
	}
	return false
}

func isCJKOrKana(r rune) bool {
	if r >= 0xFF61 && r <= 0xFF9F {
		return false
	}
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r)
}

// pairScore rates how plausible a decoded double-byte character is for its code page:
// 3 for very common characters (and kana in Shift-JIS), 1.5 for the primary area of the
// code page, 0.8 for its punctuation rows, 0.7 for secondary areas and 0.1 for extension /
// rarely used areas.
func pairScore(codec string, b1, _ byte, r rune) float64 {
	switch codec {
	case EncodingGBK:
		switch {
		case commonHan[r]:
			return 3
		case b1 >= 0xB0 && b1 <= 0xD7: // GB2312 level 1
			return 1.5
		case b1 >= 0xA1 && b1 <= 0xA9: // GB2312 symbols, kana
			return 0.8
		case b1 >= 0xD8 && b1 <= 0xF7: // GB2312 level 2
			return 0.7
		}
	case EncodingBig5:
		switch {
		case commonHan[r]:
			return 3
		case b1 >= 0xA4 && b1 <= 0xC6: // frequently used characters
			return 1.5
		case b1 >= 0xA1 && b1 <= 0xA3: // symbols, bopomofo
			return 0.8
		case b1 >= 0xC9 && b1 <= 0xF9: // less frequently used characters
			return 0.7
		}
	case EncodingShiftJIS:
		switch {
		case unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r):
			return 3
		case commonHan[r]:
			return 3
		case b1 >= 0x88 && b1 <= 0x98: // JIS level 1 kanji
			return 1.5
		case b1 == 0x81: // punctuation
			return 0.8
		case (b1 >= 0x99 && b1 <= 0x9F) || (b1 >= 0xE0 && b1 <= 0xEA): // JIS level 2 kanji
			return 0.7
		}
	}
	return 0.1
}

// commonHan holds very frequent Chinese characters (simplified and traditional forms, also
// common in Japanese) used to rank competing decodings.
var commonHan = func() map[rune]bool {
	const chars = "的一是不了在人有我他这個个们們中来來上大为為和国國地到以说說时時要就出会會可也你对對生能而子那得于於着著下自之年过過发發后後作里裡用道行所然家种種事成方多经經么麼去法学學如都同现現当當没沒动動面起看定天分还還进進好小部其些主样樣理心她本前开開但因只从從想实實日军軍者意无無力它与與长長把机機十民第公此已工使情明性知全三又关關点點正业業外将將两兩高间間由问問很最重并物手应應战戰向头頭文体體政美相见見被利什二等产產或新己制身果加西斯月话話合回特代内信表化老给給世位次度门門任常先海通教儿兒原东東声聲提立及比员員解水名真论論处處走义義各入几幾口认認条條平系气氣题題活尔爾更别別打女变變四神总總何电電数數安少报報才结結反受目太量再感建务務做接必场場件计計管期市直德资資命山金指克许許统統区區保至队隊形社便空决決治展马馬科司五基眼书書非则則听聽白却界达達光放强強即像难難且权權思王象完设設式色路记記南品住告类類求据據程北边邊死张張该該交规規万萬取拉格望觉覺术術领領共确確传傳师師观觀清今切院让讓识識候带帶导導争爭运運笑飞飛风風步改收根干造言联聯持组組每济濟车車亲親极極林服快办辦议議往元英士证證近失转轉夫令准布始怎呢存未远遠叫台单單影具罗羅字爱愛击擊流备備兵连連调調深商算质質团團集百需价價花党黨华華城石级級整府离離况況亚亞请請技际際约約示复復病息究线線似官火断斷精满滿支视視消越器容照须須九增研写寫称稱企八功吗嗎包片史委乎查轻輕易早曾除农農找装裝广廣显顯吧阿李标標谈談吃图圖念六引历歷首医醫局突专專费費号號尽盡另周较較注语語仅僅考落青随隨选選列武红紅响響虽雖推势勢参參希古众眾构構房半节節土投某案黑维維革划敌敵致陈陳律足态態护護七兴興派孩验驗责責营營星够夠章音跟志底站严嚴巴例防族供效续續施留讲講型料终終答紧緊黄黃绝絕奇察母京段依批群项項故按河米围圍江织織害斗双雙境客纪紀采举舉杀殺攻父苏蘇密低朝友诉訴止细細愿願千值仍男钱錢破网網热熱助倒育属屬坐帝限船脸臉职職速刻乐樂否刚剛威毛状狀率甚独獨球般普怕弹彈校苦创創假久错錯承印晚兰蘭试試股拿脑腦预預谁誰益阳陽若哪微尼继繼送急血惊驚伤傷素药藥适適波夜省初喜卫衛源食险險待述陆陸习習置居劳勞财財环環排福纳納欢歡雷警获獲模充负負云雲停木游遊龙龍树樹疑层層冷洲冲衝射略范竟句室异異激汉漢村哈策演简簡卡罪判担擔州静靜退既衣您宗积積余痛检檢差富灵靈协協角占配征修皮挥揮胜勝降阶階审審沉坚堅善妈媽刘劉读讀啊超免压壓银銀买買皇养養伊怀懷执執副乱亂抗犯追帮幫宣佛岁歲航优優怪香田铁鐵控税稅左右份穿艺藝背阵陣草脚腳概恶惡块塊顿頓敢守酒岛島托央户戶烈洋哥索胡款靠评評版宝寶座释釋景顾顧弟登货貨互付伯慢欧歐换換闻聞危忙核暗姐介坏壞讨討丽麗良序升监監临臨亮露永呼味野架域沙掉括舰艦鱼魚杂雜误誤湾灣吉减減编編楚肯测測败敗屋跑梦夢散温溫困剑劍渐漸封救贵貴枪槍缺楼樓县縣尚毫移娘朋画畫班智亦耳恩短掌恐遗遺固席松秘谢謝鲁魯遇康虑慮幸均销銷钟鐘诗詩藏赶趕剧劇票损損忽巨炮旧舊端探湖录錄叶葉春乡鄉附吸予礼禮港雨呀板庭妇婦归歸睛饭飯额額含顺順输輸摇搖招婚脱脫补補谓謂督毒油疗療旅泽澤材灭滅逐莫笔筆亡鲜鮮词詞圣聖择擇寻尋厂廠睡博烟煙授诺諾伦倫岸奥奧唐卖賣俄炸载載洛健堂旁宫宮喝借君禁阴陰园園谋謀宋避抓荣榮姑孙孫逃牙束跳顶頂玉镇鎮雪午练練迫爷爺篇肉嘴馆館遍凡础礎洞卷坦牛宁寧纸紙诸諸训訓私庄莊祖丝絲翻暴森塔默握戏戲隐隱熟骨访訪弱蒙歌店鬼软軟典欲萨薩伙遭盘盤爸扩擴盖蓋弄雄稳穩忘亿億刺拥擁徒姆杨楊齐齊赛賽趣曲刀床迎冰虚虛玩析窗醒妻透购購替塞努休虎扬揚途侵刑绿綠兄迅套贸貿毕畢唯谷轮輪库庫迹跡尤竞競街促延震弃棄甲伟偉麻川申缓緩潜潛闪閃售灯燈针針哲络絡抵朱埃抱鼓植纯純夏忍页頁杰傑筑築折郑鄭贝貝尊吴吳秀混臣雅振染盛怒舞圆圓搞狂措姓残殘秋培迷诚誠宽寬宇猛摆擺梅毁毀伸摩盟末乃悲拍丁赵趙硬麦麥操耶阻订訂彩抽赞讚魔纷紛沿喊违違妹浪汇匯币幣丰豐蓝藍殊献獻桌啦瓦莱萊援译譯夺奪汽烧燒距裁偏符勇触觸课課敬哭懂墙牆袭襲召罚罰侠俠厅廳拜巧侧側韩韓冒债債曼融惯慣享戴童犹猶乘挂掛奖獎绍紹厚纵縱障讯訊涉彻徹刊丈爆乌烏役描洗玛瑪患妙镜鏡唱烦煩签簽仙彼弗症仿倾傾牌陷鸟鳥轰轟咱菜闭閉奋奮庆慶撤泪淚茶疾缘緣播朗杜奶季丹狗尾仪儀偷奔珠虫蟲驻駐孔宜艾桥橋淡翼恨繁寒伴叹嘆旦愈潮粮糧缩縮罢罷聚径徑恰挑袋灰捕徐珍幕映裂泰隔启啟尖忠累炎暂暫估泛荒偿償横橫拒瑞忆憶孤鼻闹鬧羊呆厉厲衡胞零穷窮舍码碼赫婆魂灾災洪腿胆膽津俗辩辯胸晓曉劲勁贫貧仁偶辑輯邦恢赖賴圈摸仰润潤堆碰艇稍迟遲辆輛废廢净淨凶署壁御奉旋冬矿礦抬蛋晨伏吹鸡雞倍糊秦盾杯租骑騎乏隆诊診奴摄攝丧喪污渡旗甘耐凭憑扎抢搶绪緒粗肩梁幻菲皆碎宙叔岩荡蕩综綜爬荷悉蒂返井壮壯薄悄扫掃敏碍礙殖详詳迪矛霍允幅撒剩凯凱颗顆骂罵赏賞液番箱贴貼漫酸郎腰舒眉忧憂浮辛恋戀餐吓嚇挺励勵辞辭艘键鍵伍峰尺昨黎辈輩贯貫侦偵滑券崇扰擾宪憲绕繞趋趨慈乔喬阅閱汗枝拖墨胁脅插箭腊臘粉泥氏彭拔骗騙凤鳳慧媒佩愤憤扑撲龄齡驱驅惜豪掩兼跃躍尸屍肃肅帕驶駛堡届屆欣惠册冊储儲飘飄桑闲閒惨慘洁潔踪蹤勃宾賓频頻仇磨递遞邪撞拟擬滚滾奏巡颜顏剂劑绩績贡貢疯瘋坡瞧截燃焦殿伪偽柳锁鎖逼颇頗昏劝勸呈搜勤戒驾駕漂饮飲曹朵仔柔俩倆孟腐幼践踐籍牧凉涼牲佳娜浓濃芳稿竹腹跌逻邏垂遵脉脈貌柏狱獄猜怜憐惑陶兽獸帐帳饰飾贷貸昌叙敘躺钢鋼沟溝寄扶铺鋪邓鄧寿壽惧懼询詢汤湯盗盜肥尝嘗匆辉輝奈扣廷澳嘛董迁遷凝慰厌厭脏髒腾騰幽怨鞋丢丟埋泉涌躲晋晉紫艰艱魏吾慌祝邮郵吐狠鉴鑑曰械咬邻鄰赤挤擠弯彎椅陪割揭韦韋悟聪聰雾霧锋鋒梯猫貓祥阔闊誉譽筹籌丛叢牵牽鸣鳴沈阁閣穆屈旨袖猎獵臂蛇贺賀柱抛拋鼠瑟戈牢逊遜迈邁欺吨噸琴衰瓶恼惱燕仲诱誘狼池疼卢盧仗冠粒遥遙吕呂玄尘塵冯馮抚撫浅淺敦纠糾钻鑽晶岂豈峡峽苍蒼喷噴耗凌敲菌赔賠涂塗粹扁亏虧寂煤熊恭湿濕循暖糖赋賦抑秩帽哀宿踏烂爛袁侯抖夹夾昆肝擦猪豬炼煉恒恆慎搬纽紐纹紋玻渔漁磁铜銅齿齒跨押怖漠疲叛遣兹茲祭醉拳弥彌斜档檔稀捷肤膚疫肿腫豆削岗崗晃吞宏癌肚隶隸履涨漲耀扭坛壇拨撥沃绘繪伐堪仆郭牺犧歼殲墓雇廉契拼惩懲捉覆刷劫嫌瓜歇雕闷悶乳串娃缴繳唤喚赢贏莲蓮霸桃妥瘦搭赴岳嘉舱艙俊址庞龐耕锐銳缝縫悔邀玲惟斥宅添挖呵讼訟氧浩羽斤酷掠妖祸禍侍乙妨贪貪挣掙汪尿莉悬懸唇翰仓倉轨軌枚盐鹽览覽傅帅帥庙廟芬屏寺胖璃愚滴疏萧蕭姿颤顫丑醜劣柯寸扔盯辱匹俱辨饿餓蜂哦腔郁鬱溃潰谨謹糟葛苗肠腸忌溜鸿鴻爵鹏鵬鹰鷹笼籠丘桂滋聊挡擋纲綱肌茨壳殼痕碗穴膀卓贤賢卧臥膜毅锦錦欠哩函茫昂薛皱皺夸誇豫胃舌剥剝傲拾窝窩睁睜携攜陵哼棉晴铃鈴填饲飼渴吻扮逆脆喘罩卜炉爐柴愉绳繩胎蓄眠竭喂傻慕浑渾奸扇柜櫃悦悅拦攔诞誕饱飽乾泡贼賊亭夕爹酬儒姻卵氛泄杆挨僧蜜吟猩遂狭狹肖甜霞驳駁裕顽頑摘矮秒卿畜咽披辅輔勾盆疆赌賭塑畏吵囊嗯泊肺骤驟缠纏冈岡羞瞪吊贾賈漏斑涛濤悠鹿俘锡錫卑葬铭銘滩灘嫁催璇翅盒蛮蠻矣潘歧赐賜鲍鮑锅鍋廊拆灌勉盲宰佐啥胀脹扯禧辽遼抹筒棋裤褲唉朴咐孕誓喉妄拘链鏈驰馳栏欄逝窃竊艳豔臭纤纖棵趁匠盈翁愁瞬婴嬰孝颈頸倘浙谅諒蔽畅暢赠贈妮莎尉冻凍跪闯闖葡厨廚鸭鴨颠顛遮谊誼吁仑侖辟瘤嫂陀框谭譚亨钦欽庸歉芝吼甫衫摊攤宴嘱囑衷娇嬌陕陝矩浦讶訝耸聳裸碧摧薪淋耻恥胶膠屠鹅鵝饥飢盼脖虹翠崩账帳萍逢赚賺撑撐翔倡绵綿猴枯巫昭怔渊淵凑湊溪蠢禅禪阐闡旺寓藤匪伞傘碑挪琼瓊脂谎謊慨菩萄狮獅掘抄岭嶺晕暈逮砍掏狄晰罕挽脾舟痴蔡剪脊弓懒懶叉拐喃僚捐姊骚騷拓歪粘柄坑陌窄湘兆崖骄驕刹剎鞭芒筋聘钩鉤棍嚷腺弦焰耍俯厘愣厦廈恳懇饶饒钉釘寡憾摔叠疊惹喻谱譜愧煌徽溶坠墜煞巾滥濫洒灑堵瓷咒姨棒郡浴媚稣穌淮哎屁漆淫巢吩撰啸嘯滞滯玫硕碩钓釣蝶膝姚茂躯軀吏猿寨恕渠戚辰舶颁頒惶狐讽諷笨袍嘲啡泼潑衔銜倦涵雀旬僵撕肢垄壟夷逸茅侨僑舆輿窑窯涅蒲谦謙杭噢弊勋勳刮郊凄捧浸砖磚鼎篮籃蒸饼餅亩畝肾腎陡爪兔殷贞貞荐薦哑啞炭坟墳眨搏咳拢攏舅昧擅爽咖搁擱禄祿雌哨巩鞏绢絹螺裹昔轩軒谬謬谍諜龟龜媳姜瞎冤鸦鴉蓬巷琳栽沾诈詐斋齋瞒瞞彪厄咨纺紡罐桶壤糕颂頌膨谐諧垒壘咕隙辣绑綁宠寵嘿兑兌霉挫稽辐輻乞纱紗裙嘻哇绣繡杖塘衍轴軸攀膊譬斌祈踢肆坎轿轎棚泣屡屢躁邱凰溢椎砸趟帘簾帆栖棲窜竄丸斩斬堤塌贩販厢廂掀喀乖谜謎捏阎閻滨濱虏虜匙芦蘆苹蘋卸沼钥鑰株祷禱剖熙哗嘩劈怯棠胳桩樁瑰娱娛娶沫嗓蹲焚淘嫩韵韻衬襯匈钧鈞竖豎峻豹捞撈菊鄙魄兜哄颖穎镑鎊屑蚁蟻壶壺怡渗滲秃禿迦旱哟喲咸焉谴譴宛稻铸鑄锻鍛伽詹毙斃恍贬貶烛燭骇駭芯汁桓坊驴驢朽靖佣傭汝碌迄冀荆荊崔雁绅紳珊榜诵誦傍彦彥醇笛禽勿娟瞄幢寇睹贿賄踩霆呜嗚拱妃蔑谕諭缚縛诡詭篷淹腕煮倩卒勘馨逗甸贱賤炒灿燦敞蜡蠟囚栗辜垫墊妒魁谣謠寞蜀甩涯枕丐泳奎泌逾叮黛燥掷擲藉枢樞憎鲸鯨弘倚侮藩拂鹤鶴蚀蝕浆漿芙垃烤晒曬霜剿蕴蘊圾绸綢屿嶼氢氫驼駝妆妝捆铅鉛逛淑榴丙痒癢钞鈔蹄犬躬昼晝藻蛛褐颊頰奠募耽蹈陋侣侶魅岚嵐侄虐堕墮陛莹瑩荫蔭狡阀閥绞絞膏垮茎莖缅緬喇绒絨搅攪凳梭丫姬诏詔钮鈕棺耿缔締懈嫉灶竈匀勻嗣鸽鴿澡凿鑿纬緯沸畴疇刃遏烁爍嗅叭熬瞥骸奢拙栋棟毯桐砂莽泻瀉坪梳杉晤稚蔬蝇蠅捣搗顷頃尴尷镖鏢诧詫尬硫嚼羡羨沦淪沪滬旷曠彬芽狸冥碳咧惕暑咯萝蘿汹洶腥窥窺俺潭崎麟捡撿拯厥澄萎哉涡渦滔暇溯鳞鱗酿釀茵愕瞅暮衙诫誡斧兮焕煥棕佑嘶妓喧蓉删刪樱櫻伺嗡娥梢坝壩蚕蠶敷澜瀾杏绥綏冶庇挠撓搂摟倏聂聶婉噪稼鳍鰭菱盏盞匿吱寝寢揽攬髓秉哺矢啪帜幟邵嗽挟挾缸揉腻膩驯馴缆纜晌瘫癱贮貯觅覓朦僻隋蔓咋嵌虔畔琐瑣碟涩澀胧朧嘟蹦冢浏瀏裔襟叨诀訣旭虾蝦簿啤擒枣棗嘎苑牟呕嘔骆駱凸熄兀喔裳凹赎贖屯膛浇澆灼裘砰棘橡碱聋聾姥瑜毋娅婭沮萌俏黯撇粟粪糞尹苟癫癲蚂螞禹廖俭儉帖煎缕縷窦竇簇棱叩呐吶瑶瑤墅莺鶯烫燙蛙歹伶葱蔥哮眩坤廓讳諱啼乍瓣矫矯跋枉梗厕廁琢讥譏釉窟敛斂轼軾庐廬胚呻绰綽扼懿炯竿慷虞锤錘栓桨槳蚊磅孽惭慚戳禀稟鄂馈饋垣溅濺咚钙鈣礁彰豁眯磷雯墟迂瞻颅顱琉悼蝴拣揀渺眷悯憫汰慑懾婶嬸斐嘘噓镶鑲炕宦趴绷繃窘襄珀嚣囂拚酌浊濁毓撼嗜扛峭磕翘翹槽淌栅柵颓頹熏瑛颐頤忖恋恋戀夢梦歌詞词君僕私雨空星夜風花雪桜桜" +
		"愛曲唄声聲涙泪笑顔顏想思出逢会會街夏秋冬春朝昼夕月光影夢恋戀歌謡"
	m := make(map[rune]bool, 3000)
	for _, r := range chars {
		m[r] = true
	}
	return m
}()
