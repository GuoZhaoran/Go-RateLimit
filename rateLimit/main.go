package main

import (
	"advanceGo/rateLimit/funnel"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/garyburd/redigo/redis"
)

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

//获得公司接口限流配置信息
func getOrgApiRateLimitInfo(org string, pathInfo string) (*funnel.Funnel, bool) {
	conn :=  newPool().Get()
	defer conn.Close()
	funnelMsg := &funnel.Funnel{}
	pathInfo = org + pathInfo
	cfg, err := redis.Values(conn.Do("HGETALL", pathInfo))
	if err != nil {
		fmt.Println(err)
		return funnelMsg, false
	}
	if len(cfg) > 0 {
		err = redis.ScanStruct(cfg, funnelMsg)
		if err != nil {
			fmt.Println(err)
			return funnelMsg, false
		}
	}

	return funnelMsg, true
}

//更新设置公司接口限流配置信息
func updateOrgApiRateLimitInfo(funnel *funnel.Funnel, org string, pathInfo string) bool {
	conn :=  newPool().Get()
	defer conn.Close()
	pathInfo = org + pathInfo
	conn.Send("HMSET", pathInfo, "RemainingCapacity", funnel.RemainingCapacity,
		"LastLeakingTime", funnel.LastLeakingTime)
	conn.Flush()
	conn.Receive()

	return true
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
	funnelMsg, isLimit := getOrgApiRateLimitInfo(orgInfo[0], pathInfo)
	if isLimit {
		ret := funnelMsg.IsActionAllowed()
		//更新接口funnel信息
		updateOrgApiRateLimitInfo(funnelMsg, orgInfo[0], pathInfo)
		if ret {
			successMsg := strconv.FormatInt(funnelMsg.LastLeakingTime, 10) + " requess success!"
			writeLog(successMsg, "./stat.log")
			return
		}
	}

	failedMsg := strconv.FormatInt(funnelMsg.LastLeakingTime, 10) + " request failed!"
	writeLog(failedMsg, "./stat.log")
}
