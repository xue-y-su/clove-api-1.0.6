package common

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
)

const (
	ToolBillingUnitPer1KCalls = "usd_per_1k_calls"
	ToolBillingUnitPerCall    = "usd_per_call"
)

type ToolBillingContext struct {
	Pricing              *operation_setting.ToolPricingSnapshot
	PublicModel          string
	GroupRatio           decimal.Decimal
	OtherRatioMultiplier decimal.Decimal
	QuotaPerUnit         decimal.Decimal
}

type ToolEvent struct {
	Name         string
	StableID     string
	Aliases      []string
	OutputIndex  *int
	Payload      []byte
	Count        int
	ImageQuality string
	ImageSize    string
}

type ToolBillingItem struct {
	Name          string  `json:"name"`
	Count         int     `json:"count"`
	Unit          string  `json:"unit"`
	UnitPrice     float64 `json:"unit_price"`
	SubtotalQuota int     `json:"subtotal_quota"`
}

type ToolBillingResult struct {
	ToolQuota int               `json:"tool_quota"`
	Items     []ToolBillingItem `json:"tool_surcharges,omitempty"`
	exact     decimal.Decimal
}

func (r ToolBillingResult) ExactQuota() decimal.Decimal { return r.exact }

type confirmedToolEvent struct {
	event     ToolEvent
	aliases   []string
	canonical string
}

type ToolBillingMeter struct {
	context ToolBillingContext
	mu      sync.Mutex
	events  map[string]confirmedToolEvent
	aliases map[string]string
}

type ToolBillingAttempt struct {
	parent  *ToolBillingMeter
	mu      sync.Mutex
	events  map[string]confirmedToolEvent
	aliases map[string]string
	closed  bool
}

func NewToolBillingMeter(context ToolBillingContext) *ToolBillingMeter {
	if context.Pricing == nil {
		context.Pricing = operation_setting.CurrentToolPricingSnapshot()
	}
	if context.OtherRatioMultiplier.IsZero() {
		context.OtherRatioMultiplier = decimal.NewFromInt(1)
	}
	return &ToolBillingMeter{
		context: context,
		events:  make(map[string]confirmedToolEvent),
		aliases: make(map[string]string),
	}
}

func NewToolBillingMeterForRelay(info *RelayInfo) *ToolBillingMeter {
	context := ToolBillingContext{
		Pricing:              operation_setting.CurrentToolPricingSnapshot(),
		GroupRatio:           decimal.NewFromInt(1),
		OtherRatioMultiplier: decimal.NewFromInt(1),
		QuotaPerUnit:         decimal.NewFromFloat(rootcommon.QuotaPerUnit),
	}
	if info != nil {
		context.PublicModel = info.OriginModelName
		context.GroupRatio = decimal.NewFromFloat(info.PriceData.GroupRatioInfo.GroupRatio)
		context.OtherRatioMultiplier = decimal.NewFromFloat(info.PriceData.OtherRatioMultiplier())
	}
	return NewToolBillingMeter(context)
}

func (info *RelayInfo) InitializeToolBillingMeter() {
	if info == nil {
		return
	}
	info.ToolBillingMeter = NewToolBillingMeterForRelay(info)
	info.ToolBillingAttempt = nil
}

func (info *RelayInfo) BeginToolBillingAttempt() *ToolBillingAttempt {
	if info == nil {
		return nil
	}
	if info.ToolBillingMeter == nil {
		info.InitializeToolBillingMeter()
	}
	if info.ToolBillingAttempt != nil {
		info.ToolBillingMeter.DiscardAttempt(info.ToolBillingAttempt)
	}
	info.ToolBillingAttempt = info.ToolBillingMeter.NewAttempt()
	return info.ToolBillingAttempt
}

func (info *RelayInfo) SetToolBillingBaseQuota(quota int) {
	if info == nil {
		return
	}
	if quota < 0 {
		quota = 0
	}
	info.toolBillingReserveMu.Lock()
	info.toolBillingBaseQuota = quota
	info.toolBillingBaseQuotaSet = true
	info.toolBillingReservedTarget = quota
	info.toolBillingReserveMu.Unlock()
}

func (info *RelayInfo) ConfirmToolEvent(event ToolEvent) error {
	if info == nil {
		return nil
	}
	if info.ToolBillingAttempt == nil {
		info.BeginToolBillingAttempt()
	}
	if info.Billing == nil {
		return info.ToolBillingAttempt.Confirm(event)
	}

	info.toolBillingReserveMu.Lock()
	defer info.toolBillingReserveMu.Unlock()
	attempt := info.ToolBillingAttempt
	preview, err := attempt.previewConfirm(event)
	if err != nil {
		return err
	}
	if !info.toolBillingBaseQuotaSet {
		info.toolBillingBaseQuota = info.Billing.GetPreConsumedQuota()
		info.toolBillingBaseQuotaSet = true
		info.toolBillingReservedTarget = info.toolBillingBaseQuota
	}
	result, err := info.ToolBillingMeter.TargetToolQuotaIncludingAttempt(preview)
	if err != nil {
		return err
	}
	target64 := int64(info.toolBillingBaseQuota) + int64(result.ToolQuota)
	if target64 > int64(rootcommon.MaxQuota) {
		return fmt.Errorf("tool reservation target overflow: base=%d tool=%d", info.toolBillingBaseQuota, result.ToolQuota)
	}
	target := int(target64)
	if target > info.toolBillingReservedTarget {
		if err := info.Billing.ReserveToTarget(target); err != nil {
			return fmt.Errorf("reserve confirmed tool quota: %w", err)
		}
		info.toolBillingReservedTarget = target
	}
	return attempt.Confirm(event)
}

func (info *RelayInfo) CommitToolBillingAttempt() error {
	if info == nil || info.ToolBillingMeter == nil || info.ToolBillingAttempt == nil {
		return nil
	}
	attempt := info.ToolBillingAttempt
	if err := info.ToolBillingMeter.CommitAttempt(attempt); err != nil {
		return err
	}
	info.ToolBillingAttempt = nil
	return nil
}

func (info *RelayInfo) DiscardToolBillingAttempt() {
	if info == nil || info.ToolBillingMeter == nil || info.ToolBillingAttempt == nil {
		return
	}
	info.ToolBillingMeter.DiscardAttempt(info.ToolBillingAttempt)
	info.ToolBillingAttempt = nil
}

func (m *ToolBillingMeter) NewAttempt() *ToolBillingAttempt {
	return &ToolBillingAttempt{
		parent:  m,
		events:  make(map[string]confirmedToolEvent),
		aliases: make(map[string]string),
	}
}

func (a *ToolBillingAttempt) previewConfirm(event ToolEvent) (*ToolBillingAttempt, error) {
	if a == nil {
		return nil, fmt.Errorf("tool billing attempt is nil")
	}
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil, fmt.Errorf("tool billing attempt is already closed")
	}
	preview := &ToolBillingAttempt{
		parent:  a.parent,
		events:  make(map[string]confirmedToolEvent, len(a.events)+1),
		aliases: make(map[string]string, len(a.aliases)+1),
	}
	for key, confirmed := range a.events {
		preview.events[key] = confirmed
	}
	for alias, canonical := range a.aliases {
		preview.aliases[alias] = canonical
	}
	a.mu.Unlock()
	if err := preview.Confirm(event); err != nil {
		return nil, err
	}
	return preview, nil
}

func normalizeEvidencePayload(payload []byte) []byte {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err == nil {
		if normalized, marshalErr := json.Marshal(value); marshalErr == nil {
			return normalized
		}
	}
	return trimmed
}

func IsNonBillableResponsesStatus(status []byte) bool {
	if len(status) == 0 {
		return false
	}
	var value string
	if err := json.Unmarshal(status, &value); err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "failed", "cancelled", "canceled", "incomplete":
		return true
	default:
		return false
	}
}

func toolEventAliases(event ToolEvent) ([]string, error) {
	name := strings.TrimSpace(event.Name)
	if name == "" {
		return nil, fmt.Errorf("tool event name is required")
	}
	identities := make([]string, 0, len(event.Aliases)+3)
	if id := strings.TrimSpace(event.StableID); id != "" {
		identities = append(identities, "identity:"+id)
	}
	for _, alias := range event.Aliases {
		if alias = strings.TrimSpace(alias); alias != "" {
			identities = append(identities, "identity:"+alias)
		}
	}
	if event.OutputIndex != nil && *event.OutputIndex >= 0 {
		identities = append(identities, fmt.Sprintf("output:%d", *event.OutputIndex))
	}
	if normalized := normalizeEvidencePayload(event.Payload); len(normalized) > 0 {
		digest := sha256.Sum256(normalized)
		identities = append(identities, "payload:"+hex.EncodeToString(digest[:]))
	}
	if len(identities) == 0 {
		return nil, fmt.Errorf("tool event %q has no stable evidence", name)
	}
	unique := make(map[string]struct{}, len(identities))
	aliases := make([]string, 0, len(identities))
	for _, identity := range identities {
		key := name + "\x00" + identity
		if _, exists := unique[key]; exists {
			continue
		}
		unique[key] = struct{}{}
		aliases = append(aliases, key)
	}
	sort.Strings(aliases)
	return aliases, nil
}

func (a *ToolBillingAttempt) Confirm(event ToolEvent) error {
	if a == nil {
		return fmt.Errorf("tool billing attempt is nil")
	}
	aliases, err := toolEventAliases(event)
	if err != nil {
		return err
	}
	if event.Count < 0 {
		return fmt.Errorf("tool event count must be non-negative")
	}
	if event.Count == 0 {
		event.Count = 1
	}
	event.Name = strings.TrimSpace(event.Name)

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return fmt.Errorf("tool billing attempt is already closed")
	}
	for _, alias := range aliases {
		if canonical, exists := a.aliases[alias]; exists {
			for _, newAlias := range aliases {
				a.aliases[newAlias] = canonical
			}
			return nil
		}
	}
	canonical := aliases[0]
	a.events[canonical] = confirmedToolEvent{event: event, aliases: aliases, canonical: canonical}
	for _, alias := range aliases {
		a.aliases[alias] = canonical
	}
	return nil
}

func (m *ToolBillingMeter) CommitAttempt(attempt *ToolBillingAttempt) error {
	if m == nil || attempt == nil {
		return fmt.Errorf("tool billing meter or attempt is nil")
	}
	attempt.mu.Lock()
	defer attempt.mu.Unlock()
	if attempt.parent != m {
		return fmt.Errorf("tool billing attempt belongs to another meter")
	}
	if attempt.closed {
		return fmt.Errorf("tool billing attempt is already closed")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, confirmed := range attempt.events {
		duplicateCanonical := ""
		for _, alias := range confirmed.aliases {
			if canonical, exists := m.aliases[alias]; exists {
				duplicateCanonical = canonical
				break
			}
		}
		if duplicateCanonical != "" {
			for _, alias := range confirmed.aliases {
				m.aliases[alias] = duplicateCanonical
			}
			continue
		}
		m.events[confirmed.canonical] = confirmed
		for _, alias := range confirmed.aliases {
			m.aliases[alias] = confirmed.canonical
		}
	}
	attempt.closed = true
	return nil
}

func (m *ToolBillingMeter) DiscardAttempt(attempt *ToolBillingAttempt) {
	if m == nil || attempt == nil || attempt.parent != m {
		return
	}
	attempt.mu.Lock()
	attempt.closed = true
	attempt.events = nil
	attempt.aliases = nil
	attempt.mu.Unlock()
}

type exactToolBillingLine struct {
	name      string
	count     int
	unit      string
	unitPrice decimal.Decimal
	exact     decimal.Decimal
	floor     int
	remainder decimal.Decimal
}

func (m *ToolBillingMeter) snapshotEvents() []confirmedToolEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	events := make([]confirmedToolEvent, 0, len(m.events))
	for _, event := range m.events {
		events = append(events, event)
	}
	return events
}

func (m *ToolBillingMeter) exactLines(events []confirmedToolEvent) []exactToolBillingLine {
	if m == nil || m.context.Pricing == nil || !m.context.Pricing.Enabled() || !m.context.GroupRatio.IsPositive() || !m.context.QuotaPerUnit.IsPositive() {
		return nil
	}
	type aggregate struct {
		name      string
		count     int
		unit      string
		unitPrice decimal.Decimal
		exact     decimal.Decimal
	}
	aggregates := make(map[string]*aggregate)
	for _, confirmed := range events {
		event := confirmed.event
		var unit string
		var price decimal.Decimal
		if event.Name == "image_generation" {
			var mode string
			price, mode = m.context.Pricing.ImagePrice(m.context.PublicModel, event.ImageQuality, event.ImageSize)
			if mode == operation_setting.ImagePricingMatrix {
				unit = ToolBillingUnitPerCall
			} else {
				unit = ToolBillingUnitPer1KCalls
			}
		} else {
			price = m.context.Pricing.Price(event.Name, m.context.PublicModel)
			unit = ToolBillingUnitPer1KCalls
		}
		if !price.IsPositive() || event.Count <= 0 {
			continue
		}
		exact := price.Mul(decimal.NewFromInt(int64(event.Count)))
		if unit == ToolBillingUnitPer1KCalls {
			exact = exact.Div(decimal.NewFromInt(1000))
		}
		exact = exact.Mul(m.context.GroupRatio).Mul(m.context.QuotaPerUnit)
		if m.context.Pricing.MultiplierMode() == operation_setting.ToolMultiplierFull {
			exact = exact.Mul(m.context.OtherRatioMultiplier)
		}
		if !exact.IsPositive() {
			continue
		}
		key := event.Name + "\x00" + unit + "\x00" + price.String()
		if existing := aggregates[key]; existing != nil {
			existing.count += event.Count
			existing.exact = existing.exact.Add(exact)
			continue
		}
		aggregates[key] = &aggregate{name: event.Name, count: event.Count, unit: unit, unitPrice: price, exact: exact}
	}
	lines := make([]exactToolBillingLine, 0, len(aggregates))
	for _, aggregate := range aggregates {
		lines = append(lines, exactToolBillingLine{
			name: aggregate.name, count: aggregate.count, unit: aggregate.unit,
			unitPrice: aggregate.unitPrice, exact: aggregate.exact,
		})
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].name != lines[j].name {
			return lines[i].name < lines[j].name
		}
		if comparison := lines[i].unitPrice.Cmp(lines[j].unitPrice); comparison != 0 {
			return comparison < 0
		}
		return lines[i].unit < lines[j].unit
	})
	return lines
}

func allocateToolQuota(lines []exactToolBillingLine) (ToolBillingResult, error) {
	result := ToolBillingResult{}
	if len(lines) == 0 {
		return result, nil
	}
	for i := range lines {
		result.exact = result.exact.Add(lines[i].exact)
	}
	if !result.exact.IsPositive() {
		return ToolBillingResult{}, nil
	}
	target, err := rootcommon.QuotaFromDecimalStrict(result.exact)
	if err != nil {
		return ToolBillingResult{}, fmt.Errorf("tool quota overflow: %w", err)
	}
	minimumApplied := target == 0
	if minimumApplied {
		target = 1
	}

	floorTotal := 0
	for i := range lines {
		floorValue := lines[i].exact.Floor()
		lines[i].floor = int(floorValue.IntPart())
		lines[i].remainder = lines[i].exact.Sub(floorValue)
		floorTotal += lines[i].floor
	}
	if minimumApplied {
		for i := range lines {
			lines[i].floor = 0
		}
		lines[0].floor = 1
	} else {
		remaining := target - floorTotal
		order := make([]int, len(lines))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(i, j int) bool {
			return lines[order[i]].remainder.GreaterThan(lines[order[j]].remainder)
		})
		for i := 0; i < remaining; i++ {
			lines[order[i%len(order)]].floor++
		}
	}

	result.ToolQuota = target
	result.Items = make([]ToolBillingItem, 0, len(lines))
	for _, line := range lines {
		unitPrice, _ := line.unitPrice.Float64()
		result.Items = append(result.Items, ToolBillingItem{
			Name: line.name, Count: line.count, Unit: line.unit,
			UnitPrice: unitPrice, SubtotalQuota: line.floor,
		})
	}
	return result, nil
}

func (m *ToolBillingMeter) TargetToolQuota() (ToolBillingResult, error) {
	if m == nil {
		return ToolBillingResult{}, nil
	}
	return allocateToolQuota(m.exactLines(m.snapshotEvents()))
}

func (m *ToolBillingMeter) TargetToolQuotaIncludingAttempt(attempt *ToolBillingAttempt) (ToolBillingResult, error) {
	if m == nil {
		return ToolBillingResult{}, nil
	}
	if attempt == nil || attempt.parent != m {
		return ToolBillingResult{}, fmt.Errorf("tool billing attempt belongs to another meter")
	}

	attempt.mu.Lock()
	defer attempt.mu.Unlock()
	if attempt.closed {
		return ToolBillingResult{}, fmt.Errorf("tool billing attempt is already closed")
	}
	m.mu.Lock()
	events := make([]confirmedToolEvent, 0, len(m.events)+len(attempt.events))
	for _, event := range m.events {
		events = append(events, event)
	}
	for _, event := range attempt.events {
		duplicate := false
		for _, alias := range event.aliases {
			if _, exists := m.aliases[alias]; exists {
				duplicate = true
				break
			}
		}
		if !duplicate {
			events = append(events, event)
		}
	}
	m.mu.Unlock()
	return allocateToolQuota(m.exactLines(events))
}
