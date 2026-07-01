package snapshot

import (
	"testing"
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// fixedTime 提供可注入、可复现的 generatedAt（满足 I4 时钟外置）。
func fixedTime() time.Time {
	return time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
}

// validOverseasChannel 构造一个通过 I2 有效性闭包的海外渠道实例。
func validOverseasChannel(id string, market common.Market) ChannelInput {
	return ChannelInput{
		ChannelID:    id,
		Region:       "overseas",
		Market:       market,
		Hidden:       false,
		Enabled:      true,
		ConfigStatus: common.ConfigStatusValid,
	}
}

func channelByID(channels []ResolvedChannel, id string) (ResolvedChannel, bool) {
	for _, c := range channels {
		if c.ChannelID == id {
			return c, true
		}
	}
	return ResolvedChannel{}, false
}

// ─────────────────────────── 三类 market 合并规则 ───────────────────────────

// GLOBAL 分区：仅取 GLOBAL 实例，sourceMarket=GLOBAL。
func TestBuildRuntimeConfig_GlobalMarketUsesGlobalOnly(t *testing.T) {
	view := ValidDataView{
		GameID:      "100001",
		GeneratedAt: fixedTime(),
		Channels: []ChannelInput{
			validOverseasChannel("google", common.MarketGlobal),
			validOverseasChannel("line", common.MarketJP), // JP 独有，不应进 GLOBAL
		},
	}

	out := BuildRuntimeConfig(view)
	g := out.Markets["GLOBAL"].Channels
	if len(g) != 1 {
		t.Fatalf("GLOBAL 应仅含 1 个 GLOBAL 渠道，got %d", len(g))
	}
	if g[0].ChannelID != "google" || g[0].SourceMarket != "GLOBAL" {
		t.Fatalf("GLOBAL 渠道来源标注错误: %+v", g[0])
	}
}

// CN 分区：不加载 GLOBAL（无 fallback）。
func TestBuildRuntimeConfig_CNDoesNotLoadGlobal(t *testing.T) {
	view := ValidDataView{
		GameID:      "100001",
		GeneratedAt: fixedTime(),
		Channels: []ChannelInput{
			validOverseasChannel("google", common.MarketGlobal),
			{
				ChannelID:    "huawei_cn",
				Region:       "domestic",
				Market:       common.MarketCN,
				Enabled:      true,
				ConfigStatus: common.ConfigStatusValid,
			},
		},
	}

	out := BuildRuntimeConfig(view)
	cn := out.Markets["CN"].Channels
	if len(cn) != 1 {
		t.Fatalf("CN 应仅含本地渠道且不含 GLOBAL，got %d", len(cn))
	}
	if cn[0].ChannelID != "huawei_cn" || cn[0].SourceMarket != "CN" {
		t.Fatalf("CN 渠道来源标注错误: %+v", cn[0])
	}
}

// JP/KR/SEA/HMT：mergeByInstance —— GLOBAL 打底 + 具体 market 整实例覆盖同键 + market 独有追加。
func TestBuildRuntimeConfig_FallbackMarketsMergeByInstance(t *testing.T) {
	for _, market := range []common.Market{common.MarketJP, common.MarketKR, common.MarketSEA, common.MarketHMT} {
		market := market
		t.Run(string(market), func(t *testing.T) {
			view := ValidDataView{
				GameID:      "100001",
				GeneratedAt: fixedTime(),
				Channels: []ChannelInput{
					validOverseasChannel("google", common.MarketGlobal),   // 打底
					validOverseasChannel("facebook", common.MarketGlobal), // 仅 GLOBAL
					validOverseasChannel("google", market),                // 覆盖 GLOBAL google
					validOverseasChannel("local_only", market),            // market 独有追加
				},
			}

			out := BuildRuntimeConfig(view)
			ch := out.Markets[string(market)].Channels
			if len(ch) != 3 {
				t.Fatalf("%s 应合并出 3 个渠道(google 覆盖/facebook 打底/local_only 追加)，got %d: %+v", market, len(ch), ch)
			}
			google, ok := channelByID(ch, "google")
			if !ok || google.SourceMarket != string(market) {
				t.Fatalf("%s google 应被具体 market 整实例覆盖，got %+v", market, google)
			}
			fb, ok := channelByID(ch, "facebook")
			if !ok || fb.SourceMarket != string(common.MarketGlobal) {
				t.Fatalf("%s facebook 应来自 GLOBAL 打底，got %+v", market, fb)
			}
			lo, ok := channelByID(ch, "local_only")
			if !ok || lo.SourceMarket != string(market) {
				t.Fatalf("%s local_only 应作为 market 独有追加，got %+v", market, lo)
			}
		})
	}
}

// I3：具体 market 覆盖以「完整实例」为单位替换，禁止字段级深合并。
// GLOBAL google 带 packages + login；JP google 无 packages、login 不同 —— 覆盖后不得残留 GLOBAL 的 packages。
func TestBuildRuntimeConfig_I3_WholeInstanceOverride_NoFieldMerge(t *testing.T) {
	globalGoogle := validOverseasChannel("google", common.MarketGlobal)
	globalGoogle.Login = &TemplateConfig{
		Enabled: true, ConfigStatus: common.ConfigStatusValid,
		Config: map[string]any{"clientId": "GLOBAL_CLIENT"},
	}
	globalGoogle.Packages = []PackageConfig{{PackageCode: "p_global", BundleID: "b1", Enabled: true}}

	jpGoogle := validOverseasChannel("google", common.MarketJP)
	jpGoogle.Login = &TemplateConfig{
		Enabled: true, ConfigStatus: common.ConfigStatusValid,
		Config: map[string]any{"clientId": "JP_CLIENT"},
	}
	// 故意不给 JP 实例任何 packages。

	view := ValidDataView{
		GameID:      "100001",
		GeneratedAt: fixedTime(),
		Channels:    []ChannelInput{globalGoogle, jpGoogle},
	}

	out := BuildRuntimeConfig(view)
	jp, ok := channelByID(out.Markets["JP"].Channels, "google")
	if !ok {
		t.Fatalf("JP 缺少 google 渠道")
	}
	if jp.Login["clientId"] != "JP_CLIENT" {
		t.Fatalf("I3: login 应整体来自 JP 实例，got %+v", jp.Login)
	}
	if len(jp.Packages) != 0 {
		t.Fatalf("I3: 整实例覆盖不得残留 GLOBAL packages（字段级深合并），got %+v", jp.Packages)
	}

	// GLOBAL 分区不受覆盖影响，仍保留自身 packages。
	g, _ := channelByID(out.Markets["GLOBAL"].Channels, "google")
	if len(g.Packages) != 1 {
		t.Fatalf("GLOBAL 实例应保留自身 packages，got %+v", g.Packages)
	}
}

// ─────────────────────────── I2 有效性闭包 ───────────────────────────

func TestBuildRuntimeConfig_I2_ExcludesInvalidChannels(t *testing.T) {
	base := func(mut func(*ChannelInput)) ChannelInput {
		c := validOverseasChannel("google", common.MarketGlobal)
		mut(&c)
		return c
	}
	cases := []struct {
		name string
		ch   ChannelInput
	}{
		{"hidden", base(func(c *ChannelInput) { c.Hidden = true })},
		{"disabled", base(func(c *ChannelInput) { c.Enabled = false })},
		{"config_status_invalid", base(func(c *ChannelInput) { c.ConfigStatus = common.ConfigStatusInvalid })},
		{"incompatible_region", base(func(c *ChannelInput) { c.Region = "domestic" })}, // 海外 market 要求 overseas
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			view := ValidDataView{GameID: "100001", GeneratedAt: fixedTime(), Channels: []ChannelInput{tc.ch}}
			out := BuildRuntimeConfig(view)
			if n := len(out.Markets["GLOBAL"].Channels); n != 0 {
				t.Fatalf("无效渠道(%s)不应进合并，got %d", tc.name, n)
			}
		})
	}
}

// I2：Login/IAP 子模板未 valid → 整渠道无效剔除。
func TestBuildRuntimeConfig_I2_InvalidLoginOrIapExcludesChannel(t *testing.T) {
	withLogin := validOverseasChannel("google", common.MarketGlobal)
	withLogin.Login = &TemplateConfig{Enabled: true, ConfigStatus: common.ConfigStatusInvalid, Config: map[string]any{"x": "y"}}

	withIap := validOverseasChannel("apple", common.MarketGlobal)
	withIap.IAP = &TemplateConfig{Enabled: false, ConfigStatus: common.ConfigStatusValid, Config: map[string]any{"x": "y"}}

	view := ValidDataView{GameID: "100001", GeneratedAt: fixedTime(), Channels: []ChannelInput{withLogin, withIap}}
	out := BuildRuntimeConfig(view)
	if n := len(out.Markets["GLOBAL"].Channels); n != 0 {
		t.Fatalf("登录/IAP 未 valid 应剔除整渠道，got %d", n)
	}
}

// I2：required 插件未 valid → 整渠道剔除；非 required 插件无效 → 保留渠道但剔除该插件。
func TestBuildRuntimeConfig_I2_RequiredPluginGate(t *testing.T) {
	requiredBad := validOverseasChannel("google", common.MarketGlobal)
	requiredBad.Plugins = []PluginConfig{
		{PluginID: "must", Required: true, Region: "overseas", Enabled: false, ConfigStatus: common.ConfigStatusInvalid},
	}
	optionalBad := validOverseasChannel("facebook", common.MarketGlobal)
	optionalBad.Plugins = []PluginConfig{
		{PluginID: "opt", Required: false, Region: "overseas", Enabled: false, ConfigStatus: common.ConfigStatusInvalid, Config: map[string]any{"k": "v"}},
		{PluginID: "good", Required: false, Region: "overseas", Enabled: true, ConfigStatus: common.ConfigStatusValid, Config: map[string]any{"k": "v"}},
	}

	view := ValidDataView{GameID: "100001", GeneratedAt: fixedTime(), Channels: []ChannelInput{requiredBad, optionalBad}}
	out := BuildRuntimeConfig(view)
	ch := out.Markets["GLOBAL"].Channels
	if _, ok := channelByID(ch, "google"); ok {
		t.Fatalf("required 插件未 valid 应剔除整渠道 google")
	}
	fb, ok := channelByID(ch, "facebook")
	if !ok {
		t.Fatalf("非 required 插件无效不应剔除渠道 facebook")
	}
	if len(fb.Plugins) != 1 || fb.Plugins[0]["pluginId"] != "good" {
		t.Fatalf("无效的非 required 插件应被剔除，仅保留有效插件，got %+v", fb.Plugins)
	}
}

// ─────────────────────────── scope 过滤（00 §4.1.1） ───────────────────────────

func TestBuildRuntimeConfig_ScopeFilter_ClientBothInServerOut(t *testing.T) {
	ch := validOverseasChannel("google", common.MarketGlobal)
	ch.Login = &TemplateConfig{
		Enabled: true, ConfigStatus: common.ConfigStatusValid,
		Config: map[string]any{
			"clientField":  "c",
			"bothField":    "b",
			"missingScope": "m", // schema 未声明 scope → 缺省 both
			"serverSecret": "s",
		},
		FormSchema: []ScopeField{
			{Key: "clientField", Scope: "client"},
			{Key: "bothField", Scope: "both"},
			{Key: "serverSecret", Scope: "server"},
			// missingScope 不在 schema → 默认 both
		},
	}
	view := ValidDataView{GameID: "100001", GeneratedAt: fixedTime(), Channels: []ChannelInput{ch}}
	out := BuildRuntimeConfig(view)
	g, _ := channelByID(out.Markets["GLOBAL"].Channels, "google")
	if _, ok := g.Login["serverSecret"]; ok {
		t.Fatalf("scope=server 字段不得写入客户端配置，got %+v", g.Login)
	}
	for _, k := range []string{"clientField", "bothField", "missingScope"} {
		if _, ok := g.Login[k]; !ok {
			t.Fatalf("scope∈{client,both,缺省} 字段 %q 应保留，got %+v", k, g.Login)
		}
	}
}

// ─────────────────────────── I6 密文不外泄 ───────────────────────────

func TestBuildRuntimeConfig_I6_SecretMaskedNeverPlaintext(t *testing.T) {
	const plaintext = "PLAINTEXT_SECRET_VALUE"
	ch := validOverseasChannel("google", common.MarketGlobal)
	ch.Login = &TemplateConfig{
		Enabled: true, ConfigStatus: common.ConfigStatusValid,
		Config:       map[string]any{"apiKey": plaintext, "clientId": "public"},
		FormSchema:   []ScopeField{{Key: "apiKey", Scope: "client"}, {Key: "clientId", Scope: "client"}},
		SecretFields: []string{"apiKey"},
	}
	view := ValidDataView{GameID: "100001", GeneratedAt: fixedTime(), Channels: []ChannelInput{ch}}
	out := BuildRuntimeConfig(view)
	g, _ := channelByID(out.Markets["GLOBAL"].Channels, "google")
	if g.Login["apiKey"] != SecretMaskedValue {
		t.Fatalf("secret 字段应恒为掩码 %q，got %v", SecretMaskedValue, g.Login["apiKey"])
	}

	canonical, err := CanonicalJSON(out)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	if containsString(canonical, plaintext) {
		t.Fatalf("I6: canonical config_json 不得包含明文密钥")
	}
}

func containsString(haystack []byte, needle string) bool {
	return len(needle) > 0 && bytesIndex(haystack, needle) >= 0
}

func bytesIndex(b []byte, s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(b); i++ {
		if string(b[i:i+n]) == s {
			return i
		}
	}
	return -1
}

// ─────────────────────────── packages 过滤与排序 ───────────────────────────

func TestBuildRuntimeConfig_PackagesEnabledOnlyAndSorted(t *testing.T) {
	ch := validOverseasChannel("google", common.MarketGlobal)
	ch.Packages = []PackageConfig{
		{PackageCode: "z_pkg", Enabled: true},
		{PackageCode: "disabled", Enabled: false},
		{PackageCode: "a_pkg", Enabled: true},
	}
	view := ValidDataView{GameID: "100001", GeneratedAt: fixedTime(), Channels: []ChannelInput{ch}}
	out := BuildRuntimeConfig(view)
	g, _ := channelByID(out.Markets["GLOBAL"].Channels, "google")
	if len(g.Packages) != 2 {
		t.Fatalf("仅 enabled 的 package 应保留，got %+v", g.Packages)
	}
	if g.Packages[0].PackageCode != "a_pkg" || g.Packages[1].PackageCode != "z_pkg" {
		t.Fatalf("package 应按 packageCode 升序，got %+v", g.Packages)
	}
}

// ─────────────────────────── 游戏级配置 / accountAuth scope ───────────────────────────

func TestBuildRuntimeConfig_AccountAuthScopeAndSkipEmpty(t *testing.T) {
	view := ValidDataView{
		GameID:      "100001",
		GeneratedAt: fixedTime(),
		AccountAuth: []AccountAuthItem{
			{
				AuthTypeID: "google",
				Config:     map[string]any{"clientId": "c", "serverKey": "s"},
				FormSchema: []ScopeField{{Key: "clientId", Scope: "client"}, {Key: "serverKey", Scope: "server"}},
			},
			{
				AuthTypeID: "server_only",
				Config:     map[string]any{"serverKey": "s"},
				FormSchema: []ScopeField{{Key: "serverKey", Scope: "server"}},
			},
		},
	}
	out := BuildRuntimeConfig(view)
	aa := out.Markets["GLOBAL"].Game.AccountAuth
	if len(aa) != 1 {
		t.Fatalf("仅含 server 字段过滤后为空的 accountAuth 应被跳过，got %+v", aa)
	}
	if aa[0]["authTypeId"] != "google" {
		t.Fatalf("accountAuth authTypeId 错误，got %+v", aa[0])
	}
	if _, ok := aa[0]["serverKey"]; ok {
		t.Fatalf("scope=server 字段不得进入 accountAuth，got %+v", aa[0])
	}
}

// ─────────────────────────── paymentRoutes 透传与排序 ───────────────────────────

func TestBuildRuntimeConfig_PaymentRoutesSortedPerMarket(t *testing.T) {
	view := ValidDataView{
		GameID:      "100001",
		GeneratedAt: fixedTime(),
		PaymentRoutes: map[common.Market][]ResolvedRoute{
			common.MarketGlobal: {
				{PayWay: "wallet", Provider: "p2"},
				{PayWay: "card", Provider: "p1"},
			},
		},
	}
	out := BuildRuntimeConfig(view)
	routes := out.Markets["GLOBAL"].PaymentRoutes
	if len(routes) != 2 || routes[0].PayWay != "card" || routes[1].PayWay != "wallet" {
		t.Fatalf("paymentRoutes 应按 payWay 升序，got %+v", routes)
	}
}

// ─────────────────────────── I4 确定性（字节级一致 + hash 一致） ───────────────────────────

func TestBuildRuntimeConfig_I4_DeterministicAcrossInputOrder(t *testing.T) {
	mk := func(channels []ChannelInput, aa []AccountAuthItem) ValidDataView {
		return ValidDataView{
			GameID:      "100001",
			GeneratedAt: fixedTime(),
			AccountAuth: aa,
			Channels:    channels,
			PaymentRoutes: map[common.Market][]ResolvedRoute{
				common.MarketJP: {{PayWay: "b"}, {PayWay: "a"}},
			},
		}
	}
	chA := []ChannelInput{
		validOverseasChannel("google", common.MarketGlobal),
		validOverseasChannel("apple", common.MarketGlobal),
		validOverseasChannel("google", common.MarketJP),
	}
	chB := []ChannelInput{ // 逆序输入
		validOverseasChannel("google", common.MarketJP),
		validOverseasChannel("apple", common.MarketGlobal),
		validOverseasChannel("google", common.MarketGlobal),
	}
	aaA := []AccountAuthItem{{AuthTypeID: "b", Config: map[string]any{"k": "1"}}, {AuthTypeID: "a", Config: map[string]any{"k": "2"}}}
	aaB := []AccountAuthItem{{AuthTypeID: "a", Config: map[string]any{"k": "2"}}, {AuthTypeID: "b", Config: map[string]any{"k": "1"}}}

	outA := BuildRuntimeConfig(mk(chA, aaA))
	outB := BuildRuntimeConfig(mk(chB, aaB))

	ca, err := CanonicalJSON(outA)
	if err != nil {
		t.Fatalf("canonical A: %v", err)
	}
	cb, err := CanonicalJSON(outB)
	if err != nil {
		t.Fatalf("canonical B: %v", err)
	}
	if string(ca) != string(cb) {
		t.Fatalf("I4: 同源不同输入序应产出字节级一致 canonical JSON\nA=%s\nB=%s", ca, cb)
	}
	if HashCanonicalJSON(ca) != HashCanonicalJSON(cb) {
		t.Fatalf("I4: 同源应产出一致 fileHash")
	}
}

// I4：同一 view 多次构建输出稳定（map 迭代不引入抖动）。
func TestBuildRuntimeConfig_I4_StableAcrossRuns(t *testing.T) {
	view := ValidDataView{
		GameID:      "100001",
		GeneratedAt: fixedTime(),
		Channels: []ChannelInput{
			validOverseasChannel("google", common.MarketGlobal),
			validOverseasChannel("apple", common.MarketGlobal),
			validOverseasChannel("line", common.MarketJP),
		},
	}
	var first string
	for i := 0; i < 20; i++ {
		c, err := CanonicalJSON(BuildRuntimeConfig(view))
		if err != nil {
			t.Fatalf("canonical: %v", err)
		}
		if i == 0 {
			first = string(c)
			continue
		}
		if string(c) != first {
			t.Fatalf("I4: 多次构建输出应稳定，第 %d 次不一致", i)
		}
	}
}
