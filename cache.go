package cache2go

import (
    "sync"
)

var (
    cache := make(map[string]*CacheTable)
    mutex sync.RWMutex
)
/*
 * 我们第一步一般都是调用上面的Cache函数创建缓存，该函数会检查一个全局变量cache（该变量是一个map，其值类型为*CacheTable，key是缓存表的名字）
 * 如果该map中已经有名字为table的缓存表，就返回该缓存表；否则就创建。该函数返回缓存表的指针，即*CacheTable。
 * 也就是说在cache2go中，缓存表就代表一个缓存，而我们可以创建多个不同名字的缓存，存储在全局变量cache中
 *
 */
func Cache(table string) *CacheTable {
    mutex.RLock()
    t, ok := cache[table]
    mutex.RUnlock()
    if !ok {
        mutex.Lock()
        t, ok = cache[table]
        if !ok {
            t := &CacheTable {
                name: table,
                items: make(map[interface{}]*CacheItem),
            }
            cache[table] = t
        }
        mutex.Unlock()
    }
}