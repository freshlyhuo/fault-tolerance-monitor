/*监听来自链路/共性服务的原始二进制输入以 goroutine + channel 模式推送原始帧
可能包含(需要确定)：TCP/UDP socket,ZeroMQ,串口,文件模拟输入
*/