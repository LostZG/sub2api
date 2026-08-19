#!/usr/bin/env python3
"""校验 govulncheck JSON 输出中的可达漏洞是否全部被例外清单覆盖。

与 tools/check_pnpm_audit_exceptions.py（前端 pnpm audit 豁免）同构：
- 输入：govulncheck -format json 的输出文件 + 例外清单（轻量 YAML）
- 规则：每个 symbol 级可达漏洞 ID 必须在例外清单中且未过期，否则失败
- 目的：lib/pq 等无官方修复版本的依赖（Fixed in N/A）需要可控豁免，
  同时保留对未来新漏洞的监控能力。
"""
import argparse
import json
import sys
from datetime import date


REQUIRED_FIELDS = {"id", "expires_on"}


def split_kv(line: str) -> tuple[str, str]:
    # 解析 "key: value" 形式的简单 YAML 行，并去除引号。
    key, value = line.split(":", 1)
    value = value.strip()
    if (value.startswith('"') and value.endswith('"')) or (
        value.startswith("'") and value.endswith("'")
    ):
        value = value[1:-1]
    return key.strip(), value


def parse_exceptions(path: str) -> list[dict]:
    # 轻量解析异常清单，避免引入额外依赖。
    exceptions = []
    current = None
    with open(path, "r", encoding="utf-8") as handle:
        for raw in handle:
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("version:") or line.startswith("exceptions:"):
                continue
            if line.startswith("- "):
                if current:
                    exceptions.append(current)
                current = {}
                line = line[2:].strip()
                if line:
                    key, value = split_kv(line)
                    current[key] = value
                continue
            if current is not None and ":" in line:
                key, value = split_kv(line)
                current[key] = value
    if current:
        exceptions.append(current)
    return exceptions


def iter_json_objects(text: str):
    # govulncheck -format json 输出为逐个 pretty-print 的流式 JSON 对象，
    # 对象之间无分隔符，需要 raw_decode 逐个解析。
    decoder = json.JSONDecoder()
    idx = 0
    while idx < len(text):
        while idx < len(text) and text[idx] in " \n\t\r":
            idx += 1
        if idx >= len(text):
            break
        obj, end = decoder.raw_decode(text, idx)
        yield obj
        idx = end


def parse_findings(scan_path: str) -> tuple[set[str], dict[str, str]]:
    # symbol 级 finding（trace 含 function）视为可达漏洞；
    # 模块级 finding（trace 仅 module 位置）不计，与 govulncheck
    # 文本模式 "Your code is affected" 的口径一致。
    summaries: dict[str, str] = {}
    findings: dict[str, bool] = {}
    for entry in iter_json_objects(open(scan_path, "r", encoding="utf-8").read()):
        if not isinstance(entry, dict):
            continue
        osv = entry.get("osv")
        if isinstance(osv, dict) and osv.get("id"):
            summaries.setdefault(osv["id"], osv.get("summary") or "")
        finding = entry.get("finding")
        if not isinstance(finding, dict) or not finding.get("osv"):
            continue
        trace = finding.get("trace") or []
        reachable = any(step.get("function") for step in trace if isinstance(step, dict))
        findings[finding["osv"]] = findings.get(finding["osv"], False) or reachable
    return {vid for vid, ok in findings.items() if ok}, summaries


def parse_date(value: str) -> date | None:
    try:
        return date.fromisoformat(value)
    except ValueError:
        return None


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--scan", required=True)
    parser.add_argument("--exceptions", required=True)
    args = parser.parse_args()

    errors = []
    exception_index = {}
    for exc in parse_exceptions(args.exceptions):
        missing = [field for field in REQUIRED_FIELDS if not exc.get(field)]
        if missing:
            errors.append(
                f"Exception missing required fields {missing}: {exc.get('id', '<unknown>')}"
            )
            continue
        exc_date = parse_date(exc.get("expires_on"))
        if exc_date is None:
            errors.append(f"Exception has invalid expires_on date: {exc.get('id')}")
            continue
        if exc["id"] in exception_index:
            errors.append(f"Duplicate exception for {exc['id']}")
            continue
        exception_index[exc["id"]] = {
            "expires_on": exc_date,
            "reason": exc.get("reason", ""),
        }

    today = date.today()
    reachable, summaries = parse_findings(args.scan)

    missing = []
    expired = []
    for vuln_id in sorted(reachable):
        exc = exception_index.get(vuln_id)
        if exc is None:
            missing.append(vuln_id)
        elif exc["expires_on"] < today:
            expired.append((vuln_id, exc["expires_on"].isoformat()))

    if missing:
        errors.append("Reachable vulnerabilities missing exceptions:")
        for vuln_id in missing:
            summary = summaries.get(vuln_id, "")
            label = f"- {vuln_id}"
            if summary:
                label = f"{label}: {summary}"
            errors.append(label)

    if expired:
        errors.append("Exceptions expired (re-evaluate or renew):")
        for vuln_id, expires_on in expired:
            errors.append(f"- {vuln_id} expired on {expires_on}")

    if errors:
        sys.stderr.write("\n".join(errors) + "\n")
        return 1

    print(f"govulncheck exceptions validated ({len(reachable)} finding(s) covered).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
