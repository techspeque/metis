#!/usr/bin/env python3
"""Slice dispatcher — deterministic next/check/archive over the CAESAR ledger.

Usage:
    python3 harness/tools/dispatch.py next
    python3 harness/tools/dispatch.py check
    python3 harness/tools/dispatch.py archive

Schema & lifecycle: harness/slice-ledger.md
"""

import sys
import os
from pathlib import Path

try:
    import yaml
except ImportError:
    print("ERROR: pyyaml is required. Install with: pip3 install pyyaml", file=sys.stderr)
    sys.exit(1)

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
LEDGER_PATH = REPO_ROOT / "plans" / "slices.yaml"
ARCHIVE_PATH = REPO_ROOT / "plans" / "slices-done.yaml"

VALID_RISKS = {"low", "medium", "high"}

READING_RULES = {
    "low": "AGENTS.md §3,§4,§7,§9,§10 | ADRs cited by plan section | plan_section only",
    "medium": "AGENTS.md + §6,§11,§12 | ADRs touching packages in scope | plan_section + adjacent workstreams",
    "high": "AGENTS.md in full | every plausibly related ADR | the full phase section",
}


def load_yaml(path: Path) -> dict:
    if not path.exists():
        return {"version": 2, "agents": {}, "slices": []}
    with open(path, "r") as f:
        data = yaml.safe_load(f)
    if data is None:
        return {"version": 2, "agents": {}, "slices": []}
    return data


def save_yaml(path: Path, data: dict) -> None:
    with open(path, "w") as f:
        yaml.dump(data, f, default_flow_style=False, sort_keys=False, allow_unicode=True)


def cmd_next(ledger: dict) -> int:
    agents = ledger.get("agents", {})
    slices = ledger.get("slices", [])

    if not slices:
        print("No slices in the ledger. The backlog is empty.")
        return 0

    for s in slices:
        if not s.get("coded", False):
            role = "Coder"
            slug = s.get("coder", "")
        elif not s.get("reviewed", False):
            role = "Reviewer"
            slug = s.get("reviewer", "")
        else:
            continue

        # Found the active slice
        sid = s.get("id", "???")
        title = s.get("title", "")
        risk = s.get("risk", "unknown")
        stage = s.get("stage", "")
        plan = s.get("plan", "")
        plan_section = s.get("plan_section", "")
        review_cycles = s.get("review_cycles", 0)

        # Resolve agent info
        agent_info = agents.get(slug, {})
        plan_tag = agent_info.get("plan_tag", slug)

        if slug and slug not in agents:
            print(f"WARNING: slug '{slug}' not found in agents: map!", file=sys.stderr)

        print(f"Active slice: {sid}")
        print(f"  Title:          {title}")
        print(f"  Risk:           {risk}")
        print(f"  Stage:          {stage}" if stage else "")
        print(f"  Role:           {role}")
        print(f"  Required model: {slug} ({plan_tag})")
        print(f"  Plan:           {plan} {plan_section}" if plan else "  Plan:           (bespoke)")
        print(f"  Review cycles:  {review_cycles}")
        print(f"  Reading rule:   {READING_RULES.get(risk, 'unknown risk — read AGENTS.md in full')}")

        if role == "Reviewer":
            print(f"  Brief:          plans/briefs/{sid}.md")
            print(f"  Commits:        git log --oneline --grep \"{sid}\"")

        return 0

    print("All slices are done (coded && reviewed). Run 'archive' or add new slices.")
    return 0


def cmd_check(ledger: dict) -> int:
    agents = ledger.get("agents", {})
    slices = ledger.get("slices", [])
    errors = []

    # Check: unique IDs
    ids = [s.get("id") for s in slices]
    seen = set()
    for sid in ids:
        if sid in seen:
            errors.append(f"Duplicate slice ID: {sid}")
        seen.add(sid)

    for i, s in enumerate(slices):
        sid = s.get("id", f"<index {i}>")

        # Known agent slugs
        coder = s.get("coder", "")
        reviewer = s.get("reviewer", "")
        if coder and coder not in agents:
            errors.append(f"{sid}: coder '{coder}' not in agents: map")
        if reviewer and reviewer not in agents:
            errors.append(f"{sid}: reviewer '{reviewer}' not in agents: map")

        # Valid risk
        risk = s.get("risk", "")
        if risk not in VALID_RISKS:
            errors.append(f"{sid}: invalid risk '{risk}' (must be low/medium/high)")

        # plan_section required when plan is set
        if s.get("plan") and not s.get("plan_section"):
            errors.append(f"{sid}: plan is set but plan_section is missing")

        # reviewed && !coded is invalid
        if s.get("reviewed") and not s.get("coded"):
            errors.append(f"{sid}: reviewed=true but coded=false (invalid state)")

        # coder != reviewer
        if coder and reviewer and coder == reviewer:
            errors.append(f"{sid}: coder and reviewer are the same ({coder})")

    # Check archive if it exists
    if ARCHIVE_PATH.exists():
        archive = load_yaml(ARCHIVE_PATH)
        for s in archive.get("slices", []):
            sid = s.get("id", "???")
            if not (s.get("coded") and s.get("reviewed")):
                errors.append(f"ARCHIVE {sid}: not fully done (coded={s.get('coded')}, reviewed={s.get('reviewed')})")

    if errors:
        print("LEDGER CHECK FAILED:", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        return 1

    print(f"Ledger OK: {len(slices)} active slice(s), {len(agents)} agent(s) mapped.")
    return 0


def cmd_archive(ledger: dict) -> int:
    slices = ledger.get("slices", [])
    done = [s for s in slices if s.get("coded") and s.get("reviewed")]
    remaining = [s for s in slices if not (s.get("coded") and s.get("reviewed"))]

    if not done:
        print("Nothing to archive.")
        return 0

    # Load or create archive
    archive = load_yaml(ARCHIVE_PATH)
    if "slices" not in archive:
        archive["slices"] = []

    archive["slices"].extend(done)
    ledger["slices"] = remaining

    save_yaml(ARCHIVE_PATH, archive)
    save_yaml(LEDGER_PATH, ledger)

    archived_ids = [s.get("id") for s in done]
    print(f"Archived {len(done)} slice(s): {', '.join(archived_ids)}")
    return 0


def main() -> int:
    if len(sys.argv) < 2 or sys.argv[1] in ("-h", "--help"):
        print(__doc__)
        return 0

    cmd = sys.argv[1]

    if not LEDGER_PATH.exists():
        print(f"ERROR: Ledger not found at {LEDGER_PATH}", file=sys.stderr)
        return 1

    ledger = load_yaml(LEDGER_PATH)

    if cmd == "next":
        return cmd_next(ledger)
    elif cmd == "check":
        return cmd_check(ledger)
    elif cmd == "archive":
        return cmd_archive(ledger)
    else:
        print(f"Unknown command: {cmd}. Use next, check, or archive.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
