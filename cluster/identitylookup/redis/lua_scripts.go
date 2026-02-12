package redis

// Lua scripts for atomic Redis operations.
//
// These scripts are evaluated using EVALSHA (with automatic fallback to EVAL)
// via the go-redis scripting API, ensuring that multi-step operations are
// executed atomically on the Redis server.

// tryAcquireLockScript atomically acquires a spawn lock for a cluster identity.
//
// KEYS[1] = identity key (e.g. "mycluster:ci:kind/identity")
// ARGV[1] = lock ID (UUID)
// ARGV[2] = lock TTL in milliseconds
//
// Returns 1 if the lock was acquired, 0 if the key already exists.
//
// The script uses HSETNX to set the lock ID only if the key does not exist,
// then sets an expiry to prevent stale locks.
const tryAcquireLockScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then
  return 0
end
redis.call('HSET', KEYS[1], 'lid', ARGV[1])
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return 1
`

// storeActivationScript atomically stores an activation after verifying lock ownership.
//
// KEYS[1] = identity key (e.g. "mycluster:ci:kind/identity")
// KEYS[2] = member set key (e.g. "mycluster:mb:memberID")
// ARGV[1] = expected lock ID
// ARGV[2] = PID ID
// ARGV[3] = PID address
// ARGV[4] = member ID
//
// Returns 1 if the activation was stored, 0 if the lock ID does not match.
//
// The script verifies lock ownership, sets the hash fields (pid, adr, mid),
// clears the lock field, persists the key (removes TTL), and adds the
// identity key to the member's set.
const storeActivationScript = `
local lid = redis.call('HGET', KEYS[1], 'lid')
if lid ~= ARGV[1] then
  return 0
end
redis.call('HSET', KEYS[1], 'pid', ARGV[2], 'adr', ARGV[3], 'mid', ARGV[4], 'lid', '')
redis.call('PERSIST', KEYS[1])
redis.call('SADD', KEYS[2], KEYS[1])
return 1
`

// removeActivationScript atomically removes an activation after verifying PID ownership.
//
// KEYS[1] = identity key (e.g. "mycluster:ci:kind/identity")
// ARGV[1] = expected PID ID
// ARGV[2] = expected PID address
// ARGV[3] = member key prefix (e.g. "mycluster:mb:")
//
// Returns 1 if the activation was removed, 0 if the PID does not match.
//
// The script reads the stored pid and address, verifies they match, then
// removes the identity key from the member set and deletes the identity key.
const removeActivationScript = `
local fields = redis.call('HMGET', KEYS[1], 'pid', 'adr', 'mid')
if fields[1] ~= ARGV[1] or fields[2] ~= ARGV[2] then
  return 0
end
local memberKey = ARGV[3] .. fields[3]
redis.call('SREM', memberKey, KEYS[1])
return redis.call('DEL', KEYS[1])
`

// removeMemberScript removes all activations belonging to a member.
//
// KEYS[1] = member set key (e.g. "mycluster:mb:memberID")
//
// The script iterates over the member set using SSCAN, deletes each identity
// key referenced in the set, then deletes the member set itself.
const removeMemberScript = `
local cursor = '0'
repeat
  local rep = redis.call('SSCAN', KEYS[1], cursor)
  cursor = rep[1]
  if #rep[2] > 0 then
    redis.call('DEL', unpack(rep[2]))
  end
until cursor == '0'
redis.call('DEL', KEYS[1])
return 1
`

// removeLockScript atomically removes a lock only if the lock ID matches.
//
// KEYS[1] = identity key
// ARGV[1] = expected lock ID
//
// Returns 1 if the lock was removed, 0 if it did not match or key was gone.
const removeLockScript = `
local lid = redis.call('HGET', KEYS[1], 'lid')
if lid ~= ARGV[1] then
  return 0
end
return redis.call('DEL', KEYS[1])
`
