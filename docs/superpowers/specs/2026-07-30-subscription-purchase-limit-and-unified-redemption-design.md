# 订阅限购加固与统一兑换入口设计

## 状态与范围

- 日期：2026-07-30
- 基线：Clove API `v1.0.1-beta`
- 交付：本地源码、保留数据库兼容迁移、本地运行、自动化验收与 Git 提交
- 排除：生产部署、远端推送、Release，以及改变既有兑换码内容或订阅权益

本设计修复有限购套餐可通过不同购买入口、历史删除或旧兑换码快照绕过的问题，并把钱包中的余额兑换与订阅兑换合并为一个自动识别输入框。旧 API 保持兼容，已有兑换码和订阅数据原样保留。

## 不可变规则

1. `max_purchase_per_user = 0` 表示不限购；正数表示同一用户对同一套餐的终身成功取得次数上限。
2. 余额购买、在线支付、管理员绑定和订阅兑换码领取都进入同一个计数器，不能各自计数。
3. 订阅到期、停用、退款或管理员硬删除不释放已经占用的次数；提高上限后可继续购买，降低上限后立即阻止新增取得。
4. 所有入口在同一数据库事务内先锁用户、核对并占用次数，再创建订阅；事务失败时占用一并回滚。
5. 兑换码创建只是购买待领取权益，不占创建者的个人次数；每次实际领取才占领取者的一次。
6. 旧订阅兑换码使用“创建时快照与当前套餐限制中更严格的正数限制”。任一为 `0`、另一为正数时使用正数；两者均为 `0` 时不限购。
7. 兑换入口自动识别管理员余额码、用户余额码和订阅码，不让用户选择类型，也不依赖可伪造的前端类型字段。

## 终身购买计数

新增 `subscription_purchase_counters`：

```text
user_id        integer, primary key part 1
plan_id        integer, primary key part 2
used_count     integer, not null, non-negative
created_at     bigint
updated_at     bigint
```

`CreateUserSubscriptionFromPlanTx` 是唯一占用入口。它保留现有用户行锁，在创建 `UserSubscription` 前读取或创建计数行；有限购时检查 `used_count < effective_limit`，随后原子加一。即使当前套餐不限购，也记录成功取得次数，使管理员以后启用限购时已有历史仍被计算。调用者不再自行用 `user_subscriptions` 行数做最终裁决；页面展示和预检查读取相同计数服务，但事务内检查始终是最终权威。

数据库升级后执行幂等回填。每个用户与套餐的初始次数取以下证据能够证明的最大值，避免同一购买被多表重复相加：

- 当前 `user_subscriptions` 数量；
- 已成功的 `subscription_orders` 数量加订阅兑换 `user_redemption_claims` 数量；
- 已存在计数值。

回填只增加计数、不减少、不删除业务行。历史上已经硬删除且没有订单或领取记录的管理员绑定缺少可恢复证据，不伪造次数；升级后的所有入口均完整记录。

## 订阅兑换限购

领取事务继续按“套餐、用户、兑换码”锁顺序执行。加载权益快照后计算：

```text
effective_limit = stricterPositive(entitlement.max_purchase_per_user,
                                   current_plan.max_purchase_per_user)
```

构造订阅快照时只把 `effective_limit` 交给统一创建函数。删除领取者已有订阅、领取另一个兑换码、同时发起余额购买和兑换、或者并发领取多个码，都共享同一用户锁与计数行，因此最多一个事务越过最后一个可用名额。限购失败不会消耗兑换码份数、不会创建领取记录、不会触发返佣，也不会改变钱包。

## 统一兑换 API

新增认证接口：

```http
POST /api/redemption/claim
Content-Type: application/json

{"code":"CODE"}
```

服务端先按唯一兑换码值查找 `user_redemption_codes`；存在时由其真实 `reward_type` 路由到余额或订阅领取，不存在时尝试现有管理员 `redemptions`。两张表已有跨表唯一性校验，查找顺序不会造成歧义。响应使用统一判别结构：

```json
{
  "reward_type": "balance",
  "source": "administrator",
  "quota": 500000,
  "subscription_id": 0
}
```

订阅成功返回 `reward_type=subscription` 与真实 `subscription_id`；余额成功返回实际 `quota`。不存在、失效、类型损坏或竞争失败使用现有规范化错误，不向未授权用户泄露兑换码所属用户或套餐细节。接口沿用关键写限流、身份鉴权、事务、审计和幂等规则。

旧 `/api/user/topup` 与 `/api/user-redemption/claim` 继续服务旧客户端；前者仍只处理管理员余额码，后者仍可接受旧的 `expected_reward_type`。新页面只调用统一接口。

## 钱包界面

Default 与 Classic 删除“余额 / 订阅”分段控件，只保留一个兑换码输入框和一个兑换按钮。占位文字统一为“输入兑换码”，提交成功后根据服务端 `reward_type` 显示余额到账或订阅领取结果，并刷新用户余额与订阅数据。按钮、输入框和加载态维持稳定尺寸；移动端不得横向溢出。

用户兑换码管理页的创建类型、列表筛选和审计仍然保留，因为那是创建与管理语义，不属于钱包领取入口的重复选择。

## 审计日志展示

后端现有订阅兑换事件键保持稳定，不改写历史日志内容。Default 与 Classic 的共享审计模板清单补齐以下映射，并为全部现有语言补齐文案：

```text
user.redemption.subscription.create
user.redemption.subscription.claim
user.redemption.subscription.plan_disabled
user.redemption.subscription.refund_pending
user.redemption.subscription.refund
user.redemption.subscription.forfeit
user.redemption.cancel_pending
user.redemption.expire_pending
```

展示层必须把 `user.redemption.subscription.claim` 显示为明确的“领取订阅兑换码”操作，并保留兑换码、套餐和订阅等现有结构化参数；不得把未知事件键直接当成用户可见文案。测试从后端实际发出的事件集合校验两套前端覆盖率，防止以后新增事件再次落入“未识别的操作”。一项业务动作仍只写一条业务审计，不为了修复显示而复制日志。

## 错误与兼容

- 限购冲突返回现有“已达到该套餐购买上限”语义，并映射为稳定错误码。
- 计数占用、订阅创建、兑换码份数、领取记录和返佣必须同事务成功或全部回滚。
- 回填或迁移失败时服务不进入可写状态，保留原数据库，不以清表或重建方式继续。
- 统一接口失败不得回退为第二次提交另一接口，避免竞态双兑。
- 前端刷新失败不回滚已经成功的服务端兑换，只提示刷新并允许重新加载页面。

## 测试与验收

- 同一用户依次通过余额、在线支付、管理员绑定和不同订阅兑换码取得套餐，合计不超过限制。
- 并发余额购买与兑换、并发不同兑换码领取、并发管理员绑定与兑换均只有剩余名额数量的请求成功。
- 当前限制比快照严格、快照比当前严格、仅一方有限制和双方不限购四种组合。
- 订阅到期、退款和硬删除后仍不能重新购买；提高上限后可以购买新增名额。
- 失败事务不增加计数；重复回调和重复领取不重复占用。
- SQLite、MySQL、PostgreSQL 的迁移、回填、锁与计数行为通过测试。
- 统一接口覆盖管理员余额码、用户余额码、订阅码、未知码、禁用码、已用尽码和限购错误。
- Default 与 Classic 单测、类型检查和生产构建通过；Playwright 在桌面和移动端验证单输入框、三种码自动识别和无重复提交。
- Default 与 Classic 对全部订阅兑换事件显示本地化操作名称，现有 `user.redemption.subscription.claim` 历史记录不再显示“未识别的操作”。
- 使用保留本地数据库启动前后比较用户、余额、订单、订阅、兑换码、领取和钱包流水基线，除测试夹具外不得变化。

## 完成标准

- 所有订阅创建路径只通过一个事务级限购占用点。
- 删除或过期订阅不再恢复购买名额，旧兑换码不能绕过后来启用或收紧的限制。
- 钱包中没有兑换类型选择，单输入框可兑换三种现有码。
- 旧接口和旧数据保持兼容，完整测试通过，生产环境未修改。
