package channel

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// 创建模式枚举。
const (
	modeEmpty = "empty"
	modeCopy  = "copy"
)

// ChannelService 渠道与渠道实例的读/写用例。依赖 TxManager（仓储+事务）/ clock（hidden_at）/ AuditSink / env。
// ctx 携带按当前 env 钉死 search_path 的连接，env 不作显式写入参数（01 §4.4）。
type ChannelService struct {
	tx    TxManager
	now   func() time.Time
	audit AuditSink
	env   common.Environment
}

// NewChannelService 构造服务（now 为 nil 时取 time.Now）。
func NewChannelService(tx TxManager, now func() time.Time, audit AuditSink, env common.Environment) *ChannelService {
	if now == nil {
		now = time.Now
	}
	return &ChannelService{tx: tx, now: now, audit: audit, env: env}
}

// ===== query =====

// ListChannelOptions 列出该游戏新增渠道实例时的候选渠道主数据 + 策略（GET /games/{gameId}/channels）。
func (s *ChannelService) ListChannelOptions(ctx context.Context, gameID string) ([]dto.ChannelOptionView, error) {
	repos := s.tx.Repositories()
	if _, err := repos.GameChannels.ResolveGameRowID(ctx, gameID); err != nil {
		return nil, mapLoadErr(err, "game not found")
	}
	channels, err := repos.Channels.ListChannelsWithPolicy(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ChannelOptionView, 0, len(channels))
	for _, c := range channels {
		out = append(out, toOptionView(c))
	}
	return out, nil
}

// ListMarketChannels 渠道实例分页列表（GET /games/{gameId}/market-channels）。
func (s *ChannelService) ListMarketChannels(ctx context.Context, q dto.ListMarketChannelsQuery) (dto.Page[dto.MarketChannelListItem], error) {
	empty := dto.Page[dto.MarketChannelListItem]{}
	if q.Market != "" && !strings.EqualFold(q.Market, "ALL") && !common.Market(q.Market).IsKnown() {
		return empty, validationErr("market 非法", fieldDetail("market", "enum"))
	}
	if q.ConfigStatus != "" && !domainchannel.IsValidConfigStatus(common.ConfigStatus(q.ConfigStatus)) {
		return empty, validationErr("configStatus 非法", fieldDetail("configStatus", "enum"))
	}
	page, pageSize := normalizePage(q.Page, q.PageSize)
	q.Page, q.PageSize = page, pageSize

	if _, err := s.tx.Repositories().GameChannels.ResolveGameRowID(ctx, q.GameID); err != nil {
		return empty, mapLoadErr(err, "game not found")
	}

	items, total, err := s.tx.Repositories().GameChannels.List(ctx, q)
	if err != nil {
		return empty, err
	}
	views := make([]dto.MarketChannelListItem, 0, len(items))
	for i := range items {
		views = append(views, toListItem(items[i]))
	}
	return dto.Page[dto.MarketChannelListItem]{Items: views, Page: page, PageSize: pageSize, Total: total}, nil
}

// GetMarketChannel 渠道实例详情（GET /game-channels/{gameChannelId}）。
func (s *ChannelService) GetMarketChannel(ctx context.Context, gameChannelID int64) (dto.MarketChannelDetail, error) {
	inst, err := s.loadInstance(ctx, gameChannelID)
	if err != nil {
		return dto.MarketChannelDetail{}, err
	}
	return s.toDetail(inst), nil
}

// ===== command =====

// CreateMarketChannel 创建渠道实例（空白/复制）。校验兼容性 → 唯一性 → 空白或复制清 secret/file 置 invalid → 落库 → 审计。
func (s *ChannelService) CreateMarketChannel(ctx context.Context, cmd dto.CreateMarketChannelCmd) (dto.CreateMarketChannelResult, error) {
	zero := dto.CreateMarketChannelResult{}
	market := cmd.Market
	if !common.Market(market).IsKnown() {
		return zero, validationErr("market 非法", fieldDetail("market", "enum"))
	}
	channelID := strings.TrimSpace(cmd.ChannelID)
	if channelID == "" {
		return zero, validationErr("channelId 必填", fieldDetail("channelId", "required"))
	}
	mode := cmd.Mode
	if mode == "" {
		mode = modeEmpty
	}
	if mode != modeEmpty && mode != modeCopy {
		return zero, validationErr("mode 非法", fieldDetail("mode", "empty/copy"))
	}
	if mode == modeCopy && strings.TrimSpace(cmd.CopyFromMarket) == "" {
		return zero, validationErr("copyFromMarket 必填", fieldDetail("copyFromMarket", "required when mode=copy"))
	}
	if len(cmd.Remark) > 255 {
		return zero, validationErr("remark 长度不能超过 255", fieldDetail("remark", "maxLen 255"))
	}

	// 候选渠道主数据（含 region），并做服务端兼容性二次校验（compact 红线：不能只信前端）。
	ch, err := s.tx.Repositories().Channels.GetChannelByChannelID(ctx, channelID)
	if err != nil {
		if errors.Is(err, adminapp.ErrNotFound) {
			return zero, validationErr("channelId 不存在", fieldDetail("channelId", "not found"))
		}
		return zero, err
	}
	region := ch.Channel.Region
	if err := domainchannel.ValidateMarketChannelCompatibility(common.Market(market), region); err != nil {
		return zero, incompatibleErr("渠道与 market 不兼容")
	}

	var created domainchannel.GameMarketChannel
	err = s.tx.InTx(ctx, func(repos Repositories) error {
		gameIDRef, err := repos.GameChannels.ResolveGameRowID(ctx, cmd.GameID)
		if err != nil {
			return mapLoadErr(err, "game not found")
		}
		exists, err := repos.GameChannels.ExistsInstance(ctx, gameIDRef, market, channelID)
		if err != nil {
			return err
		}
		if exists {
			return conflictErr("market channel already exists")
		}

		var inst domainchannel.GameMarketChannel
		var copySource *domainchannel.GameMarketChannel
		if mode == modeCopy {
			source, err := repos.GameChannels.FindInstance(ctx, gameIDRef, cmd.CopyFromMarket, channelID)
			if err != nil {
				if errors.Is(err, adminapp.ErrNotFound) {
					return validationErr("复制来源渠道实例不存在", fieldDetail("copyFromMarket", "source not found"))
				}
				return err
			}
			copySource = &source
			inst = domainchannel.NewCopiedMarketChannel(cmd.GameID, market, channelID, region, source)
		} else {
			inst = domainchannel.NewBlankMarketChannel(cmd.GameID, market, channelID, region)
		}
		inst.GameIDRef = gameIDRef
		inst.ChannelIDRef = ch.Channel.ID
		if cmd.Enabled != nil {
			inst.Enabled = *cmd.Enabled
		}
		inst.Remark = cmd.Remark

		saved, err := repos.GameChannels.Insert(ctx, inst)
		if err != nil {
			return err
		}
		created = saved

		// 复制创建强约束（00 §3.4 / channel-login compact §业务规则3/4）：
		// channel_only 渠道复制实例时，同步向 game_channel_login_configs 落库
		// （仅普通字段、清空 secret/file、config_status=invalid），避免 GET login-config 仍返回 empty 占位。
		if mode == modeCopy && copySource != nil && ch.Policy.LoginMode == common.LoginModeChannelOnly {
			if err := copyLoginConfig(ctx, repos, ch.Channel.ID, *copySource, saved.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return zero, mapWriteErr(err, "market channel already exists")
	}

	s.writeAudit(ctx, "channel.create", created.ID, map[string]any{
		"gameId": cmd.GameID, "market": market, "channelId": channelID, "mode": mode, "copyFromMarket": created.CopiedFromMarket,
	})
	return dto.CreateMarketChannelResult{
		GameChannelID:    created.ID,
		DisplayKey:       created.DisplayKey(),
		Market:           created.Market,
		ChannelID:        created.ChannelID,
		ConfigStatus:     string(created.ConfigStatus),
		LastCheckMessage: created.LastCheckMessage,
		CopiedFromMarket: created.CopiedFromMarket,
	}, nil
}

// UpdateMarketChannel 编辑渠道实例（仅 enabled/remark；身份不可改）。
func (s *ChannelService) UpdateMarketChannel(ctx context.Context, cmd dto.UpdateMarketChannelCmd) (dto.MarketChannelDetail, error) {
	if _, err := s.loadInstance(ctx, cmd.GameChannelID); err != nil {
		return dto.MarketChannelDetail{}, err
	}
	patch := GameChannelPatch{Enabled: cmd.Enabled}
	if cmd.Remark != nil {
		if len(*cmd.Remark) > 255 {
			return dto.MarketChannelDetail{}, validationErr("remark 长度不能超过 255", fieldDetail("remark", "maxLen 255"))
		}
		patch.Remark = cmd.Remark
	}
	if err := s.tx.InTx(ctx, func(repos Repositories) error {
		return repos.GameChannels.UpdateBasics(ctx, cmd.GameChannelID, patch)
	}); err != nil {
		return dto.MarketChannelDetail{}, mapWriteErr(err, "market channel conflict")
	}
	s.writeAudit(ctx, "channel.update", cmd.GameChannelID, map[string]any{"fields": changedBasics(patch)})
	return s.GetMarketChannel(ctx, cmd.GameChannelID)
}

// HideMarketChannel 隐藏渠道实例（仅 valid 可隐藏；隐藏后运行态全 false；审计 channel.hide）。
func (s *ChannelService) HideMarketChannel(ctx context.Context, cmd dto.HideMarketChannelCmd) (dto.MarketChannelDetail, error) {
	inst, err := s.loadInstance(ctx, cmd.GameChannelID)
	if err != nil {
		return dto.MarketChannelDetail{}, err
	}
	if err := domainchannel.CanHide(inst.ConfigStatus); err != nil {
		return dto.MarketChannelDetail{}, conflictErr("仅可隐藏 valid 状态的渠道实例")
	}
	operator := s.operator(ctx)
	if err := s.tx.InTx(ctx, func(repos Repositories) error {
		return repos.GameChannels.Hide(ctx, cmd.GameChannelID, operator, s.now())
	}); err != nil {
		return dto.MarketChannelDetail{}, mapWriteErr(err, "market channel conflict")
	}
	s.writeAudit(ctx, "channel.hide", cmd.GameChannelID, map[string]any{"reason": cmd.Reason, "operator": operator})
	return s.GetMarketChannel(ctx, cmd.GameChannelID)
}

// UnhideMarketChannel 恢复渠道实例（运行态按规则重新推导；审计 channel.unhide）。
func (s *ChannelService) UnhideMarketChannel(ctx context.Context, cmd dto.UnhideMarketChannelCmd) (dto.MarketChannelDetail, error) {
	if _, err := s.loadInstance(ctx, cmd.GameChannelID); err != nil {
		return dto.MarketChannelDetail{}, err
	}
	if err := s.tx.InTx(ctx, func(repos Repositories) error {
		return repos.GameChannels.Unhide(ctx, cmd.GameChannelID)
	}); err != nil {
		return dto.MarketChannelDetail{}, mapWriteErr(err, "market channel conflict")
	}
	s.writeAudit(ctx, "channel.unhide", cmd.GameChannelID, map[string]any{"operator": s.operator(ctx)})
	return s.GetMarketChannel(ctx, cmd.GameChannelID)
}

// ===== packages =====

// ListPackages 列出渠道实例下的渠道包（GET /game-channels/{gameChannelId}/packages）。
func (s *ChannelService) ListPackages(ctx context.Context, gameChannelID int64) ([]dto.ChannelPackageView, error) {
	if _, err := s.loadInstance(ctx, gameChannelID); err != nil {
		return nil, err
	}
	pkgs, err := s.tx.Repositories().Packages.ListByGameChannel(ctx, gameChannelID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ChannelPackageView, 0, len(pkgs))
	for i := range pkgs {
		out = append(out, toPackageView(pkgs[i]))
	}
	return out, nil
}

// CreatePackage 创建渠道包（POST /game-channels/{gameChannelId}/packages）。
func (s *ChannelService) CreatePackage(ctx context.Context, cmd dto.CreatePackageCmd) (dto.ChannelPackageView, error) {
	zero := dto.ChannelPackageView{}
	code := strings.TrimSpace(cmd.PackageCode)
	if code == "" || len(code) > 64 {
		return zero, validationErr("packageCode 必填且不超过 64", fieldDetail("packageCode", "1-64"))
	}
	name := strings.TrimSpace(cmd.PackageName)
	if name == "" || len(name) > 128 {
		return zero, validationErr("packageName 必填且不超过 128", fieldDetail("packageName", "1-128"))
	}
	if strings.TrimSpace(cmd.MarketCode) == "" {
		return zero, validationErr("marketCode 必填", fieldDetail("marketCode", "required"))
	}
	if len(cmd.BundleID) > 128 {
		return zero, validationErr("bundleId 长度不能超过 128", fieldDetail("bundleId", "maxLen 128"))
	}

	inst, err := s.loadInstance(ctx, cmd.GameChannelID)
	if err != nil {
		return zero, err
	}
	if err := domainchannel.ValidatePackageMarket(cmd.MarketCode, inst.Market); err != nil {
		return zero, validationErr("marketCode 必须与所属渠道实例一致", fieldDetail("marketCode", "must equal instance market"))
	}

	pkg := domainchannel.ChannelPackage{
		GameChannelIDRef:     cmd.GameChannelID,
		PackageCode:          code,
		PackageName:          name,
		MarketCode:           cmd.MarketCode,
		BundleID:             cmd.BundleID,
		InheritChannelConfig: boolOr(cmd.InheritChannelConfig, true),
		Enabled:              boolOr(cmd.Enabled, true),
		OverrideJSON:         map[string]any{},
	}
	var created domainchannel.ChannelPackage
	err = s.tx.InTx(ctx, func(repos Repositories) error {
		exists, err := repos.Packages.ExistsPackageCode(ctx, cmd.GameChannelID, code)
		if err != nil {
			return err
		}
		if exists {
			return conflictErr("package code already exists")
		}
		saved, err := repos.Packages.InsertPackage(ctx, pkg)
		if err != nil {
			return err
		}
		created = saved
		return nil
	})
	if err != nil {
		return zero, mapWriteErr(err, "package code already exists")
	}
	s.writeAudit(ctx, "package.create", created.ID, map[string]any{"gameChannelId": cmd.GameChannelID, "packageCode": code})
	return toPackageView(created), nil
}

// UpdatePackage 编辑渠道包（PATCH /channel-packages/{packageId}）。
func (s *ChannelService) UpdatePackage(ctx context.Context, cmd dto.UpdatePackageCmd) (dto.ChannelPackageView, error) {
	zero := dto.ChannelPackageView{}
	if _, err := s.loadPackage(ctx, cmd.PackageID); err != nil {
		return zero, err
	}
	patch := PackagePatch{
		InheritChannelConfig: cmd.InheritChannelConfig,
		Enabled:              cmd.Enabled,
		OverrideJSON:         cmd.OverrideJSON,
	}
	if cmd.PackageName != nil {
		name := strings.TrimSpace(*cmd.PackageName)
		if name == "" || len(name) > 128 {
			return zero, validationErr("packageName 必填且不超过 128", fieldDetail("packageName", "1-128"))
		}
		patch.PackageName = &name
	}
	if cmd.BundleID != nil {
		if len(*cmd.BundleID) > 128 {
			return zero, validationErr("bundleId 长度不能超过 128", fieldDetail("bundleId", "maxLen 128"))
		}
		patch.BundleID = cmd.BundleID
	}
	if err := s.tx.InTx(ctx, func(repos Repositories) error {
		return repos.Packages.UpdatePackage(ctx, cmd.PackageID, patch)
	}); err != nil {
		return zero, mapWriteErr(err, "package conflict")
	}
	s.writeAudit(ctx, "package.update", cmd.PackageID, map[string]any{"fields": changedPackage(patch)})
	updated, err := s.loadPackage(ctx, cmd.PackageID)
	if err != nil {
		return zero, err
	}
	return toPackageView(updated), nil
}

// ===== helpers =====

// copyLoginConfig 复制创建 channel_only 实例时，向新实例写入渠道登录配置：
// 取该渠道 enabled 最新模板区分普通/secret/file 字段，读取源实例登录配置，
// 只复制普通字段、清空 secret/file，强制 config_status=invalid（复制后不联动源实例）。
// 落当前 env schema（仓储 SQL 不带 schema 前缀/不带 env 谓词），不存明文密钥（secret 字段直接清空）。
func copyLoginConfig(ctx context.Context, repos Repositories, channelIDRef int64, source domainchannel.GameMarketChannel, newGameChannelID int64) error {
	if repos.LoginTemplates == nil || repos.LoginConfigs == nil {
		return nil
	}
	tpl, err := repos.LoginTemplates.GetPublishedByChannel(ctx, channelIDRef)
	if err != nil {
		return err
	}
	srcCfg, err := repos.LoginConfigs.GetByGameChannel(ctx, source.ID)
	if err != nil {
		return err
	}
	cfg := domainchannel.NewCopiedLoginConfig(newGameChannelID, tpl, srcCfg)
	return repos.LoginConfigs.Upsert(ctx, &cfg)
}

func (s *ChannelService) loadInstance(ctx context.Context, id int64) (domainchannel.GameMarketChannel, error) {
	inst, err := s.tx.Repositories().GameChannels.GetByID(ctx, id)
	if err != nil {
		return domainchannel.GameMarketChannel{}, mapLoadErr(err, "market channel not found")
	}
	return inst, nil
}

func (s *ChannelService) loadPackage(ctx context.Context, id int64) (domainchannel.ChannelPackage, error) {
	pkg, err := s.tx.Repositories().Packages.GetPackageByID(ctx, id)
	if err != nil {
		return domainchannel.ChannelPackage{}, mapLoadErr(err, "package not found")
	}
	return pkg, nil
}

func (s *ChannelService) operator(ctx context.Context) string {
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		if ac.UserName != "" {
			return ac.UserName
		}
	}
	return "system"
}

func (s *ChannelService) writeAudit(ctx context.Context, action string, resourceID int64, detail map[string]any) {
	if s.audit == nil {
		return
	}
	actor := int64(0)
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		actor = ac.UserID
	}
	resType := "channel"
	if strings.HasPrefix(action, "package.") {
		resType = "channel_package"
	}
	s.audit.Write(ctx, AuditEntry{
		ActorID: actor, Action: action, ResourceType: resType, ResourceID: itoa(resourceID), Detail: detail,
	})
}

func (s *ChannelService) toDetail(inst domainchannel.GameMarketChannel) dto.MarketChannelDetail {
	flags := inst.ResolveRuntimeFlags()
	return dto.MarketChannelDetail{
		GameChannelID:           inst.ID,
		DisplayKey:              inst.DisplayKey(),
		GameID:                  inst.GameID,
		Market:                  inst.Market,
		ChannelID:               inst.ChannelID,
		Region:                  string(inst.Region),
		Compatible:              inst.Compatible(),
		Enabled:                 inst.Enabled,
		Hidden:                  inst.Hidden,
		HiddenBy:                inst.HiddenBy,
		HiddenAt:                inst.HiddenAt,
		ConfigStatus:            string(inst.ConfigStatus),
		LastCheckAt:             inst.LastCheckAt,
		LastCheckMessage:        inst.LastCheckMessage,
		CopiedFromMarket:        inst.CopiedFromMarket,
		Remark:                  inst.Remark,
		IncludedInSnapshot:      flags.IncludedInSnapshot,
		IncludedInSync:          flags.IncludedInSync,
		IncludedInRuntimeConfig: flags.IncludedInRuntimeConfig,
		RuntimeReason:           flags.Reason,
		Environment:             string(s.env),
		CreatedAt:               inst.CreatedAt,
		UpdatedAt:               inst.UpdatedAt,
	}
}

func toListItem(inst domainchannel.GameMarketChannel) dto.MarketChannelListItem {
	flags := inst.ResolveRuntimeFlags()
	return dto.MarketChannelListItem{
		GameChannelID:           inst.ID,
		DisplayKey:              inst.DisplayKey(),
		GameID:                  inst.GameID,
		Market:                  inst.Market,
		ChannelID:               inst.ChannelID,
		Region:                  string(inst.Region),
		Compatible:              inst.Compatible(),
		Hidden:                  inst.Hidden,
		ConfigStatus:            string(inst.ConfigStatus),
		IncludedInSnapshot:      flags.IncludedInSnapshot,
		IncludedInSync:          flags.IncludedInSync,
		IncludedInRuntimeConfig: flags.IncludedInRuntimeConfig,
		RuntimeReason:           flags.Reason,
		CopiedFromMarket:        inst.CopiedFromMarket,
		UpdatedAt:               inst.UpdatedAt,
	}
}

func toOptionView(c domainchannel.ChannelWithPolicy) dto.ChannelOptionView {
	return dto.ChannelOptionView{
		ChannelID:     c.Channel.ChannelID,
		ChannelName:   c.Channel.ChannelName,
		ChannelType:   c.Channel.ChannelType,
		Region:        string(c.Channel.Region),
		LoginMode:     string(c.Policy.LoginMode),
		PaymentMode:   string(c.Policy.PaymentMode),
		LoginLocked:   c.Policy.LoginLocked,
		PaymentLocked: c.Policy.PaymentLocked,
	}
}

func toPackageView(p domainchannel.ChannelPackage) dto.ChannelPackageView {
	override := p.OverrideJSON
	if override == nil {
		override = map[string]any{}
	}
	return dto.ChannelPackageView{
		PackageID:            p.ID,
		GameChannelID:        p.GameChannelIDRef,
		PackageCode:          p.PackageCode,
		PackageName:          p.PackageName,
		MarketCode:           p.MarketCode,
		BundleID:             p.BundleID,
		InheritChannelConfig: p.InheritChannelConfig,
		Enabled:              p.Enabled,
		OverrideJSON:         override,
		CreatedAt:            p.CreatedAt,
		UpdatedAt:            p.UpdatedAt,
	}
}

func changedBasics(p GameChannelPatch) []string {
	fields := []string{}
	if p.Enabled != nil {
		fields = append(fields, "enabled")
	}
	if p.Remark != nil {
		fields = append(fields, "remark")
	}
	return fields
}

func changedPackage(p PackagePatch) []string {
	fields := []string{}
	if p.PackageName != nil {
		fields = append(fields, "packageName")
	}
	if p.BundleID != nil {
		fields = append(fields, "bundleId")
	}
	if p.InheritChannelConfig != nil {
		fields = append(fields, "inheritChannelConfig")
	}
	if p.Enabled != nil {
		fields = append(fields, "enabled")
	}
	if p.OverrideJSON != nil {
		fields = append(fields, "overrideJson")
	}
	return fields
}

func boolOr(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}

// normalizePage 归一化分页（00 §7.3：page>=1，pageSize 默认 20、最大 100）。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// mapLoadErr 把仓储 NotFound 映射为统一 NOT_FOUND；其它原样返回。
func mapLoadErr(err error, notFoundMsg string) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr(notFoundMsg)
	}
	return err
}

// mapWriteErr 透传 *Error；DB 唯一冲突映射为指定冲突消息；NotFound 映射为 NOT_FOUND；其它原样返回。
func mapWriteErr(err error, conflictMsg string) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrConflict) {
		return conflictErr(conflictMsg)
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr("resource not found")
	}
	return err
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
