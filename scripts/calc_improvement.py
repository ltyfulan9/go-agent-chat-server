#!/usr/bin/env python3
import argparse

parser = argparse.ArgumentParser(description="Calculate improvement percentage for resume metrics.")
parser.add_argument("--before", type=float, required=True, help="baseline value, e.g. latency before optimization")
parser.add_argument("--after", type=float, required=True, help="optimized value, e.g. latency after optimization")
parser.add_argument("--lower-is-better", action="store_true", help="use for latency/error rate")
args = parser.parse_args()

if args.before == 0:
    raise SystemExit("before cannot be 0")

if args.lower_is_better:
    pct = (args.before - args.after) / args.before * 100
else:
    pct = (args.after - args.before) / args.before * 100

print(f"before={args.before}")
print(f"after={args.after}")
print(f"improvement={pct:.2f}%")
