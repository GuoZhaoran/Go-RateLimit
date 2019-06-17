package main

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/garyburd/redigo/redis"
)

const luaScript = `
-- 接口限流
-- last_leaking_time 最后访问时间的毫秒
-- remaining_capacity 当前令牌桶中可用请求令牌数
-- capacity 令牌桶容量
-- leaking_rate	令牌桶添加令牌的速率

-- 把发生数据变更的命令以事务的方式做持久化和主从复制(Redis4.0支持)
redis.replicate_commands()

-- 获取令牌桶的配置信息
local rate_limit_info = redis.call("HGETALL", KEYS[1])

-- 获取当前时间戳
local timestamp = redis.call("TIME")
local now = math.floor((timestamp[1] * 1000000 + timestamp[2]) / 1000)

if rate_limit_info == nil then -- 没有设置限流配置,则默认拿到令牌
	return now * 10 + 1
end

local capacity = tonumber(rate_limit_info[2])
local leaking_rate = tonumber(rate_limit_info[4])
local remaining_capacity = tonumber(rate_limit_info[6])
local last_leaking_time = tonumber(rate_limit_info[8])

-- 计算需要补给的令牌数,更新令牌数和补给时间戳
local supply_token = math.floor((now - last_leaking_time) * leaking_rate)
if (supply_token > 0) then
   last_leaking_time = now
   remaining_capacity = supply_token + remaining_capacity
   if remaining_capacity > capacity then
      remaining_capacity = capacity
   end
end

local result = 0 -- 返回结果是否能够拿到令牌,默认否

-- 计算请求是否能够拿到令牌
if (remaining_capacity > 0) then
	remaining_capacity = remaining_capacity - 1
	result = 1
end

-- 更新令牌桶的配置信息
redis.call("HMSET", KEYS[1], "RemainingCapacity", remaining_capacity, "LastLeakingTime", last_leaking_time)

return now * 10 + result
`

func main() {
	http.HandleFunc("/user/list", handleReq)
	http.ListenAndServe(":8082", nil)
}

//初始化redis连接池
func newPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:   80,
		MaxActive: 12000, // max number of connections
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", ":6379")
			if err != nil {
				panic(err.Error())
			}
			return c, err
		},
	}
}

//写入日志
func writeLog(msg string, logPath string) {
	fd, _ := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	defer fd.Close()
	content := strings.Join([]string{msg, "\r\n"}, "")
	buf := []byte(content)
	fd.Write(buf)
}

//处理请求函数,根据请求将响应结果信息写入日志
func handleReq(w http.ResponseWriter, r *http.Request) {
	//获取url信息
	pathInfo := r.URL.Path
	//获取get传递的公司信息org
	orgInfo, ok := r.URL.Query()["org"]
	if !ok || len(orgInfo) < 1 {
		fmt.Println("Param org is missing!")
	}

	//调用lua脚本原子性进行接口限流统计
	conn := newPool().Get()
	key := orgInfo[0] + pathInfo
	lua := redis.NewScript(1, luaScript)
	reply, err := redis.Int64(lua.Do(conn, key))
	if err != nil {
		fmt.Println(err)
		return
	}
	//接口是否被限制访问
	isLimit := bool(reply % 10 == 1)
	reqTime := int64(math.Floor(float64(reply) / 10))
	//将统计结果写入日志当中
	if !isLimit {
		successLog := strconv.FormatInt(reqTime, 10) + " request failed!"
		writeLog(successLog, "./stat.log")
		return
	}

	failedLog := strconv.FormatInt(reqTime, 10) + " request success!"
	writeLog(failedLog, "./stat.log")
}