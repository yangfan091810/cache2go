package cache2go

import (
    "log"
    "sort"
    "time"
    "sync"
)

//缓存表 cachetable 结构
type CacheTable struct {
    sync.RWMutex
    // 缓存表名称
    name string
    //所有缓存记录
    items map[interface{}]*CacheItem
    // 触发缓存清理的定时器
    cleanupTimer *time.Timer
    // 缓存清理周期
    cleanupInterval time.Duration
    //缓存表日志
    logger *log.Logger
    //访问不存在的key时的回调函数
    loadData func(key interface{}, args ...interface{}) *CacheItem
    //添加一个新的缓存key时的回调函数
    addedItem func(item *CacheItem)
    //删除任一条记录时的回调函数
    aboutToDeleteItem func(item *CacheItem)
}

//返回缓存表中的缓存记录总条数
func (table *CacheTable) Count() int {
    table.Lock()
    defer table.Unlock()
    return len(table.items)
}

//循环遍历缓存中所有记录，并对记录执行某操作
func (table *CacheTable) Foreach(trans func(key interface{}, value *CacheItem)) {
    table.Lock()
    defer table.Unlock()
    for k, v := range table.items {
        trans(k, v)
    }
}

//设置访问不存在的缓存key时的回调函数
func (table *CacheTable) SetDataLoader(f func(interface{}, ...interface{}) *CacheItem) {
    table.Lock()
    defer table.Unlock()
    table.loadData = f
}

//设置添加新的缓存item时的回调函数
func (table *CacheTable) SetAddedItemCallback(f func(*CacheItem)) {
    table.Lock()
    defer table.Unlock()
    table.addedItem = f
}

//设置缓存记录被删除时执行的回调函数
func (table *CacheTable) SetAboutToDeleteItemCallback(f func(*CacheItem)) {
    table.Lock()
    defer table.Unlock()
    table.aboutToDeleteItem = f
}

//设置缓存表日志
func (table *CacheTable) SetLogger(logger *log.Logger) {
    table.Lock()
    defer table.Unlock()
    table.logger = logger
}

//缓存过期检查
//代码中会去遍历所有缓存项，找到最快要被淘汰掉的缓存项的的时间作为cleanupInterval，即下一次启动缓存刷新的时间，从而保证可以及时的更新缓存，
//可以看到其实质就是自调节下一次启动缓存更新的时间。另外我们也注意到，如果lifeSpan设置为0的话，就不会被淘汰，即永久有效
func (table *CacheTable) expirationCheck() {
    table.Lock()
    if table.cleanupTimer != nil {
        table.cleanupTimer.Stop()
    }
    //检查清理缓存周期是否大于0，
    if table.cleanupInterval > 0 {
        table.log("Expiration check triggered after", table.cleanupInterval, "for table", table.name)
    } else {
        table.log("Expiration check installed for table", table.name)
    }

    now := time.Now()
    //设置最小检查缓存过期周期为 0 
    smallestDuration := 0 * time.Second
    //循环缓存map，检查是否过期
    for key, item := range table.items {
        item.RLock()
        lifeSpan := item.lifeSpan
        accessedOn := item.accessedOn
        item.RUnlock()
        //如果缓存记录的 lifeSpan 设置为0，则永久不过期
        if lifeSpan == 0 {
            continue
        }
        if now.Sub(accessedOn) >= lifeSpan { //已过期的缓存记录，清理掉
            table.deleteInternal(key)
        } else {
            //更新最小检查缓存过期周期时间
            if smallestDuration == 0 || lifeSpan-now.Sub(accessedOn) < smallestDuration {
                smallestDuration = lifeSpan - now.Sub(accessedOn)
            }
        }
    }
    //更新缓存表的过期周期检查时间
    table.cleanupInterval = smallestDuration
    if smallestDuration > 0 { //smallestDuration 时长后开启单独的goroutine执行缓存过期检查
        table.cleanupTimer = time.AfterFunc(smallestDuration, func() {
            go table.expirationCheck()
        })
    }
    table.Unlock()
}

//添加新的缓存item，该方法包外部不可调用
func (table *CacheTable) addInternal(item *CacheItem) {
    //注意：不要运行该方法，除非缓存表被锁定
    table.log("Adding item with key", item.key, "and lifespan of", item.lifeSpan, "to table", table.name)
    table.items[item.key] = item
    expDur := table.cleanupInterval
    addedItem := table.addedItem
    table.Unlock()
    //执行添加缓存item的回调函数
    if table.addedItem != nil {
        addedItem(item)
    }
    //添加完新的缓存，检查该item的生存周期，并更新缓存表table的检查缓存生存周期项 cleanupInterval
    if item.lifeSpan >0 && (expDur == 0 || item.lifeSpan < expDur) {
        table.expirationCheck()
    }
}

//添加缓存
func (table *CacheTable) Add(key interface{}, lifeSpan time.Duration, data interface{}) *CacheItem {
    item := NewCacheItem(key, lifeSpan, data)
    table.Lock()
    table.addInternal(item)
    return item
}

//删除缓存项item, 该方法包外部不可调用
func (table *CacheTable) deleteInternal(key interface{}) (*CacheItem, error) {
    r, ok := table.items[key]
    if !ok {
        return nil, ErrKeyNotFound
    }
    //检查删除缓存项的回调函数是否为nil，不为nil,则调用回调函数
    aboutToDeleteItem := table.aboutToDeleteItem
    table.Unlock()
    if aboutToDeleteItem != nil {
        aboutToDeleteItem(r)
    }
    r.RLock()
    defer r.RUnlock()
    //检查缓存项删除回调函数是否为nil，不为nil，则调用回调函数
    if r.aboutToExpire != nil {
        r.aboutToExpire(key)
    }
    table.Lock()
    table.log("Deleting item with key", key, "created on", r.createdOn, "and hit", r.accessCount, "times from table", table.name)
    delete(table.items, key)
    return r, nil
}

//删除缓存项
func (table *CacheTable) Delete(key interface{}) (*CacheItem, error) {
    table.Lock()
    defer table.Unlock()
    return table.deleteInternal(key)
}

//检查缓存项是否存在
func (table *CacheTable) Exists(key interface{}) bool {
    table.RLock()
    defer table.RUnlock()
    _, ok := table.items[key]
    return ok
}

//检查缓存项是否存在，如果不存在则添加该缓存
func (table *CacheTable) NotFoundAdd(key interface{}, lifeSpan time.Duration, data interface{}) bool {
    table.Lock()
    if _, ok := table.items[key]; ok {
        table.Unlock()
        return false
    }
    //添加进去
    item := NewCacheItem(key, lifeSpan, data)
    table.addInternal(item)
    return true
}

//获取缓存，如果缓存不存在，则执行回调函数
func (table *CacheTable) Value(key interface{}, args ...interface{}) (*CacheItem, error) {
    table.RLock()
    r, ok := table.items[key]
    loadData := table.loadData
    table.RUnlock()
    if ok {
        // 更新最后访问时间和总访问数量
        r.KeepAlive()
        return r, nil
    }
    // 调用回调函数
    if loadData != nil {
        item := loadData(key, args...)
        if item != nil {
            table.Add(key, item.lifeSpan, item.data)
            return item, nil
        }
        return nil, ErrKeyNotFoundOrLoadable
    }
    return nil, ErrKeyNotFound
}

//清空缓存表
func (table *CacheTable) Flush() {
    table.Lock()
    defer table.Unlock()
    table.log("Flushing table", table.name)
    table.items = make(map[interface{}]*CacheItem)
    table.cleanupInterval = 0
    if table.cleanupTimer != nil {
        table.cleanupTimer.Stop()
    }
}

//提供访问最多的前几个缓存项，CacheItemPair有缓存的key和AccessCount组成
//CacheItemPairList则是CacheItemPair组成的Slice，且实现了Sort接口。
type CacheItemPair struct {
    Key         interface{}
    AccessCount int64
}

type CacheItemPairList []CacheItemPair

func (p CacheItemPairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p CacheItemPairList) Len() int           { return len(p) }
func (p CacheItemPairList) Less(i, j int) bool { return p[i].AccessCount > p[j].AccessCount }

//返回访问量最大的前 count 个缓存项
func (table *CacheTable) MostAccessed(count int64) []*CacheItem {
    table.RLock()
    defer table.RUnlock()
    //创建变量p为 CacheItemPairList类型 长度为整个缓存表table的缓存记录数
    p := make(CacheItemPairList, len(table.items))
    i := 0
    //初始化变量 p
    for k, v := range table.items {
        p[i] = CacheItemPair{k, v.accessCount}
        i++
    }
    sort.Sort(p)
    var r []*CacheItem
    c := int64(0)
    //取变量P的前 count 项
    for _, v := range p {
        if c >= count {
            break
        }
        item, ok := table.items[v.Key]
        if ok {
            r = append(r, item)
        }
        c++
    }
    return r
}

//记录缓存
func (table *CacheTable) log(v ...interface{}) {
    if table.logger == nil {
        return
    }
    table.logger.Println(v)
}
























