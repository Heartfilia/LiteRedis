package redis

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"LiteRedis/backend/config"
	"github.com/redis/go-redis/v9"
)

const checkedStringScript = `
local kind = redis.call('TYPE', KEYS[1]).ok
if kind ~= 'string' then return 0 end
if redis.call('STRLEN', KEYS[1]) > tonumber(ARGV[3]) then return 0 end
local current = redis.call('GET', KEYS[1])
if redis.sha1hex(current) ~= ARGV[1] then return 0 end
local ttl = redis.call('PTTL', KEYS[1])
if ttl == -2 then return 0 end
if ttl >= 0 then
  redis.call('SET', KEYS[1], ARGV[2], 'PX', math.max(1, ttl))
else
  redis.call('SET', KEYS[1], ARGV[2])
end
return 1`

const checkedHashScript = `
if redis.call('TYPE', KEYS[1]).ok ~= 'hash' then return 0 end
local current = redis.call('HGET', KEYS[1], ARGV[1])
if ARGV[2] == '1' then
  if not current or current ~= ARGV[3] then return 0 end
else
  if current then return 0 end
end
redis.call('HSET', KEYS[1], ARGV[1], ARGV[4])
return 1`

func checkedEditResult(value int64, err error) config.EditResult {
	if err != nil {
		code := RedisErrorCode(err)
		return config.EditResult{Code: code, Message: err.Error()}
	}
	if value != 1 {
		return config.EditResult{Code: "conflict", Message: "Data changed or expired; refresh before saving"}
	}
	return config.EditResult{Success: true}
}

// RedisErrorCode 区分明确的服务端拒绝和传输错误。后者不能证明写入未执行。
func RedisErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var invalid *InvalidExportError
	if errors.As(err, &invalid) {
		return "invalid"
	}
	if errors.Is(err, redis.TxFailedErr) {
		return "conflict"
	}
	if errors.Is(err, errKeyAlreadyExists) || errors.Is(err, errSetMemberNotFound) || errors.Is(err, errZSetMemberNotFound) || errors.Is(err, errStringKeyNotFound) {
		return "conflict"
	}
	text := strings.ToUpper(err.Error())
	// Lua runtime errors do not roll back earlier writes. Never report these as
	// definitely unapplied, including ACL failures after another script command.
	if strings.Contains(text, "USER_SCRIPT") || strings.Contains(text, "@USER_SCRIPT") || strings.Contains(text, "ERROR RUNNING SCRIPT") || strings.Contains(text, "ACL FAILURE IN SCRIPT") {
		return "partial_possible"
	}
	if strings.HasPrefix(text, "MOVED ") || strings.HasPrefix(text, "ASK ") {
		return "cluster_redirect"
	}
	if strings.Contains(text, "NOPERM") || strings.Contains(text, "NOAUTH") {
		return "permission_denied"
	}
	if isUnknownCommandError(err) {
		return "unsupported"
	}
	var serverErr redis.Error
	if errors.As(err, &serverErr) {
		return "rejected"
	}
	return "outcome_unknown"
}

func SetStringChecked(ctx context.Context, client redis.UniversalClient, key, value, token string) config.EditResult {
	if len(value) > maxStringValueBytes || !utf8.ValidString(value) || len(token) != 40 {
		return config.EditResult{Code: "invalid", Message: "Full editable preview required; maximum 5MiB UTF-8"}
	}
	n, err := client.Eval(ctx, checkedStringScript, []string{key}, token, value, maxStringValueBytes).Int64()
	result := checkedEditResult(n, err)
	if result.Success {
		result.EditToken = stringEditToken(value)
	}
	return result
}

func HSetChecked(ctx context.Context, client redis.UniversalClient, key, field, value, expected string, existed bool) config.EditResult {
	if len(value) > maxStringValueBytes || len(expected) > maxStringValueBytes || !utf8.ValidString(value) || !utf8.ValidString(expected) {
		return config.EditResult{Code: "invalid", Message: "Maximum 5MiB UTF-8 field value"}
	}
	flag := "0"
	if existed {
		flag = "1"
	}
	n, err := client.Eval(ctx, checkedHashScript, []string{key}, field, flag, expected, value).Int64()
	return checkedEditResult(n, err)
}
