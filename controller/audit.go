package controller

import (
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// auditContentTemplates 将稳定的操作标识 action 映射为英文兜底模板，渲染后写入
// Log.Content（供导出 / 经典前端等非本地化消费者使用）。占位符为 ${name}，由该
// action 的 params 填充。本地化展示文案在前端 i18n 模板中维护，本表是语言中立的
// 英文基线——调用方因此无需在每个埋点处手写句子（避免与 params 重复书写同一份值）。
var auditContentTemplates = map[string]string{
	"user.create":                                     "Created user ${username} (role ${role})",
	"user.create.invite":                              "Created user ${username} with inviter ${inviter_id}",
	"user.create.invite_skip":                         "Created user ${username} without invitation code: ${aff_skip_reason}",
	"user.register.invite":                            "User ${username} registered with invitation code ${aff_code} from inviter ${inviter_id}",
	"user.update":                                     "Updated user ${username} (ID: ${id})",
	"user.delete":                                     "Deleted user ${username} (ID: ${id})",
	"user.manage":                                     "Performed ${action} on user ${username} (ID: ${id})",
	"user.disable":                                    "Disabled user ${username} (ID: ${id})",
	"user.enable":                                     "Enabled user ${username} (ID: ${id})",
	"user.role_change":                                "Changed user ${username} role from ${role_from} to ${role_to}",
	"user.quota_add":                                  "Increased user quota by ${quota}",
	"user.quota_subtract":                             "Decreased user quota by ${quota}",
	"user.quota_override":                             "Overrode user quota from ${from} to ${to}",
	"user.binding_clear":                              "Cleared ${bindingType} binding for user ${username}",
	"user.binding_clear_self":                         "Cleared ${binding_type} binding",
	"user.profile_update":                             "Updated personal profile",
	"user.2fa_disable":                                "Force-disabled two-factor authentication for the user",
	"user.passkey_register":                           "Registered a passkey",
	"user.passkey_delete":                             "Deleted a passkey",
	"user.reset_passkey":                              "Reset the user passkey",
	"user.redeem_code":                                "Redeemed a redemption code for ${quota}",
	"user.topup_complete":                             "Completed top-up order ${trade_no}",
	"option.update":                                   "Updated system setting ${key}",
	"option.update.relay_billing":                     "Updated relay billing settings",
	"registration.settings_update":                    "Updated invitation registration settings",
	"source_attribution.settings_update":              "Updated source attribution settings",
	"source_attribution.confirm":                      "Confirmed source attribution ${option_name}",
	"compliance.settings_update":                      "Updated compliance statement settings",
	"compliance.confirm":                              "Confirmed compliance statement ${version}",
	"announcement.acknowledge":                        "Acknowledged the latest announcement",
	"announcement.media_upload":                       "Uploaded an announcement image",
	"announcement.settings_update":                    "Updated announcement settings",
	"security.settings_update":                        "Updated security settings",
	"billing.topup":                                   "Recorded top-up event ${trade_no}",
	"billing.consume":                                 "Recorded consumption event ${request_id}",
	"billing.refund":                                  "Recorded refund event ${request_id}",
	"sensitive.token_balance_query":                   "Queried API token balance ${masked_key} (${result})",
	"sensitive.ip_audit_view":                         "Viewed IP audit details",
	"playground.web_search":                           "Used playground web search with model ${model}",
	"playground.image_generation":                     "Generated playground image with model ${model}",
	"invitation.application_created":                  "User ${target_user_id} submitted invitation application ${application_id}",
	"invitation.application_withdrawn":                "User ${target_user_id} withdrew invitation application ${application_id}",
	"invitation.application_reviewed":                 "Reviewed invitation application ${application_id} for user ${target_user_id}: ${status}",
	"invitation.code_issued":                          "Issued invitation code ${invite_code_id} to user ${target_user_id}",
	"invitation.code_revoked":                         "Revoked invitation code ${invite_code_id} for user ${target_user_id}",
	"invitation.code_used":                            "Invitation code ${invite_code_id} registered user ${registered_user_id}",
	"invitation.reward_review":                        "Reviewed invitation reward ${payout_id} for user ${target_user_id}: ${status}",
	"invitation.reward_withdraw":                      "User ${target_user_id} withdrew ${amount} from approved invitation rewards",
	"invitation.reward_payout_submitted":              "User ${target_user_id} submitted invitation reward payout request ${request_id} for ${amount} across ${item_count} rewards",
	"invitation.reward_payout_reviewed":               "Reviewed invitation reward payout request ${request_id} for user ${target_user_id}: ${status}",
	"invitation.reward_payout_frozen":                 "Froze invitation reward payout request ${request_id} for user ${target_user_id}: ${amount}",
	"invitation.reward_payout_unfrozen":               "Unfroze invitation reward payout request ${request_id} for user ${target_user_id}: ${amount}",
	"invitation.incentive_settings_update":            "Updated invitation incentive settings",
	"token.custom_key_collision_suspected_compromise": "Disabled API key ${masked_key} after a custom key collision",
	"token.create":                                    "Created API token ${name} (ID: ${token_id})",
	"token.update":                                    "Updated API token ${name} (ID: ${token_id})",
	"token.status_update":                             "Updated API token ${name} status to ${status}",
	"token.delete":                                    "Deleted API token ${name} (ID: ${token_id})",
	"token.key_view":                                  "Viewed API token key ${name} (ID: ${token_id})",
	"token.secret_reveal":                             "Viewed API token key ${name} (ID: ${token_id})",
	"token.secret_reveal_batch":                       "Viewed ${item_count} API token keys",
	"token.secret_reveal_verify":                      "Verified password for API token key access",

	"channel.create":              "Created channel ${name} (type ${type}, count ${count})",
	"channel.update":              "Updated channel ${name} (ID: ${id})",
	"channel.status_update":       "Updated channel ${id} status to ${status}",
	"channel.status_update_batch": "Batch updated ${count}/${total} channels to status ${status}",
	"channel.delete":              "Deleted channel ${name} (ID: ${id})",
	"channel.delete_batch":        "Batch deleted ${count} channels",
	"channel.delete_disabled":     "Deleted all disabled channels (${count})",
	"channel.key_view":            "Viewed channel key ${name} (ID: ${id})",
	"channel.tag_disable":         "Disabled channels with tag ${tag}",
	"channel.tag_enable":          "Enabled channels with tag ${tag}",
	"channel.tag_edit":            "Edited channels with tag ${tag}",
	"channel.tag_batch_set":       "Batch set tag for ${count} channels",
	"channel.copy":                "Copied channel (source ID: ${sourceId}) to ${name} (new ID: ${id})",
	"channel.multi_key_manage":    "Multi-key management ${action} on channel (ID: ${id})",
	"channel.upstream_apply":      "Applied upstream model changes to channel (ID: ${id})",
	"channel.upstream_apply_all":  "Applied upstream model changes to ${count} channels",
	"channel.upstream_detect_all": "Queued upstream model detection task ${task_id}",

	"redemption.create":         "Created ${count} redemption codes named ${name} (${quota} each)",
	"redemption.view":           "Viewed redemption code ${id}",
	"redemption.update":         "Updated redemption code ${id}",
	"redemption.status_update":  "Updated redemption code ${id} status to ${status}",
	"redemption.delete":         "Deleted redemption code ${id}",
	"redemption.delete_invalid": "Deleted invalid redemption codes (${count})",

	"subscription.plan_reset":      "Reset active subscriptions for plan ${plan_id}",
	"subscription.user_plan_reset": "Reset active plan ${plan_id} subscriptions for user ${target_user_id}",
}

// auditContentEN 按 action 模板渲染英文兜底文本；未登记的 action 退回 action 本身。
func auditContentEN(action string, params map[string]interface{}) string {
	tmpl, ok := auditContentTemplates[action]
	if !ok {
		return action
	}
	return os.Expand(tmpl, func(key string) string {
		if v, ok := params[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	})
}

// auditOperatorInfo 从上下文构建操作者身份信息（管理员 id/用户名/角色）。
func auditOperatorInfo(c *gin.Context) map[string]interface{} {
	return map[string]interface{}{
		"admin_id":       c.GetInt("id"),
		"admin_username": c.GetString("username"),
		"admin_role":     c.GetInt("role"),
		"auth_method":    auditAuthMethod(c),
	}
}

func auditAuthMethod(c *gin.Context) string {
	if c.GetBool("use_access_token") {
		return "access_token"
	}
	return "session"
}

// markAuditLogged 标记当前请求已在 handler 内手动记录审计日志，
// 使鉴权链路中的审计兜底（finishAdminAudit）跳过兜底记录，避免重复。
func markAuditLogged(c *gin.Context) {
	common.SetContextKey(c, constant.ContextKeyAuditLogged, true)
}

// recordManageAudit 记录一条由操作者本人归属的管理/高危审计日志（资源类操作：
// 渠道 / 系统设置 / 兑换码等）。content 由 action+params 自动渲染。
func recordManageAudit(c *gin.Context, action string, params map[string]interface{}) {
	recordManageAuditFor(c, c.GetInt("id"), action, params)
}

// recordManageAuditFor 记录一条管理审计日志，日志归属于操作者；targetUserId
// 只表示被操作用户，用于在结构化参数中保留目标上下文。
func recordManageAuditFor(c *gin.Context, targetUserId int, action string, params map[string]interface{}) {
	if params == nil {
		params = map[string]interface{}{}
	}
	operatorUserId := c.GetInt("id")
	if _, ok := params["target_user_id"]; !ok && targetUserId > 0 && targetUserId != operatorUserId {
		params["target_user_id"] = targetUserId
	}
	model.RecordOperationAuditLog(operatorUserId, auditContentEN(action, params), c.ClientIP(), action, params, auditOperatorInfo(c), nil)
	markAuditLogged(c)
}

// recordUserSecurityAudit 记录普通用户自己的安全敏感操作（如 passkey 绑定/解绑）。
// 这类日志没有管理员操作者，不写 admin_info；同时不依赖 AdminAuth/RootAuth 的兜底。
func recordUserSecurityAudit(c *gin.Context, userId int, action string, params map[string]interface{}) {
	model.RecordOperationAuditLog(userId, auditContentEN(action, params), c.ClientIP(), action, params, nil, nil)
}
