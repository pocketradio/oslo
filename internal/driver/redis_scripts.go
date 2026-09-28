package driver

import "github.com/redis/go-redis/v9"

// keys1 is the resv key of driver. GET will get the token stored at the key.
// thats compared with the caller key. if matched, key deleted.
// else, nothing deleted. ( the ttl will remove it later anyway )

var releaseReservationScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0
`)

// keys1 is driver:<driverID> ( the key ; value is a redis hash containing available, lat, long, lastseen_ms) ,
// so first is the key availability check
// then ensuring that the location is fresh by checking if last seen is within the past 30s window ( argv3 = t.now minus 30s )

// then the distributed lock, ( nx prev another worker from reserving the same driver , px for automatic expiry )
// key2 is the resrv key, argv1 is the token. (NX ensures it doesnt exist)
// so that'll create the <reskey : restoken> if the key doesnt already exist, with the px 10s TTL expiration

// px ensures its not a permanent lock in case of no response

var reserveDriverScript = redis.NewScript(`
if redis.call("HGET", KEYS[1], "available") ~= "true" then
    return 0
end
local last_seen = redis.call("HGET", KEYS[1], "last_seen_ms")
if not last_seen or tonumber(last_seen) <= tonumber(ARGV[3]) then
    return 0
end
if redis.call("SET", KEYS[2], ARGV[1], "NX", "PX", ARGV[2]) then
    return 1
end
return 0
`)

// keys1 is the driver hash. keys2 is the geo index. argv1 is cutoff timestamp
// argv2 is driverID

var removeStaleDriverScript = redis.NewScript(`
local last_seen = redis.call("HGET", KEYS[1], "last_seen_ms")
if last_seen and tonumber(last_seen) <= tonumber(ARGV[1]) then
    return redis.call("ZREM", KEYS[2], ARGV[2])
end
return 0
`)
