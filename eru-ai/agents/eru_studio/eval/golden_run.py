#!/usr/bin/env python3
"""Run a prompt against the live Eru Studio agent and record what it produced.

This is the slow half of the eval harness. The fast half is `go test ./...` in
this package, which scores recorded pages with the same checks the agent is held
to at runtime. This script is what produces those recordings: it asks the real
agent a real question and writes every page that comes back into testdata, so
the next `go test` compares this run against the last one.

It exists because "is the page better than it was?" was, until now, answered by
opening it and looking. That answer does not survive a week, does not survive a
second opinion, and cannot be run in CI.

    ./golden_run.py --prompt-file ../../../local-testdata/prompts/financier_board.txt
    ./golden_run.py --prompt "make a page to display bank accounts ..." --name bank_accounts
    ./golden_run.py --list

Afterwards:
    go test ./agents/eru_studio/eval/ -v          # score what was recorded
    UPDATE_BASELINE=1 go test ./agents/eru_studio/eval/   # accept it as the new normal

The visual pass is deliberately not automated here. Save the page, open it in
processo's page viewer and look at it - the structural checks cannot tell you
whether a page is ugly, only whether it is wrong.
"""

import argparse
import json
import os
import sys
import urllib.request

DEFAULT_URL = os.environ.get("ERU_AI_MCP_URL", "http://localhost:8088/mcp")
# Everything recorded here describes a real tenant - its entities, its pages, the
# words its people use - so it is written to the module's ignored folder and
# never leaves this machine. The committed fixtures are the synthetic ones in
# each package's own testdata/.
LOCAL = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "..", "local-testdata")
FIXTURES = os.path.join(LOCAL, "studio_pages")
RUNS = os.path.join(LOCAL, "studio_runs")
PROMPTS = os.path.join(LOCAL, "prompts")


def headers_from_claude_config():
    """Reuse the headers the MCP client already uses, so this needs no secrets of its own."""
    path = os.path.expanduser("~/.claude.json")
    try:
        with open(path) as f:
            servers = json.load(f).get("mcpServers", {})
        for name, cfg in servers.items():
            if "eru" in name.lower() and cfg.get("headers"):
                return dict(cfg["headers"])
    except Exception:
        pass
    return {}


class Mcp:
    def __init__(self, url, headers):
        self.url = url
        self.base = dict(headers)
        self.base.setdefault("Content-Type", "application/json")
        self.base.setdefault("Accept", "application/json, text/event-stream")
        self.session = {}

    def post(self, body, notify=False, timeout=900):
        headers = dict(self.base)
        headers.update(self.session)
        req = urllib.request.Request(self.url, data=json.dumps(body).encode(), headers=headers)
        with urllib.request.urlopen(req, timeout=timeout) as r:
            sid = r.headers.get("Mcp-Session-Id") or r.headers.get("mcp-session-id")
            if sid:
                self.session["Mcp-Session-Id"] = sid
            raw = r.read().decode()
        if notify or not raw.strip():
            return None
        for line in raw.splitlines():
            if line.startswith("data:"):
                raw = line[5:].strip()
                break
        return json.loads(raw)

    def handshake(self):
        self.post({"jsonrpc": "2.0", "id": 0, "method": "initialize", "params": {
            "protocolVersion": "2024-11-05", "capabilities": {},
            "clientInfo": {"name": "eru-studio-golden-run", "version": "1"}}})
        self.post({"jsonrpc": "2.0", "method": "notifications/initialized"}, notify=True)

    def call(self, tool, arguments, timeout=900):
        return self.post({"jsonrpc": "2.0", "id": 1, "method": "tools/call",
                          "params": {"name": tool, "arguments": arguments}}, timeout=timeout)


def payload_of(response):
    for block in (response or {}).get("result", {}).get("content", []):
        if block.get("type") == "text":
            try:
                return json.loads(block["text"])
            except Exception:
                return block["text"]
    return response


def pages_from(payload):
    """Every page in the answer, root first, as (name, page_def).

    The agent answers with one action per page: the root envelope, then one per
    nested page. A bare page (output_mode "full") is the page itself.
    """
    out = []
    actions = []
    if isinstance(payload, dict):
        actions = payload.get("actions") or []
    for entry in actions:
        action = entry.get("action") if isinstance(entry, dict) else None
        if not isinstance(action, dict):
            continue
        page = action.get("page")
        if isinstance(page, dict) and page.get("components") is not None:
            out.append((page.get("id") or action.get("page_id") or "page", page))
        elif action.get("components") is not None:
            out.append((action.get("id") or "page", action))
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--prompt")
    ap.add_argument("--prompt-file")
    ap.add_argument("--name", help="fixture prefix; defaults to the page id the agent chose")
    ap.add_argument("--url", default=DEFAULT_URL)
    ap.add_argument("--output-mode", default="auto", choices=["auto", "patch", "full"])
    ap.add_argument("--timeout", type=int, default=900)
    ap.add_argument("--list", action="store_true", help="list recorded fixtures and exit")
    args = ap.parse_args()

    os.makedirs(FIXTURES, exist_ok=True)
    os.makedirs(RUNS, exist_ok=True)

    if args.list:
        for name in sorted(os.listdir(FIXTURES)):
            if name.endswith(".json"):
                print(name)
        return 0

    prompt = args.prompt
    if args.prompt_file:
        with open(args.prompt_file) as f:
            prompt = f.read()
    if not prompt:
        ap.error("one of --prompt or --prompt-file is required")

    mcp = Mcp(args.url, headers_from_claude_config())
    print(f"-> {args.url}", file=sys.stderr)
    mcp.handshake()

    response = mcp.call("agent__eru_studio", {
        "content": prompt,
        "params": {"output_mode": args.output_mode, "inline_nested_pages": "false"},
    }, timeout=args.timeout)

    if response and response.get("error"):
        print("agent error:", json.dumps(response["error"])[:2000], file=sys.stderr)
        return 1

    payload = payload_of(response)
    run_name = args.name or "run"
    with open(os.path.join(RUNS, run_name + ".json"), "w") as f:
        json.dump({"prompt": prompt, "output_mode": args.output_mode, "response": payload}, f, indent=2, sort_keys=True)

    pages = pages_from(payload)
    if not pages:
        print("no pages in the answer; the raw response was recorded under local-testdata/studio_runs", file=sys.stderr)
        return 1

    for index, (page_id, page) in enumerate(pages):
        stem = args.name or page_id
        if index > 0:
            stem = f"{stem}__{page_id}"
        path = os.path.join(FIXTURES, stem + ".json")
        with open(path, "w") as f:
            json.dump(page, f, indent=2, sort_keys=True)
        components = page.get("components") or []
        print(f"recorded {os.path.basename(path)}: {len(components)} top-level component(s)")

    print("\nnow score it:  go test ./agents/eru_studio/eval/ -v", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
