# Benchmark Result

> 这份报告由 `scripts/benchmark_suite.py` 自动生成。

## 测试配置

- Base URL: `http://127.0.0.1:8080`
- Debug URL: `http://127.0.0.1:6060`
- 测试用户: `bench_1781529763_z91raafx`
- Session ID: `324901542614274048`
- 种子消息数: `120`
- 普通接口请求数/并发: `N=300`, `C=30`
- mock 聊天请求数/并发: `N=40`, `C=4`

## 压测结果

| Case | Method | Path | N | C | Success | Failed | QPS | Avg(ms) | p95(ms) | p99(ms) | Status |
|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| messages_page_cold | GET | `/api/sessions/324901542614274048/messages?page=1&page_size=30` | 60 | 30 | 60 | 0 | 560.70 | 26.77 | 71.22 | 73.08 | `{"200": 60}` |
| messages_page_cached | GET | `/api/sessions/324901542614274048/messages?page=1&page_size=30` | 300 | 30 | 300 | 0 | 1572.95 | 16.10 | 24.83 | 28.39 | `{"200": 300}` |
| sessions_page | GET | `/api/sessions?page=1&page_size=20` | 300 | 30 | 300 | 0 | 1731.84 | 14.22 | 23.53 | 26.46 | `{"200": 300}` |
| mock_chat | POST | `/api/chat` | 40 | 4 | 40 | 0 | 26.87 | 146.86 | 160.57 | 163.65 | `{"200": 40}` |

## 缓存与链路指标

- 消息缓存命中增量: `360`
- 消息缓存未命中增量: `40`
- 本轮消息分页缓存命中率: `90.00%`
- 消息分页 p95 延迟优化: `71.22 ms -> 24.83 ms`，降低约 `65.14%`
- LLM 调用增量: `40`，失败增量: `0`
- 异步任务创建增量: `0`，成功增量: `0`，失败增量: `0`
