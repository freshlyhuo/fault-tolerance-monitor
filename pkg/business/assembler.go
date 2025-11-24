/*对 raw bytes 进行“组帧”,校验长度字段,校验 CRC 或数字签名

输入：raw []byte
输出：frame []byte

错误情况应转化为业务异常指标（BusinessError）。
*/