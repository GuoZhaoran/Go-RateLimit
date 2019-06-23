package funnel

import (
	"math"
	"time"
)

type Funnel struct {
	Capacity          int64   //令牌桶容量
	LeakingRate       float64 //令牌桶流水速率:每毫秒向令牌桶中添加的令牌数
	RemainingCapacity int64   //令牌桶剩余空间
	LastLeakingTime   int64   //上次流水(放入令牌)时间:毫秒时间戳
}

//有请求时更新令牌桶的状态,主要是令牌桶剩余空间和记录取走Token的时间戳
func (rateLimit *Funnel) updateFunnelStatus() {
	nowTs := time.Now().UnixNano() / int64(time.Millisecond)
	//距离上一次取走令牌已经过去了多长时间
	timeDiff := nowTs - rateLimit.LastLeakingTime
	//根据时间差和流水速率计算需要向令牌桶中添加多少令牌
	needAddSpace := int64(math.Floor(rateLimit.LeakingRate * float64(timeDiff)))
	//不需要添加令牌
	if needAddSpace < 1 {
		return
	}
	rateLimit.RemainingCapacity += needAddSpace
	//添加的令牌不能大于令牌桶的剩余空间
	if rateLimit.RemainingCapacity > rateLimit.Capacity {
		rateLimit.RemainingCapacity = rateLimit.Capacity
	}
	//更新上次令牌桶流水(上次添加令牌)时间戳
	rateLimit.LastLeakingTime = nowTs
}

//判断接口是否被限流
func (rateLimit *Funnel) IsActionAllowed() bool {
	//更新令牌桶状态
	rateLimit.updateFunnelStatus()
	if rateLimit.RemainingCapacity < 1 {
		return false
	}
	rateLimit.RemainingCapacity = rateLimit.RemainingCapacity - 1
	return true
}
