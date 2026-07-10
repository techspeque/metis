#!/usr/bin/env python3
"""Caesar slice progress dashboard.

Parses plans/slices.yaml and plans/slices-done.yaml and renders a
terminal progress report with per-phase bars and overall completion.
"""

import re
import sys
from pathlib import Path
from collections import OrderedDict

try:
    import yaml
except ImportError:
    print("ERROR: pyyaml not installed (pip3 install pyyaml)", file=sys.stderr)
    sys.exit(1)

# ─── Config ───────────────────────────────────────────────────────────────────

ROOT = Path(__file__).resolve().parent.parent.parent
SLICES_FILE = ROOT / "plans" / "slices.yaml"
DONE_FILE = ROOT / "plans" / "slices-done.yaml"

BAR_WIDTH = 20

# ANSI colors
GREEN = "\033[32m"
YELLOW = "\033[33m"
RED = "\033[31m"
CYAN = "\033[36m"
BOLD = "\033[1m"
DIM = "\033[2m"
RESET = "\033[0m"


# ─── Helpers ──────────────────────────────────────────────────────────────────

def discover_phase_titles():
    """Extract phase titles from the implementation plan markdown headings.

    Looks for lines like '## Phase 3 - Planner' and builds an ordered
    mapping of phase-key -> title.  Falls back to generic 'Phase N' if
    the plan file is missing or unparseable.
    """
    titles = OrderedDict()
    plan_refs = set()

    # Gather plan file paths referenced by slices
    for path in (SLICES_FILE, DONE_FILE):
        if not path.exists():
            continue
        with open(path) as f:
            data = yaml.safe_load(f)
        for s in data.get("slices", []) or []:
            plan = s.get("plan")
            if plan:
                plan_refs.add(plan)

    # Parse headings from each referenced plan
    heading_re = re.compile(r"^##\s+Phase\s+(\d+)\s*[-–—]\s*(.+)$")
    for plan_rel in plan_refs:
        plan_path = ROOT / plan_rel
        if not plan_path.exists():
            continue
        with open(plan_path) as f:
            for line in f:
                m = heading_re.match(line.strip())
                if m:
                    num, title = m.group(1), m.group(2).strip()
                    titles[f"phase-{num}"] = title

    return titles

def load_slices(path):
    """Load slices from a YAML file, returning a list of dicts."""
    if not path.exists():
        return []
    with open(path) as f:
        data = yaml.safe_load(f)
    return data.get("slices", []) or []


def phase_key(slice_id):
    """Extract phase key from a slice ID like 'phase-2-ws-2.3' or 'docs-recon-3'."""
    if slice_id.startswith("docs-recon-"):
        # docs-recon slices belong to their numbered phase
        num = slice_id.split("-")[-1]
        return f"phase-{num}"
    parts = slice_id.split("-")
    if len(parts) >= 2 and parts[0] == "phase":
        return f"phase-{parts[1]}"
    return "other"


def progress_bar(done, total, width=BAR_WIDTH, in_progress=0):
    """Render a tri-state progress bar string.

    Segments: done (green) | in_progress (cyan) | remaining (dim).
    """
    if total == 0:
        return " " * width

    done_cells = int((done / total) * width)
    wip_cells = int(((done + in_progress) / total) * width) - done_cells
    empty_cells = width - done_cells - wip_cells

    parts = []
    if done_cells:
        color = GREEN if (done + in_progress) >= total else YELLOW
        parts.append(f"{color}{'█' * done_cells}{RESET}")
    if wip_cells:
        parts.append(f"{CYAN}{'▓' * wip_cells}{RESET}")
    if empty_cells:
        parts.append(f"{DIM}{'░' * empty_cells}{RESET}")

    return "".join(parts)


def format_pct(done, total):
    """Format percentage with right-alignment."""
    if total == 0:
        return "  -"
    pct = int((done / total) * 100)
    return f"{pct:3d}%"


def stage_label(stage, index=0):
    """Colorize stage labels based on position in the stage ordering."""
    palette = [GREEN, YELLOW, CYAN, RED]
    color = palette[index % len(palette)]
    return f"{color}{stage}{RESET}"


# ─── Main ─────────────────────────────────────────────────────────────────────

def main():
    done_slices = load_slices(DONE_FILE)
    remaining_slices = load_slices(SLICES_FILE)

    # Partition remaining slices into "in progress" (coded but not reviewed)
    # and truly pending (not yet coded).
    wip_slices = [s for s in remaining_slices if s.get("coded")]
    pending_slices = [s for s in remaining_slices if not s.get("coded")]

    total_done = len(done_slices)
    total_wip = len(wip_slices)
    total_pending = len(pending_slices)
    total_all = total_done + total_wip + total_pending

    # Discover phase titles from the plan file
    phase_titles = discover_phase_titles()

    # Group by phase: done / wip / total
    phase_done = {}
    phase_wip = {}
    phase_total = {}
    for s in done_slices:
        pk = phase_key(s["id"])
        phase_done[pk] = phase_done.get(pk, 0) + 1
        phase_total[pk] = phase_total.get(pk, 0) + 1
    for s in wip_slices:
        pk = phase_key(s["id"])
        phase_wip[pk] = phase_wip.get(pk, 0) + 1
        phase_total[pk] = phase_total.get(pk, 0) + 1
    for s in pending_slices:
        pk = phase_key(s["id"])
        phase_total[pk] = phase_total.get(pk, 0) + 1

    # Build ordered phase list (sorted numerically)
    all_phases = sorted(phase_total.keys(), key=lambda k: int(k.split("-")[-1]) if k.split("-")[-1].isdigit() else 999)

    # Group by stage (preserving first-seen order from the YAML)
    stage_order = []
    stage_done = {}
    stage_wip = {}
    stage_total = {}
    for s in done_slices:
        st = s.get("stage", "unknown")
        if st not in stage_order:
            stage_order.append(st)
        stage_done[st] = stage_done.get(st, 0) + 1
        stage_total[st] = stage_total.get(st, 0) + 1
    for s in wip_slices:
        st = s.get("stage", "unknown")
        if st not in stage_order:
            stage_order.append(st)
        stage_wip[st] = stage_wip.get(st, 0) + 1
        stage_total[st] = stage_total.get(st, 0) + 1
    for s in pending_slices:
        st = s.get("stage", "unknown")
        if st not in stage_order:
            stage_order.append(st)
        stage_total[st] = stage_total.get(st, 0) + 1

    # Review cycles stats
    review_cycles = [s.get("review_cycles", 0) for s in done_slices]
    multi_review = sum(1 for c in review_cycles if c > 0)

    # ─── Render ───────────────────────────────────────────────────────────────

    print()
    project_name = ROOT.name.upper()
    print(f"  {BOLD}{project_name} — Slice Progress{RESET}")
    print(f"  {'─' * 58}")
    print()

    # Legend
    print(f"  {GREEN}█{RESET} done  {CYAN}▓{RESET} coded (awaiting review)  {DIM}░{RESET} pending")
    print()

    # Per-phase bars
    for pk in all_phases:
        done = phase_done.get(pk, 0)
        wip = phase_wip.get(pk, 0)
        total = phase_total.get(pk, 0)
        if total == 0:
            continue
        title = phase_titles.get(pk, pk.replace("-", " ").title())
        bar = progress_bar(done, total, in_progress=wip)
        pct = format_pct(done + wip, total)
        if wip:
            status = f"({done}+{wip}/{total})"
        else:
            status = f"({done}/{total})"
        phase_num = pk.replace("phase-", "Ph ")
        print(f"  {phase_num}  {bar}  {pct}  {status:>10}  {DIM}{title}{RESET}")

    print()
    print(f"  {'─' * 58}")

    # Overall bar
    overall_bar = progress_bar(total_done, total_all, width=BAR_WIDTH, in_progress=total_wip)
    overall_pct = format_pct(total_done + total_wip, total_all)
    if total_wip:
        overall_status = f"({total_done}+{total_wip}/{total_all})"
    else:
        overall_status = f"({total_done}/{total_all})"
    print(f"  ALL  {overall_bar}  {overall_pct}  {overall_status}")
    print()

    # Stage summary
    print(f"  {BOLD}By stage:{RESET}")
    for i, st in enumerate(stage_order):
        done = stage_done.get(st, 0)
        wip = stage_wip.get(st, 0)
        total = stage_total.get(st, 0)
        if total == 0:
            continue
        pct = format_pct(done + wip, total)
        label = stage_label(st, i)
        if wip:
            print(f"    {label:<22} {done}+{wip}/{total} {pct}")
        else:
            print(f"    {label:<22} {done}/{total} {pct}")
    print()

    # Quality stats
    print(f"  {BOLD}Quality:{RESET}")
    print(f"    First-pass acceptance:  {total_done - multi_review}/{total_done} ({format_pct(total_done - multi_review, total_done).strip()})")
    print(f"    Required re-review:     {multi_review}/{total_done}")
    print()

    # In-progress slices (coded, awaiting review)
    if wip_slices:
        print(f"  {BOLD}In progress (coded, awaiting review):{RESET}")
        for s in wip_slices:
            rid = s["id"]
            title = s.get("title", "")
            reviewer = s.get("reviewer", "?")
            print(f"    {CYAN}{rid}{RESET} — {title}  {DIM}(reviewer: {reviewer}){RESET}")
        print()

    # Next up (first pending slice that is not yet coded)
    if pending_slices:
        nxt = pending_slices[0]
        print(f"  {BOLD}Next up:{RESET} {nxt['id']} — {nxt['title']}")
        print(f"          coder: {nxt.get('coder', '?')}  |  risk: {nxt.get('risk', '?')}")
        print()
    elif wip_slices and not pending_slices:
        print(f"  {BOLD}Next up:{RESET} All remaining slices are coded — awaiting review only.")
        print()


if __name__ == "__main__":
    main()
