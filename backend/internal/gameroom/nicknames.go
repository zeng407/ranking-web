package gameroom

import (
	"crypto/rand"
	"strings"
	"unicode/utf8"
)

// Starting display names, ported from resources/lang/*/nicknames.php.
//
// The word lists are the site's own, kept verbatim so a player who has joined a room
// before sees the same kind of name they did under Laravel. Two words rather than a
// number: a room full of new players reads as a cast rather than as "Player 4193".
// Collisions inside one room are possible and harmless — the leaderboard is keyed on the
// player digest, not the name.

// nicknameWords is one language's halves. Laravel joined them with no separator and so
// does this, which is why the English list produces "LovelyPanther".
type nicknameWords struct {
	adjectives []string
	names      []string
}

// nicknamesByLocale is keyed by the locales the SPA uses. A locale with no list falls
// back to defaultNicknameLocale rather than to a generic name, so an unexpected
// Accept-Language still yields a real nickname.
var nicknamesByLocale = map[string]nicknameWords{
	"zh_TW": {
		adjectives: []string{
			"敏捷", "勇敢", "狡猾", "弱小", "優雅", "兇猛", "優美", "謙遜", "聰明", "快樂", "善良", "忠誠", "雄偉", "靈巧", "有趣",
			"文靜", "任性", "強壯", "寧靜", "獨特", "自大", "智慧", "可愛", "懶惰", "熱情", "頑皮", "無能", "頑固", "豪邁", "天真",
			"細心", "謹慎", "陰沉", "陰險", "卑鄙", "貪心", "虛偽", "糊塗", "愚昧",
		},
		names: []string{
			"小貓", "小狗", "狐狸", "小熊", "兔子", "小鹿", "老虎", "大象", "長頸鹿", "斑馬", "河馬", "犀牛", "熊貓", "無尾熊", "樹懶",
			"水獺", "猴子", "鸚鵡", "猴子", "青蛙", "烏龜", "小豹", "駱駝", "鯨魚", "海豚", "蝴蝶", "蜜蜂", "章魚", "鯊魚", "企鵝",
			"蟑螂", "螞蟻", "烏龜", "蛇蛇", "鴕鳥", "鱷魚", "猩猩", "海星", "海馬", "袋鼠", "老鼠", "食蟻獸", "鴨嘴獸", "獨角獸", "羊駝",
			"土撥鼠", "蝙蝠", "禿鷹", "娃娃魚",
		},
	},
	"en": {
		adjectives: []string{
			"Lovely", "Cute", "Majestic", "Graceful", "Fierce", "Playful", "Mysterious", "Gentle",
			"Swift", "Powerful", "Adventurous", "Brave", "Charming", "Daring", "Energetic", "Fearless",
			"Gleeful", "Heroic", "Inventive", "Jolly", "Kindhearted", "Lively", "Mysterious", "Noble",
			"Optimistic", "Playful", "Quirky", "Radiant", "Spirited", "Tenacious", "Unique", "Vibrant",
			"Witty", "Xenial", "Youthful", "Zealous",
		},
		names: []string{
			"Cat", "Dog", "Fox", "Bear", "Wolf", "Rabbit", "Deer", "Lion", "Tiger", "Leopard",
			"Elephant", "Giraffe", "Zebra", "Hippo", "Rhino", "Panda", "Koala", "Sloth", "Otter",
			"Meerkat", "Aardvark", "Beaver", "Cheetah", "Dolphin", "Flamingo", "Giraffe",
			"Hippopotamus", "Iguana", "Jaguar", "Kangaroo", "Lemur", "Meerkat", "Narwhal", "Ocelot",
			"Penguin", "Quokka", "Raccoon", "Sloth", "Tiger", "Umbrellabird", "Vulture", "Walrus",
			"Xerus", "Yak", "Zebra",
		},
	},
	"ja": {
		adjectives: []string{
			"愛らしい", "かわいい", "壮大な", "優雅な", "激しい", "遊び心のある", "神秘的な", "優しい", "迅速な", "強力な", "冒険好きな", "勇敢な",
			"魅力的な", "大胆な", "エネルギッシュな", "恐れを知らない", "歓喜に満ちた", "英雄的な", "独創的な", "陽気な", "心優しい", "生き生きとした",
			"神秘的な", "高貴な", "楽観的な", "遊び心のある", "風変わりな", "輝かしい", "元気いっぱいの", "粘り強い", "ユニークな", "活気のある",
			"機知に富んだ",
		},
		names: []string{
			"子猫", "子犬", "キツネ", "子グマ", "ウサギ", "子鹿", "トラ", "象", "キリン", "シマウマ", "カバ", "サイ", "パンダ", "コアラ",
			"ナマケモノ", "カワウソ", "サル", "オウム", "カエル", "カメ", "ヒョウ", "ラクダ", "クジラ", "イルカ", "チョウ", "ミツバチ", "タコ",
			"サメ", "ペンギン", "ゴキブリ", "アリ", "ヘビ", "ダチョウ", "ワニ", "ゴリラ", "ヒトデ", "タツノオトシゴ", "カンガルー", "ネズミ",
			"アリクイ", "カモノハシ", "ユニコーン", "アルパカ", "モグラ", "コウモリ", "ハゲワシ", "アホロートル",
		},
	},
}

// defaultNicknameLocale is the site's own language, used when the caller's is unknown or
// unsupported.
const defaultNicknameLocale = "zh_TW"

// normalizeNicknameLocale maps what a client sends — "zh-tw", "en-US", "ja" — onto the
// lists above.
func normalizeNicknameLocale(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if index := strings.IndexAny(locale, ",;"); index >= 0 {
		// An Accept-Language header: the first tag is the preferred one, and the quality
		// values behind it are not worth parsing to pick a starting nickname.
		locale = strings.TrimSpace(locale[:index])
	}
	switch {
	case strings.HasPrefix(locale, "zh"):
		return "zh_TW"
	case strings.HasPrefix(locale, "ja"):
		return "ja"
	case strings.HasPrefix(locale, "en"):
		return "en"
	default:
		return defaultNicknameLocale
	}
}

// RandomNickname produces a starting display name in the caller's language.
//
// The result always fits NicknameColumnRunes. That guard is not decorative: the English
// list can pair "Adventurous" with "Hippopotamus" for 23 characters, which the column
// would refuse outright, and Laravel never noticed because it validated only renames.
func RandomNickname(locale string) string {
	words := nicknamesByLocale[normalizeNicknameLocale(locale)]

	raw := make([]byte, 2)
	if _, err := rand.Read(raw); err != nil {
		// A fixed name is better than a failed join. The player can rename themselves.
		return words.names[0]
	}

	adjective := words.adjectives[int(raw[0])%len(words.adjectives)]
	start := int(raw[1]) % len(words.names)
	// Walk the names rather than re-drawing: every list has plenty that fit, and a loop
	// that cannot terminate is not worth the marginally flatter distribution.
	for offset := range words.names {
		name := words.names[(start+offset)%len(words.names)]
		if utf8.RuneCountInString(adjective+name) <= NicknameColumnRunes {
			return adjective + name
		}
	}
	// Unreachable with the lists above, where no single name is over the limit.
	return words.names[start]
}
