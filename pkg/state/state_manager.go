/* 1）实时状态维护

UpdateMetric(metric Metric)

更新：

最新节点状态

最新容器状态

最新服务状态

最新业务层状态

2）统一查询接口

GetLatestState(id string)

返回某服务/容器/业务字段的最新状态。

3）历史窗口缓存

允许告警生成器查询趋势：

AppendHistory(metric Metric)
QueryHistory(id, duration)

4）时间戳对齐

AlignTimestamp(metric)

用于多源数据对齐。 */