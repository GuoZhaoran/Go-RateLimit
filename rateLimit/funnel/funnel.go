package funnel

import (
	"math"
	"time"
)

type Funnel struct {
	Capacity          int64   //漏斗容量
	LeakingRate       float64 //漏斗流水速率:每毫秒控制接收多少个请求
	RemainingCapacity int64   //漏斗剩余空间
	LastLeakingTime   int64   //上次流水时间:毫秒时间戳
}

//有请求时更新漏斗的状态,主要是漏斗剩余空间和上次流水时间
func (rateLimit *Funnel) updateFunnelStatus() {
	nowTs := time.Now().UnixNano() / int64(time.Millisecond)
	//距离上一次漏水已经过去了多长时间
	timeDiff := nowTs - rateLimit.LastLeakingTime
	//根据时间差和流水速率计算需要向漏斗中添加多少水
	needAddSpace := int64(math.Floor(rateLimit.LeakingRate * float64(timeDiff)))
	//不需要添加水
	if needAddSpace < 1 {
		return
	}
	rateLimit.RemainingCapacity += needAddSpace
	//添加的水不能大于漏斗的剩余空间
	if rateLimit.RemainingCapacity > rateLimit.Capacity {
		rateLimit.RemainingCapacity = rateLimit.Capacity
	}
	//更新上次漏斗流水时间戳
	rateLimit.LastLeakingTime = nowTs
}

//判断接口是否被限流
func (rateLimit *Funnel) IsActionAllowed() bool {
	//更新漏斗状态
	rateLimit.updateFunnelStatus()
	if rateLimit.RemainingCapacity < 1 {
		return false
	}
	rateLimit.RemainingCapacity = rateLimit.RemainingCapacity - 1
	return true
}
