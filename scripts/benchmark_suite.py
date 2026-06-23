#!/usr/bin/env python3
"""
Run a one-command benchmark suite against the local server and generate a Markdown report.

No third-party Python packages are required. It uses only urllib + ThreadPoolExecutor so it
works on Windows / macOS / Linux as long as Python 3 is installed.
"""
from __future__ import annotations

import argparse
import concurrent.futures
import json
import math
import os
import random
import string
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, asdict
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple


def now_ms() -> float:
    return time.perf_counter() * 1000.0


def pct(values: List[float], percentile: float) -> float:
    if not values:
        return 0.0
    values = sorted(values)
    if len(values) == 1:
        return values[0]
    k = (len(values) - 1) * percentile / 100.0
    f = math.floor(k)
    c = math.ceil(k)
    if f == c:
        return values[int(k)]
    return values[f] * (c - k) + values[c] * (k - f)


@dataclass
class CaseResult:
    name: str   
    method: str  
    path: str   
    total: int   
    concurrency: int
    success: int
    failed: int
    elapsed_ms: float
    qps: float
    avg_ms: float
    p50_ms: float
    p95_ms: float
    p99_ms: float
    min_ms: float
    max_ms: float
    status_counts: Dict[str, int]


class Client:
    def __init__(self, base_url: str, timeout: float = 10.0):
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.token: Optional[str] = None

    def request(self, method: str, path: str, body: Optional[dict] = None, token: Optional[str] = None) -> Tuple[int, Dict[str, Any], str]:
        url = self.base_url + path
        data = None
        headers = {"Content-Type": "application/json"}
        bearer = token if token is not None else self.token
        if bearer:
            headers["Authorization"] = "Bearer " + bearer
        if body is not None:
            data = json.dumps(body).encode("utf-8")
        req = urllib.request.Request(url, data=data, method=method.upper(), headers=headers)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                text = resp.read().decode("utf-8", errors="replace")
                return resp.status, parse_json(text), text
        except urllib.error.HTTPError as e:
            text = e.read().decode("utf-8", errors="replace")
            return e.code, parse_json(text), text


def parse_json(text: str) -> Dict[str, Any]:
    try:
        obj = json.loads(text)
        return obj if isinstance(obj, dict) else {"_raw": obj}
    except Exception:
        return {"_text": text}


def get_data(obj: Dict[str, Any]) -> Any:
    if "data" in obj:
        return obj["data"]
    return obj


def random_username(prefix: str = "bench") -> str:
    suffix = "".join(random.choice(string.ascii_lowercase + string.digits) for _ in range(8))
    return f"{prefix}_{int(time.time())}_{suffix}"


def setup_user_and_session(client: Client) -> Tuple[str, str, str]:
    username = os.environ.get("BENCH_USERNAME") or random_username()
    password = os.environ.get("BENCH_PASSWORD") or "bench123456"

    status, obj, text = client.request("POST", "/api/auth/register", {"username": username, "password": password}, token="")
    if status >= 400:
        status, obj, text = client.request("POST", "/api/auth/login", {"username": username, "password": password}, token="")
    if status >= 400:
        raise RuntimeError(f"register/login failed: status={status}, body={text[:300]}")
    data = get_data(obj)
    token = data.get("token") if isinstance(data, dict) else None
    if not token:
        raise RuntimeError(f"token not found in auth response: {text[:300]}")
    client.token = token

    status, obj, text = client.request("POST", "/api/sessions", {"title": "benchmark session"})
    if status >= 400:
        raise RuntimeError(f"create session failed: status={status}, body={text[:300]}")
    data = get_data(obj)
    session_id = data.get("id") if isinstance(data, dict) else None
    if not session_id:
        raise RuntimeError(f"session id not found: {text[:300]}")
    return username, token, session_id


def seed_messages(client: Client, session_id: str, count: int) -> None:
    for i in range(count):
        role = "user" if i % 2 == 0 else "assistant"
        content = f"benchmark seed message {i:04d} " + ("x" * 80)
        status, _, text = client.request("POST", "/api/messages", {"session_id": session_id, "role": role, "content": content})
        if status >= 400:
            raise RuntimeError(f"seed message failed at {i}: status={status}, body={text[:300]}")


def read_metrics(debug_url: str) -> Dict[str, Any]:
    url = debug_url.rstrip("/") + "/debug/vars"
    try:
        with urllib.request.urlopen(url, timeout=3) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except Exception:
        return {}


def metric_delta(before: Dict[str, Any], after: Dict[str, Any], key: str) -> int:
    try:
        return int(after.get(key, 0)) - int(before.get(key, 0))
    except Exception:
        return 0


def run_case(client: Client, name: str, method: str, path: str, body: Optional[dict], total: int, concurrency: int, timeout: float) -> CaseResult:
    latencies: List[float] = []
    status_counts: Dict[str, int] = {}
    failed = 0

    def one(_: int) -> Tuple[int, float]:
        start = now_ms()
        status, _, _ = client.request(method, path, body=body)
        elapsed = now_ms() - start
        return status, elapsed

    start_all = now_ms()
    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as pool:
        futures = [pool.submit(one, i) for i in range(total)]
        for fut in concurrent.futures.as_completed(futures):
            try:
                status, elapsed = fut.result(timeout=timeout + 5)
                latencies.append(elapsed)
                key = str(status)
                status_counts[key] = status_counts.get(key, 0) + 1
                if status >= 400:
                    failed += 1
            except Exception:
                failed += 1
                status_counts["exception"] = status_counts.get("exception", 0) + 1
    elapsed_all = now_ms() - start_all
    success = total - failed
    qps = total / (elapsed_all / 1000.0) if elapsed_all > 0 else 0.0
    return CaseResult(
        name=name,
        method=method,
        path=path,
        total=total,
        concurrency=concurrency,
        success=success,
        failed=failed,
        elapsed_ms=round(elapsed_all, 2),
        qps=round(qps, 2),
        avg_ms=round(sum(latencies) / len(latencies), 2) if latencies else 0.0,
        p50_ms=round(pct(latencies, 50), 2),
        p95_ms=round(pct(latencies, 95), 2),
        p99_ms=round(pct(latencies, 99), 2),
        min_ms=round(min(latencies), 2) if latencies else 0.0,
        max_ms=round(max(latencies), 2) if latencies else 0.0,
        status_counts=status_counts,
    )


def improvement(before: float, after: float, lower_is_better: bool = True) -> float:
    if before == 0:
        return 0.0
    if lower_is_better:
        return (before - after) / before * 100.0
    return (after - before) / before * 100.0


def write_report(path: Path, args: argparse.Namespace, username: str, session_id: str, cases: List[CaseResult], metrics_before: Dict[str, Any], metrics_after: Dict[str, Any], cache_before: Optional[CaseResult], cache_after: Optional[CaseResult]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    hit = metric_delta(metrics_before, metrics_after, "message_cache_hit_total")
    miss = metric_delta(metrics_before, metrics_after, "message_cache_miss_total")
    cache_total = hit + miss
    hit_rate = hit / cache_total * 100.0 if cache_total else 0.0
    llm_calls = metric_delta(metrics_before, metrics_after, "llm_call_total")
    llm_failed = metric_delta(metrics_before, metrics_after, "llm_call_failed_total")
    async_created = metric_delta(metrics_before, metrics_after, "chat_job_created_total")
    async_success = metric_delta(metrics_before, metrics_after, "chat_job_success_total")
    async_failed = metric_delta(metrics_before, metrics_after, "chat_job_failed_total")
    queue_wait = metric_delta(metrics_before, metrics_after, "chat_job_queue_wait_ms_total")
    avg_queue_wait = queue_wait / async_success if async_success else 0.0

    cache_p95_improve = None
    if cache_before and cache_after and cache_before.p95_ms > 0:
        cache_p95_improve = improvement(cache_before.p95_ms, cache_after.p95_ms, True)

    md = []
    md.append("# Benchmark Result\n")
    md.append("> 这份报告由 `scripts/benchmark_suite.py` 自动生成。简历里的百分比只能引用本报告中的实测数据，不要手填。\n")
    md.append("## 测试配置\n")
    md.append(f"- Base URL: `{args.base_url}`")
    md.append(f"- Debug URL: `{args.debug_url}`")
    md.append(f"- 测试用户: `{username}`")
    md.append(f"- Session ID: `{session_id}`")
    md.append(f"- 种子消息数: `{args.seed_messages}`")
    md.append(f"- 普通接口请求数/并发: `N={args.n}`, `C={args.c}`")
    md.append(f"- mock 聊天请求数/并发: `N={args.chat_n}`, `C={args.chat_c}`")
    md.append("")

    md.append("## 压测结果\n")
    md.append("| Case | Method | Path | N | C | Success | Failed | QPS | Avg(ms) | p95(ms) | p99(ms) | Status |")
    md.append("|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---|")
    for r in cases:
        md.append(f"| {r.name} | {r.method} | `{r.path}` | {r.total} | {r.concurrency} | {r.success} | {r.failed} | {r.qps:.2f} | {r.avg_ms:.2f} | {r.p95_ms:.2f} | {r.p99_ms:.2f} | `{json.dumps(r.status_counts, ensure_ascii=False)}` |")
    md.append("")

    md.append("## 缓存与链路指标\n")
    md.append(f"- 消息缓存命中增量: `{hit}`")
    md.append(f"- 消息缓存未命中增量: `{miss}`")
    md.append(f"- 本轮消息分页缓存命中率: `{hit_rate:.2f}%`")
    if cache_p95_improve is not None:
        md.append(f"- 消息分页 p95 延迟优化: `{cache_before.p95_ms:.2f} ms -> {cache_after.p95_ms:.2f} ms`，降低约 `{cache_p95_improve:.2f}%`")
    md.append(f"- LLM 调用增量: `{llm_calls}`，失败增量: `{llm_failed}`")
    md.append(f"- 异步任务创建增量: `{async_created}`，成功增量: `{async_success}`，失败增量: `{async_failed}`")
    if async_success:
        md.append(f"- 异步任务平均队列等待时间: `{avg_queue_wait:.2f} ms/job`")
    md.append("")

    md.append("## 可直接替换到简历的写法\n")
    msg_case = next((r for r in cases if r.name == "messages_page_cached"), None) or next((r for r in cases if r.name.startswith("messages")), None)
    sess_case = next((r for r in cases if r.name == "sessions_page"), None)
    chat_case = next((r for r in cases if r.name == "mock_chat"), None)
    async_case = next((r for r in cases if r.name == "async_chat_submit"), None)

    if sess_case:
        md.append(f"- 使用 Python 并发压测脚本对会话分页接口进行验证，在 `N={sess_case.total}, C={sess_case.concurrency}` 下 QPS `{sess_case.qps:.2f}`、p95 `{sess_case.p95_ms:.2f}ms`、错误数 `{sess_case.failed}`。")
    if msg_case:
        suffix = f"；缓存命中率 `{hit_rate:.2f}%`" if cache_total else ""
        md.append(f"- 对消息分页接口进行压测，在 `N={msg_case.total}, C={msg_case.concurrency}` 下 QPS `{msg_case.qps:.2f}`、p95 `{msg_case.p95_ms:.2f}ms`{suffix}。")
    if cache_p95_improve is not None:
        md.append(f"- 基于 Redis 对消息分页结果进行缓存，重复查询场景下 p95 延迟由 `{cache_before.p95_ms:.2f}ms` 降至 `{cache_after.p95_ms:.2f}ms`，降低约 `{cache_p95_improve:.2f}%`。")
    if chat_case:
        md.append(f"- 使用 mock LLM 隔离模型耗时后压测聊天主链路，在 `N={chat_case.total}, C={chat_case.concurrency}` 下 QPS `{chat_case.qps:.2f}`、p95 `{chat_case.p95_ms:.2f}ms`，用于验证后端链路和并发控制开销。")
    if async_case:
        md.append(f"- 新增异步聊天提交接口，API 层创建 job 并投递 RabbitMQ，在 `N={async_case.total}, C={async_case.concurrency}` 下提交 QPS `{async_case.qps:.2f}`、p95 `{async_case.p95_ms:.2f}ms`；Worker 侧通过任务状态和队列等待指标观察处理能力。")
    md.append("")

    md.append("## 注意\n")
    md.append("- `mock_chat` 只用于验证后端链路，不代表真实 Ollama/Coze 生成吞吐。")
    md.append("- LLM 真实吞吐要单独说明：瓶颈在模型服务/GPU/API 限额，不能和普通 HTTP QPS 混在一起吹。")
    md.append("- 如果某项 failed 不为 0，先不要写进简历，先修接口或降低并发。")
    md.append("")

    path.write_text("\n".join(md), encoding="utf-8")


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--base-url", default=os.environ.get("BASE_URL", "http://127.0.0.1:8080"))
    ap.add_argument("--debug-url", default=os.environ.get("DEBUG_URL", "http://127.0.0.1:6060"))
    ap.add_argument("--seed-messages", type=int, default=int(os.environ.get("SEED_MESSAGES", "120")))
    ap.add_argument("--n", type=int, default=int(os.environ.get("N", "300")))
    ap.add_argument("--c", type=int, default=int(os.environ.get("C", "30")))
    ap.add_argument("--chat-n", type=int, default=int(os.environ.get("CHAT_N", "50")))
    ap.add_argument("--chat-c", type=int, default=int(os.environ.get("CHAT_C", "5")))
    ap.add_argument("--include-async", action="store_true", help="also benchmark /api/chat/async; requires RabbitMQ publisher configured")
    ap.add_argument("--output", default="docs/BENCHMARK_RESULT.md")
    args = ap.parse_args()

    client = Client(args.base_url, timeout=15.0)
    status, _, text = client.request("GET", "/api/health", token="")
    if status >= 400:
        raise SystemExit(f"server is not healthy: status={status}, body={text[:300]}")

    username, token, session_id = setup_user_and_session(client)
    print(f"created/login user={username}, session_id={session_id}")
    if args.seed_messages > 0:
        print(f"seeding {args.seed_messages} messages...")
        seed_messages(client, session_id, args.seed_messages)

    metrics_before = read_metrics(args.debug_url)
    cases: List[CaseResult] = []

    # First messages-page run warms the Redis page cache. Second run measures cache-hit scenario.
    messages_path = f"/api/sessions/{session_id}/messages?page=1&page_size=30"
    print("running messages_page_cold...")
    cache_before_case = run_case(client, "messages_page_cold", "GET", messages_path, None, max(20, args.n // 5), args.c, 15)
    cases.append(cache_before_case)

    print("running messages_page_cached...")
    cache_after_case = run_case(client, "messages_page_cached", "GET", messages_path, None, args.n, args.c, 15)
    cases.append(cache_after_case)

    print("running sessions_page...")
    cases.append(run_case(client, "sessions_page", "GET", "/api/sessions?page=1&page_size=20", None, args.n, args.c, 15))

    print("running mock_chat...")
    cases.append(run_case(client, "mock_chat", "POST", "/api/chat", {"session_id": session_id, "model": "mock", "message": "benchmark mock llm"}, args.chat_n, args.chat_c, 30))

    if args.include_async:
        print("running async_chat_submit...")
        cases.append(run_case(client, "async_chat_submit", "POST", "/api/chat/async", {"session_id": session_id, "model": "mock", "message": "benchmark async llm"}, args.chat_n, args.chat_c, 30))

    # Let async worker and background event publishers update metrics.
    time.sleep(1)
    metrics_after = read_metrics(args.debug_url)

    out = Path(args.output)
    write_report(out, args, username, session_id, cases, metrics_before, metrics_after, cache_before_case, cache_after_case)
    print(f"report written: {out}")
    print("\nSummary:")
    for r in cases:
        print(f"- {r.name}: QPS={r.qps:.2f}, p95={r.p95_ms:.2f}ms, failed={r.failed}, status={r.status_counts}")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
#先检查Go服务有没有启动，自动注册/登录一个测试用户，自动创建一个测试session，往这个session里插入一批测试消息
#开始并发压测接口，读取/Debug/vars的监控指标，生成压测报告