package store

// CurrencyDef 描述一种货币。
type CurrencyDef struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	NameZh string `json:"name_zh"`
	Symbol string `json:"symbol"`
}

// DefaultCurrencies：新用户初始化时写入的默认货币（与原版种子数据一致）。
var DefaultCurrencies = []CurrencyDef{
	{"USD", "US Dollar", "美元", "$"},
	{"CNY", "Chinese Yuan", "人民币", "¥"},
	{"EUR", "Euro", "欧元", "€"},
	{"GBP", "British Pound", "英镑", "£"},
	{"JPY", "Japanese Yen", "日元", "¥"},
	{"RUB", "Russian Ruble", "卢布", "₽"},
	{"HKD", "Hong Kong Dollar", "港币", "HK$"},
	{"TWD", "Taiwan Dollar", "新台币", "NT$"},
	{"KRW", "Korean Won", "韩元", "₩"},
	{"SGD", "Singapore Dollar", "新加坡元", "S$"},
	{"AUD", "Australian Dollar", "澳元", "A$"},
	{"CAD", "Canadian Dollar", "加元", "C$"},
	{"CHF", "Swiss Franc", "瑞士法郎", "Fr"},
	{"INR", "Indian Rupee", "印度卢比", "₹"},
	{"BRL", "Brazilian Real", "巴西雷亚尔", "R$"},
	{"THB", "Thai Baht", "泰铢", "฿"},
	{"MYR", "Malaysian Ringgit", "马来西亚林吉特", "RM"},
	{"PHP", "Philippine Peso", "菲律宾比索", "₱"},
	{"IDR", "Indonesian Rupiah", "印尼盾", "Rp"},
	{"TRY", "Turkish Lira", "土耳其里拉", "₺"},
	{"AED", "UAE Dirham", "阿联酋迪拉姆", "د.إ"},
}

// CurrencyCatalog：可供「新增货币」选择的常见货币（含默认货币）。不在其中的可自定义。
var CurrencyCatalog = append(append([]CurrencyDef{}, DefaultCurrencies...), []CurrencyDef{
	{"MOP", "Macanese Pataca", "澳门元", "MOP$"},
	{"NZD", "New Zealand Dollar", "新西兰元", "NZ$"},
	{"SEK", "Swedish Krona", "瑞典克朗", "kr"},
	{"NOK", "Norwegian Krone", "挪威克朗", "kr"},
	{"DKK", "Danish Krone", "丹麦克朗", "kr"},
	{"PLN", "Polish Zloty", "波兰兹罗提", "zł"},
	{"CZK", "Czech Koruna", "捷克克朗", "Kč"},
	{"HUF", "Hungarian Forint", "匈牙利福林", "Ft"},
	{"RON", "Romanian Leu", "罗马尼亚列伊", "lei"},
	{"BGN", "Bulgarian Lev", "保加利亚列弗", "лв"},
	{"ISK", "Icelandic Krona", "冰岛克朗", "kr"},
	{"UAH", "Ukrainian Hryvnia", "乌克兰格里夫纳", "₴"},
	{"KZT", "Kazakhstani Tenge", "哈萨克斯坦坚戈", "₸"},
	{"VND", "Vietnamese Dong", "越南盾", "₫"},
	{"KHR", "Cambodian Riel", "柬埔寨瑞尔", "៛"},
	{"LAK", "Lao Kip", "老挝基普", "₭"},
	{"MMK", "Myanmar Kyat", "缅甸元", "K"},
	{"BDT", "Bangladeshi Taka", "孟加拉塔卡", "৳"},
	{"PKR", "Pakistani Rupee", "巴基斯坦卢比", "₨"},
	{"LKR", "Sri Lankan Rupee", "斯里兰卡卢比", "Rs"},
	{"NPR", "Nepalese Rupee", "尼泊尔卢比", "₨"},
	{"MNT", "Mongolian Tugrik", "蒙古图格里克", "₮"},
	{"SAR", "Saudi Riyal", "沙特里亚尔", "﷼"},
	{"QAR", "Qatari Riyal", "卡塔尔里亚尔", "﷼"},
	{"KWD", "Kuwaiti Dinar", "科威特第纳尔", "د.ك"},
	{"BHD", "Bahraini Dinar", "巴林第纳尔", ".د.ب"},
	{"OMR", "Omani Rial", "阿曼里亚尔", "﷼"},
	{"ILS", "Israeli New Shekel", "以色列新谢克尔", "₪"},
	{"EGP", "Egyptian Pound", "埃及镑", "E£"},
	{"ZAR", "South African Rand", "南非兰特", "R"},
	{"NGN", "Nigerian Naira", "尼日利亚奈拉", "₦"},
	{"KES", "Kenyan Shilling", "肯尼亚先令", "KSh"},
	{"MAD", "Moroccan Dirham", "摩洛哥迪拉姆", "د.م."},
	{"MXN", "Mexican Peso", "墨西哥比索", "MX$"},
	{"ARS", "Argentine Peso", "阿根廷比索", "AR$"},
	{"CLP", "Chilean Peso", "智利比索", "CL$"},
	{"COP", "Colombian Peso", "哥伦比亚比索", "CO$"},
	{"PEN", "Peruvian Sol", "秘鲁索尔", "S/"},
	{"UYU", "Uruguayan Peso", "乌拉圭比索", "$U"},
	{"GEL", "Georgian Lari", "格鲁吉亚拉里", "₾"},
	{"AMD", "Armenian Dram", "亚美尼亚德拉姆", "֏"},
	{"AZN", "Azerbaijani Manat", "阿塞拜疆马纳特", "₼"},
	{"UZS", "Uzbekistani Som", "乌兹别克斯坦苏姆", "soʻm"},
	{"BTC", "Bitcoin", "比特币", "₿"},
	{"USDT", "Tether", "泰达币", "₮"},
}...)

// CatalogCurrency 按代码在目录中查找。
func CatalogCurrency(code string) (CurrencyDef, bool) {
	for _, c := range CurrencyCatalog {
		if c.Code == code {
			return c, true
		}
	}
	return CurrencyDef{}, false
}

// UniqueCatalog 返回去重后的货币目录。
func UniqueCatalog() []CurrencyDef {
	seen := map[string]bool{}
	out := make([]CurrencyDef, 0, len(CurrencyCatalog))
	for _, c := range CurrencyCatalog {
		if !seen[c.Code] {
			seen[c.Code] = true
			out = append(out, c)
		}
	}
	return out
}

type categoryDef struct{ Name, Icon, Color string }

// DefaultCategories：系统预置订阅分类（与原版一致）。
var DefaultCategories = []categoryDef{
	{"流媒体 / Streaming", "📺", "#e50914"},
	{"AI 服务 / AI", "🤖", "#10a37f"},
	{"游戏 / Gaming", "🎮", "#107c10"},
	{"VPS / 服务器", "🖥️", "#0078d4"},
	{"电信运营商 / Carrier (SIM 保号)", "📱", "#e60000"},
	{"软件 / Software", "🧩", "#5b6ee1"},
	{"音乐 / Music", "🎵", "#1db954"},
	{"云存储 / Cloud", "☁️", "#4285f4"},
	{"域名 / Domain", "🌐", "#f59e0b"},
	{"会员 / Membership", "💳", "#9333ea"},
	{"教育 / Education", "📚", "#0ea5e9"},
	{"新闻 / News", "📰", "#374151"},
	{"健身 / Fitness", "💪", "#ef4444"},
	{"其它 / Other", "📦", "#6b7280"},
}

type paymentDef struct{ Name, Icon string }

// DefaultPaymentMethods：新用户初始化时写入的付款方式（与原版一致）。
var DefaultPaymentMethods = []paymentDef{
	{"信用卡 / Credit Card", "💳"},
	{"借记卡 / Debit Card", "🏦"},
	{"Apple Pay", "🍎"},
	{"Google Pay", "🅖"},
	{"支付宝 / Alipay", "🅰️"},
	{"微信支付 / WeChat Pay", "💬"},
	{"PayPal", "🅿️"},
	{"银行转账 / Bank Transfer", "🏛️"},
	{"加密货币 / Crypto", "₿"},
}
