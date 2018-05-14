package cache2go

import (
    "sync"
    "time"
)

//定义 CacheItem 类型 struct 类型
type CacheItem struct {
    sync.RWMutex

    //定义缓存key 为 接口interface类型
    key interface{}

    //定义缓存value 为 接口interface类型
    data interface{}

    //定义缓存的过期时间，time.Duration 类型
    lifeSpan time.Duration

    //缓存的创建时间，时间戳
    createdOn time.Time

    //缓存上次访问时间，时间戳
    accessedOn time.Time

    //缓存被访问的次数
    accessCount int64

    //缓存项被删除之前执行的回调函数
    aboutToExpire func(key interface{})
}

//初始化一个 CacheItem 类型的变量，并返回该变量(CacheItem类型)的指针
func NewCacheItem(key interface{}, lifeSpan time.Duration, data interface{}) *CacheItem {
    t := time.Now()
    return &CacheItem{
        key:           key,
        lifeSpan:      lifeSpan,
        createdOn:     t,
        accessedOn:    t,
        accessCount:   0,
        aboutToExpire: nil,
        data:          data,
    }
}

//每次访问后，更新缓存key的最后访问时间，访问总次数，维活缓存key
func (item *CacheItem) KeepAlive() {
    item.Lock()
    defer item.Unlock()
    item.accessedOn = time.Now()
    item.accessCount++
}

//返回缓存key的生命期
func (item *CacheItem) LifeSpan() time.Duration {
    return item.lifeSpan
}

//返回缓存key的上次访问时间
func (item *CacheItem) AccessedOn() time.Time {
    item.Lock()
    defer item.Unlock()
    return item.accessedOn
}

//返回缓存key的创建时间
func (item *CacheItem) CreatedOn() time.Time {
    return item.createdOn
}

//返回缓存key的访问次数
func (item *CacheItem) AccessCount() int64 {
    item.Lock()
    defer item.Unlock()
    return item.accessCount
}

//返回缓存记录key
func (item *CacheItem) Key() interface{} {
    return item.key
}

//返回缓存记录的value
func (item *CacheItem) Data() interface{} {
    return item.data
}

//设置缓存key被删除时的回调函数，回调函数会在缓存被删除之前调用
func (item *CacheItem) SetAboutToExpireCallback(f func(interface{})) {
    item.Lock()
    defer item.Unlock()
    item.aboutToExpire = f
}
