# UDP 协议可靠传输模拟库 (TCP-like over UDP)
本项目基于 Go 语言，从零实现了在 UDP 协议之上构建可靠数据传输（TCP-like）的全套演进机制。项目涵盖了从最基础的单包停等确认，到滑动窗口、延迟 ACK、选择性确认 (SACK) 以及快速重传与拥塞控制的渐进式演进。

## 📁 项目目录与演进
```bash
.
├── v1/    # 基础版本：停等机制 (Stop-and-Wait) 与逐包 ACK 确认
├── v2/    # 优化版本：延迟 ACK (Delayed ACK) 确认机制
├── v3/    # 进阶版本：滑动窗口 (Sliding Window) 与累计确认 (Cumulative ACK)
├── v4/    # 高级版本：SACK (选择性确认) 与精准区间重传 / 快速重传
└── v5/    # 完全体：拥塞控制 (Slow Start / Congestion Avoidance) 与动态 RTO
```

### 演进对比一览表

| 版本 | 核心特性 | ACK 响应机制 | 重传与异常处理 | 流量/拥塞控制 |
| :--- | :--- | :--- | :--- | :--- |
| **v1** | 停等机制 | 逐包立即确认 | 阻塞式超时重传 | 无 |
| **v2** | 延迟 ACK | 异步/定时批量确认 | 超时重传 | 无 |
| **v3** | 滑动窗口 | 累计确认 (Cumulative ACK) | 超时重传 | 发送方滑动窗口控制 |
| **v4** | 选择性重传 | SACK 区间确认 + Dup ACK | 快速重传 + 精准区间重传 | 接收方窗口管理 |
| **v5** | 拥塞控制 | 完整 TCP ACK 状态机 | 动态 RTO (RTT 估算) 重传 | 慢启动 + 拥塞避免 (`cwnd`) |
---

🛠️ 协议头二进制结构设计 (Header Design)
为了避免文本解析带来的性能损耗，底层传输采用自定义二进制编解码格式（大端序），头部在包的最前端，变长数据 Payload 永远放在最末尾。
1. 基础协议头 (v1 / v2 / v3)固定长度为 9 字节：
- Seq (4 Bytes, int32): 数据包序列号
- Ack (4 Bytes, int32): 确认号
- Flag (1 Byte, uint8): 标志位 (FlagTypeData, FlagTypeAck)
2. SACK 扩展协议头 (v4 / v5)
固定头部扩展至 11 字节，后续跟随变长 SAck 区间列表与负载 Data：
- Seq (4 Bytes, int32): 序列号
- Ack (4 Bytes, int32): 确认号
- Flag (1 Byte, uint8): 标志位 (FlagTypeData, FlagTypeAck, FlagTypeDupAck)
- SAckCount (1 Byte, uint8): 当前数据包携带的 SAck 区间数量（最多 255 个）
- Retransmit (1 Byte, bool): 重传标志位
- SAck Array ($N \times 8$ Bytes): $N$ 个非连续接收到的区间块 $[Left, Right]$
- Data: 变长数据负载 (Payload)
## 🚀 核心机制详解
v1：基础停等机制 (Stop-and-Wait)
- 客户端发送单个 UDP 数据包后阻塞等待 ACK 响应。
- 服务端收到数据后立即回复确认包。  特点：简单直观，但由于传输RTT等待，带宽利用率较低。
v2：延迟 ACK (Delayed ACK)
- 服务端接收到数据包后不立刻发送 ACK，而是由后台 Goroutine 积攒数据。
- 当超过延迟时间（如 200ms）或累计一定数量时统一发送批量 ACK。
- 特点：有效减少了网络中频繁发送的 ACK 微小数据包。
v3：滑动窗口与累计确认 (Sliding Window & Cumulative ACK)
- 引入发送方滑动窗口（Send Window），允许在未收到 ACK 的情况下连续发送多个数据包。
- 服务端采用累计确认机制（Cumulative ACK），仅回复连续按序接收到的最大 Seq。
v4：选择性确认与快速重传 (SACK & Fast Retransmit)
- SACK 块维护：服务端维护历史已接收区间并进行区间合并，将未连续接收的数据通过 SACK 块标记通知客户端。
- 精准区间重传：客户端结合 sentPackets 与收到的 SAck 集合进行区间差集计算，仅重传丢失的数据包而非全量重发。重复 ACK (Dup ACK)：模拟 TCP 乱序接收时的 Dup ACK 响应，触发客户端快速重传。
v5：拥塞控制与动态 RTO (Congestion Control & Dynamic RTO)
- 拥塞控制算法：实现慢启动（Slow Start）与拥塞避免（Congestion Avoidance）状态机，根据 ACK 状态动态调整拥塞窗口 cwnd 和慢启动阈值 ssthresh。
- 动态 RTO 计算：实时测量采样网络往返时间 RTT，采用 Jacobson 算法计算平滑 SRTT 与波动 RTTVAR，自适应调整超时重传时间 RTO。
## 💡 代码设计规范与最佳实践
在项目的代码重构与维护中，严格践行以下 Go 语言最佳实践：
1. 错误前置拦截 (Guard Clauses)：
在所有网络 Read/Write、数据包解码、空指针判定中，优先处理异常与边界条件并提前 return / continue，消除了深层代码嵌套。
```go
Go// 示例：错误前置拦截
if len(data) < HeaderSize {
    return Packet{}, errors.New("packet data too short for header")
}
```
2. 安全切片截断：
UDP 读取数据时，严格利用 ReadFromUDP 返回的实际字节数 n 截取 buffer[:n]，杜绝无效末尾 0 字节造成的序列化污染。
3. 超时与防死锁设计：
客户端与服务端连接均显式配置 SetReadDeadline 以及 select-timeout 机制，防止网络丢包时协程永久阻塞死锁。
## 📖 使用示例
每个版本包内均包含完整的客户端与服务端模拟测试，可以直接在入口中调用：
```go
package main

import (
	"v4" // 可切换为 v1, v2, v3, v4, v5
)

func main() {
	// 启动当前版本的 UDP 传输流程测试
	v4.Run()
}
```