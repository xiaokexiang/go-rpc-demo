## RPC
RPC（远程过程调用） 是一种计算机通信协议，它允许程序像调用本地函数一样调用远程服务，隐藏了底层网络细节

为什么需要 RPC：
简化开发： 像调用本地方法一样调用远程服务，降低网络编程复杂度。
高性能： 二进制协议（如 Protobuf）比 HTTP+JSON 更高效。
服务治理： 集成负载均衡、熔断等功能，适合分布式系统。

### 有了HTTP为什么还需要RPC?
> https://www.zhihu.com/question/524580708
- 纯裸 TCP 是能收发数据，但它是个无边界的数据流，上层需要定义消息格式用于定义消息边界。于是就有了各种协议，HTTP 和各类 RPC 协议就是在 TCP 之上定义的应用层协议。
- RPC 本质上不算是协议，而是一种调用方式，而像 gRPC 和 Thrift 这样的具体实现，才是协议，它们是实现了 RPC 调用的协议。目的是希望程序员能像调用本地方法那样去调用远端的服务方法。同时 RPC 有很多种实现方式，不一定非得基于 TCP 协议。
- 从发展历史来说，HTTP 主要用于 B/S 架构，而 RPC 更多用于 C/S 架构。但现在其实已经没分那么清了，B/S 和 C/S 在慢慢融合。很多软件同时支持多端，所以对外一般用 HTTP 协议，而内部集群的微服务之间则采用 RPC 协议进行通讯。RPC 其实比 HTTP 出现的要早，且比目前主流的 HTTP/1.1 性能要更好，所以大部分公司内部都还在使用 RPC。
- HTTP/2.0 在 HTTP/1.1 的基础上做了优化，性能可能比很多 RPC 协议都要好，但由于是这几年才出来的，所以也不太可能取代掉 RPC。

## client & server交互
```bash
Client                              Server
  │                                    │
  │ 1. Send Options (协商协议参数)        │
  │───────────────────────────────────▶│
  │                                    │
  │ 2. Ack Options (确认参数)            │
  │◀───────────────────────────────────│
  │                                    │
  │ 3. Write Request Header (H)        │
  │───────────────────────────────────▶│
  │                                    │
  │ 4. Write Request Body (B)          │
  │───────────────────────────────────▶│
  │                                    │
  │ 5. Read Response Header (H)        │
  │◀───────────────────────────────────│
  │                                    │
  │ 6. Read Response Body (B)          │
  │◀───────────────────────────────────│
  │                                    │
  ▼                                    ▼
```

## 流程图-Day1
![Day1](https://www.plantuml.com/plantuml/png/rLPHJzim47xFhpY5XwMbP3nCuwMhs1WmJMEJA1gFbUDS6s_fETWEh6Byzzbne2QXbBrCMgH6pjrt_kplyrazr8OfKo_BYDWITSKu0fSvShYDm3w23AgvYnurqJRidbquHvP_iZzK2GpzO02eb5GTq0UPhn9WjEgdBvKm-50-WX3LmEwQDo-Prd0gCx-CyHYIwUZzLSsMJ0d20KmcmTBsW4iY439rx0t5KIargW2HrNCOsfS5DO0mvoY62vcb7z694QQGsjGn-TJJUXGl42Iph-T4ATQgR8Os-qesAkOdsARNBanHNGkEPsWAcOCs5hHubBw2Mf1v-oO87ZlA5jjd_gmb_JkpVbYsJMwvtFzXbs-zzAGchzh527PEJWuEzlSRpU1sGy75gi8N5iJZkIHZwqgftcLbZPEvPHk_Tj59qZTXu1i_1gR1OPf2L-VAN438JaM33_nzTL48tRBVC3jIo0pce2MOWXcuGU2CW4kba9jo1GMxHh0HLWeBhwIm9BG0u_12WxkeKfWAuWZ2xOGC4wGXbqRx6TrqcMNv2G-Nccw5k8kYs6lMRclyUsyFsj3zzH8_WscFjGC5LwQsxOCARWEXlrL6IDhUUMTi8VZNUWc2J11kE9olWSNzypBrl5ixZ76EbKbAeGqI49hqIpDq_WOmluqZIPHiwCa-tFkTS0WaUuHITePplAfHcoug6IgV6Fc660Hb1QEyn2SBHw1WNQcHSdS-Xdf5F3sh3VuQmStzzoKJvbJ9kkgidEoQT2L9OtTksyV-kzlDJr8DeEzAkxJjjdIeGiFK0XJEI-Gc_SxA6gFr0OxYsIohHMa4pMQfltzZs4bJYDTdvU4C_qNJ1fFuIoYE3f5oz-_W3m00)

## 流程图-Day2
![Day2](https://www.plantuml.com/plantuml/png/rLRDZjis4BxxAGRAeR8hgpaK3L1iZMoSR8gYsmBhHNCeYWL5ZIssz51BKjuQRTwzf2ZRj2J4cqk1-22OypCVvv-lZMNQDbivUQCAc2_WMWgEy3rKAAMQl4Og7VCaHMoBfcPEX4k1PE6V2G2u0pC6banAkBg2T9LTFpPioxGWdC9YuQMwtzUmmaTe1DdllZqZwfq3laLRgHACTPgRY7sjDmOOxIFI7TPotwEzqrffGF-Dg6yL8Loj5LdWIYhzGo4RNF2AqTKtdhg0cR_vvAWoXzAff5C8f8-nYy7h2qxEWJke4dc-zD-8hceh5C7CE7zHvx-snuXla4hdEfWzyVosiVXtnAoGbQWxop-sXuYxRSQrp33USwW3pZ0iMrPtKVazAFqHWXEv9fNngeLICTbKZItL4mLwn60x9YGhf0zeJqVZHXYhznuXvUuJCZ65D61d8PVJCvMjsc6hS79i2lrBMjHRGSm6IMUbimHdfZYAdRzMBm7HLGQyQn_Zr8oKQmQigz8CzA4uCoYVLtr35ppQwhF0fbtfHxM_bCfW6Z5yEN-SJE96zkNK5QyWYthjgSkMwibHnx4oL5qAi_2dCzmJUgFAVUelD6H_qCBYKJs3M6hGREzL4QvnkTqK4GYLU_GMelKxxewYMSFCVPnGAn3pFCHieuhyBVfWn2oaZlME0ciy7jKp0bAupwU2KfsYCYuwoESmOi70fNXT6BVnY1uCro0xHFx9AFh_alMdGnlFLk1z8UaAa_oceH2r-mnUtDoyUlNoPHkpBcX3XVjzZSzKI0QJP-tGkicJ3fCm3OsV03FWPLf5h-VLF46edaOh7-Ntg_jAc0xzYRb_6fd54em2Mp4BJmYS4N2bDN8hzr2n7OAZC2cSUAq4MRzUh9SN1loWoc4bY2C8xmbP9iX0gQm95hsvmg7y1SUXPawJBeaMSG9N_yxdq94EuoNvXConh9DuP8RjVVRs2PlySkVTietWlpHRFseZSelXyngbU5Wlgu1xiSCOvrZRL0fZaO3GvZ-tV_5y0GOtbeee7U_9eNYNx_JiVTFRggoymyS6ZUrBfJCT62DxlM9KIXnLp433IVFmGfgT7ZiPF1tE7vYmxxLgwi7YlaDJApBOjrt1B9pxtJmADVOcxAvsCIrzg3XXUWCQrnAvpNzZii5uLISF3qKZf1Nqy927-Pl0Ypo7v5Ghnnh-ZAQDZ8GXb1pU-vF_0000)

```txt
Dial() -> newClient() -> options & codec() -> go receive()
⬇
sendSync() 
⬇
sendAsync() -> newCall()
⬇
send() -> registry call() -> write()
```
