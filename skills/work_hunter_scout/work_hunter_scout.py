#!/usr/bin/env python3
"""Work Hunter Scout V0.2 — discovery multi-fuente con evidencia de dinero,
competencia real (PRs) y dataset persistente.

Fuentes:
  - bounties: issues con label bounty (filtra granjas/spam/token)
  - orphan:   issues viejas "help wanted" en repos activos (trabajo huérfano)

Reglas V0.2 (aprendidas del experimento V0.1):
  - Comentarios en bounty issues = claims = competencia, NO demanda.
  - label:bounty no implica dinero: se requiere money_evidence >= 1.
  - Repo con muchas issues templadas = BOUNTY_FARM.
  - Veredicto economico: expected_hourly = reward x P(accept) x automation / human_hours
    (la skill calcula reward y P(accept); Judge estima automation y human_hours).

Salida: JSON con stats + candidatos rankeados. Cada observacion se agrega al
dataset JSONL (una linea por oportunidad, con categoria).
"""
import json
import math
import os
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone

INVOK_URL = os.environ.get("INVOK_URL", "http://localhost:8080").rstrip("/")
# Fallback directo a api.github.com SOLO si Invok está caído y se pide
# explicitamente. Default: fallar ruidoso (sin Invok no hay credenciales).
ALLOW_DIRECT = os.environ.get("WORK_HUNTER_ALLOW_DIRECT", "0") == "1"
GH_TOKEN = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
QUERIES = {
    "bounties": [
        'label:bounty is:issue is:open sort:created-desc',
        'label:"💰 bounty" is:issue is:open',
    ],
    "orphan": [
        'is:issue is:open label:"help wanted" created:2026-01-01..2026-07-15 sort:reactions',
        'is:issue is:open label:"good first issue" created:2026-02-01..2026-07-01 sort:reactions',
    ],
}
JUNK_TITLE = ["[radar]", "open bounty 202", "airdrop", "giveaway", "checkpoint reward"]
JUNK_REPO = ["test-repo", "testrepo"]
BAIT_WORDS = ["planted", "planted auth", "testnet bounty", "victory audit", "flag capture"]
STACK_KW = [("test", 8), ("coverage", 8), ("ci", 6), ("docker", 7), ("github action", 7),
            ("build", 5), ("typescript", 5), ("react", 4), ("go", 3), ("java", 3),
            ("python", 4), ("api", 4), ("docs", 4), ("dependency", 5)]
MONEY_RE = re.compile(
    r"[$€]\s?([0-9][0-9,]{1,6})|([0-9][0-9,]{1,6})\s?(USD|USDC|USDT|EUR)\b|(?:0\.\d+|[1-9]\d*)\s?(ETH|SOL)\b",
    re.IGNORECASE)
ESCROW_RE = re.compile(r"escrow|paid on merge|paid on approval|first mergeable", re.IGNORECASE)
PLATFORM_RE = re.compile(r"algora\.dev|polar\.sh|gitcoin", re.IGNORECASE)


def log(m):
    print(m, file=sys.stderr, flush=True)


def _parse_mcp_body(body):
    """Extrae el texto de una respuesta JSON-RPC (acepta body SSE o JSON puro)
    y parsea el contenido de la tool (Invok envuelve respuestas externas en
    <UntrustedExternalContent>: se limpia antes de json.loads)."""
    raw = body
    if "data:" in body and body.lstrip().startswith(("event:", "data:")):
        lines = [l[5:].strip() for l in body.splitlines() if l.startswith("data:")]
        raw = lines[-1] if lines else body
    resp = json.loads(raw)
    if "error" in resp:
        log(f"invok rpc error: {str(resp['error'])[:200]}")
        return None
    result = resp.get("result") or {}
    content = (result.get("content") or [{}])[0].get("text", "")
    cleaned = re.sub(r"</?UntrustedExternalContent>", "", content).strip()
    if not cleaned:
        return None
    try:
        return json.loads(cleaned)
    except json.JSONDecodeError:
        return cleaned


_RPC_ID = [0]


def invok_call(tool, args, retries=2):
    """Ejecuta una tool de Invok via MCP (JSON-RPC sobre HTTP). Las credenciales
    de GitHub viven cifradas en Invok: esta skill jamás las ve ni las maneja."""
    _RPC_ID[0] += 1
    payload = json.dumps({
        "jsonrpc": "2.0", "id": _RPC_ID[0], "method": "tools/call",
        "params": {"name": tool, "arguments": args},
    }).encode()
    req = urllib.request.Request(f"{INVOK_URL}/mcp", data=payload, method="POST")
    req.add_header("Content-Type", "application/json")
    req.add_header("Accept", "application/json, text/event-stream")
    for attempt in range(retries + 1):
        try:
            with urllib.request.urlopen(req, timeout=40) as r:
                return _parse_mcp_body(r.read().decode())
        except urllib.error.HTTPError as e:
            if attempt < retries and e.code in (500, 502, 503, 504):
                time.sleep(2 * (attempt + 1))
                continue
            log(f"invok {tool}: HTTP {e.code}")
            return None
        except Exception as e:
            if attempt < retries:
                time.sleep(2 * (attempt + 1))
                continue
            log(f"invok {tool}: {e}")
            return None
    return None


def gh(path, retries=1):
    """Fallback directo (sin credenciales salvo GH_TOKEN en env). Solo se usa
    si ALLOW_DIRECT=1 e Invok no responde."""
    url = f"https://api.github.com{path}"
    req = urllib.request.Request(url)
    req.add_header("Accept", "application/vnd.github+json")
    req.add_header("User-Agent", "aigent-work-hunter")
    if GH_TOKEN:
        req.add_header("Authorization", f"Bearer {GH_TOKEN}")
    for attempt in range(retries + 1):
        try:
            with urllib.request.urlopen(req, timeout=25) as r:
                return json.loads(r.read().decode())
        except urllib.error.HTTPError as e:
            if e.code in (403, 429) and attempt < retries:
                reset = int(e.headers.get("X-RateLimit-Reset", "0")) or 0
                delta = max(5, min(reset - time.time(), 90)) if reset else 65
                log(f"rate limit, wait {int(delta)}s")
                time.sleep(delta)
                continue
            log(f"HTTP {e.code} {path}")
            return None
        except Exception as e:
            log(f"error {path}: {e}")
            return None
    return None


MODE = "invok"  # o "direct", resuelto en main()


def api_search(q, per_page, sort=None, order=None):
    if MODE == "invok":
        args = {"q": q, "per_page": per_page}
        if sort:
            args["sort"] = sort
        if order:
            args["order"] = order
        return invok_call("github-search-issues", args)
    extra = f"&per_page={per_page}"
    if sort:
        extra += f"&sort={sort}"
    if order:
        extra += f"&order={order}"
    return gh(f"/search/issues?q={urllib.parse.quote(q)}{extra}")


def api_repo(fullname):
    if MODE == "invok":
        owner, repo = fullname.split("/", 1)
        return invok_call("github-get-repo", {"owner": owner, "repo": repo})
    return gh(f"/repos/{fullname}")


def api_timeline(fullname, number):
    if MODE == "invok":
        owner, repo = fullname.split("/", 1)
        return invok_call("github-issue-timeline",
                          {"owner": owner, "repo": repo,
                           "issue_number": int(number), "per_page": 100})
    return gh(f"/repos/{fullname}/issues/{number}/timeline?per_page=100")


def days_ago(iso):
    if not iso:
        return 9999.0
    dt = datetime.fromisoformat(iso.replace("Z", "+00:00"))
    return (datetime.now(timezone.utc) - dt).total_seconds() / 86400


def money_evidence(text, labels):
    """0=ninguna 1=mencionada 2=bounty verificable 3=escrow 4=plataforma reputada."""
    m = MONEY_RE.search(text)
    amount = None
    if m:
        for g in m.groups():
            if g and g.replace(",", "").isdigit():
                amount = int(g.replace(",", ""))
                break
    if PLATFORM_RE.search(text):
        return 4, amount
    if ESCROW_RE.search(text):
        return 3, amount
    has_label = any("bounty" in l.lower() or "💰" in l for l in labels)
    if m and has_label:
        return 2, amount
    if m or has_label:
        return 1, amount
    return 0, None


def competition_factor(prs, comments, is_bounty):
    """P(aceptacion base) por competencia real (PRs cross-referenced)."""
    if prs is None:
        f = 0.5  # sin verificar
    elif prs == 0:
        f = 0.9
    elif prs <= 2:
        f = 0.6
    elif prs <= 8:
        f = 0.3
    else:
        f = 0.12
    if is_bounty and comments > 20:
        f *= 0.7  # comentarios = claims
    return round(f, 2)


def categorize(it, info, prs, evidence):
    is_bounty = any("bounty" in l.lower() or "💰" in l for l in it["labels"])
    title_l = it["title"].lower()
    if any(p in title_l for p in JUNK_TITLE) or any(p in (it["repo"] or "").lower() for p in JUNK_REPO):
        return "SPAM"
    n_repo_issues = repo_counts.get(it["repo"], 0)
    if n_repo_issues >= 8 or (info and info.get("open_issues", 0) > 500 and "bounty" in it["repo"].lower()):
        return "BOUNTY_FARM"
    if any(w in title_l or w in it.get("body", "").lower() for w in BAIT_WORDS):
        return "TOKEN_FARM"
    if info and (info.get("archived") or info.get("days_since_push", 999) > 120):
        return "DEAD_REPO"
    if evidence[0] == 0 and is_bounty:
        return "AMBIGUOUS"
    if len(it["title"]) < 15 or len(it.get("body", "")) < 100:
        return "AMBIGUOUS"
    prs_n = prs if prs is not None else 99
    if is_bounty:
        return "REAL_HIGH_COMPETITION" if (prs_n > 2 or it["comments"] > 20) else "REAL_LOW_COMPETITION"
    # orphan: sin bounty, repo activo, 0 PRs, issue clara
    if prs_n == 0 and it.get("age_days", 0) > 45:
        return "GOOD_OPPORTUNITY"
    return "AMBIGUOUS"


def task_type(title, body):
    """Clasifica la tarea: engagement = siempre SKIP (no puede hacerla un bot
    sin violar ToS: stars/forks/follows son acciones de cuenta humana)."""
    t = (title + " " + body).lower()
    if re.search(r"\b(star|fork|watch|follow|subscribe|clap|react to)\b.{0,30}\b(repo|repository|project|us)\b", t) \
            or "pay-per-star" in t or "paga por estrella" in t:
        return "engagement"
    if re.search(r"\b(test|coverage|e2e|unit test)", t):
        return "test"
    if re.search(r"\b(docs|documentation|readme|jsdoc|tutorial)", t):
        return "docs"
    if re.search(r"\b(bug|fix|ci|cd|docker|build|dependency|api|implement|add|refactor)", t):
        return "code"
    return "unknown"


def bot_hours_estimate(it):
    """Horas de bot estimadas (crudas; el Judge las refina). El bot no sufre
    el costo de atención humana: una tarea 'aburrida' le pesa igual que una
    interesante. Solo importa volumen de trabajo mecánico."""
    tt = it.get("task_type", "unknown")
    base = {"docs": 0.4, "test": 0.8, "code": 1.2, "engagement": 0.1, "unknown": 1.0}.get(tt, 1.0)
    body = it.get("body", "")
    base *= 1 + min(1.0, len(body) / 4000.0)          # issues largas = más alcance
    base *= 1 + min(0.5, it.get("comments", 0) / 60)  # overhead de coordinación
    return round(base, 2)


def main():
    global repo_counts
    try:
        args = json.loads(sys.stdin.read() or "{}")
    except json.JSONDecodeError:
        args = {}
    max_per_query = int(args.get("max_per_query", 25))
    deep_check = int(args.get("deep_check", 8))
    sources = args.get("sources", ["bounties", "orphan"])
    dataset_path = args.get("dataset_path", "scratch/work_hunter/dataset/opportunities.jsonl")

    # ── 0. resolver modo de acceso: Invok (autenticado, sin ver credenciales) ──
    global MODE
    health = invok_call("github-get-user", {}, retries=1)
    if isinstance(health, dict) and health.get("login"):
        MODE = "invok"
        log(f"modo: invok (autenticado como {health['login']}) — sin cuotas anonimas")
    elif ALLOW_DIRECT:
        MODE = "direct"
        log("modo: direct (fallback) — Invok no responde; cuotas anonimas salvo GH_TOKEN")
    else:
        stats = {"error": "invok_unavailable",
                 "hint": f"Invok no responde en {INVOK_URL}/mcp. Levantalo (docker compose up -d en el proyecto invok) o pasa WORK_HUNTER_ALLOW_DIRECT=1 para forzar fallback sin credenciales."}
        print(json.dumps({"stats": stats, "opportunities": []}, ensure_ascii=False))
        return

    # ── 1. recolectar ──
    items = {}
    for src in sources:
        for q in QUERIES.get(src, []):
            data = api_search(q, min(max_per_query, 50))
            if data and "items" in data:
                for it in data["items"]:
                    url = it.get("html_url", "")
                    if not url or "/pull/" in url:
                        continue
                    repo = url.split("https://github.com/")[1].rsplit("/", 2)[0]
                    items[url] = {
                        "url": url, "repo": repo, "number": it.get("number"),
                        "title": it.get("title", ""),
                        "labels": [l["name"] for l in it.get("labels", [])],
                        "comments": it.get("comments", 0),
                        "reactions": (it.get("reactions") or {}).get("total_count", 0),
                        "created_at": it.get("created_at", ""),
                        "source": src,
                        "body": (it.get("body") or "")[:1500],
                    }
            time.sleep(2 if MODE == "invok" else 7)
    all_items = list(items.values())
    repo_counts = {}
    for it in all_items:
        repo_counts[it["repo"]] = repo_counts.get(it["repo"], 0) + 1
    stats = {"collected": len(all_items), "sources": sources,
             "mode": MODE, "github_user": health.get("login") if isinstance(health, dict) else None}
    log(f"collected: {len(all_items)}")

    # ── 2. pre-filtro cheapo y pre-score ──
    candidates = []
    for it in all_items:
        text = it["title"] + " " + it["body"]
        ev, amount = money_evidence(text, it["labels"])
        it["money_evidence"], it["reward_estimate"] = ev, amount
        is_bounty = any("bounty" in l.lower() or "💰" in l for l in it["labels"])
        it["is_bounty"] = is_bounty
        it["age_days"] = round(days_ago(it["created_at"]), 1)
        it["task_type"] = task_type(it["title"], it["body"])
        it["bot_hours"] = bot_hours_estimate(it)
        # pre-score mecanico (sin datos de repo)
        score = 20 + (12 if is_bounty else 4) + max(0, 18 - it["age_days"] * 0.35)
        score += min(10, math.log2(it["comments"] + 1) * 2) * (0.5 if is_bounty else 1.0)
        score += ev * 6
        score += min(10, it["reactions"] * 2)
        if it["task_type"] == "engagement":
            score = min(score, 30)  # paga-por-stars/forks: nunca interesa
        text_l = text.lower()
        it["stack_hits"] = [kw for kw, _ in STACK_KW if kw in text_l]
        score += sum(p * 0.5 for kw, p in STACK_KW if kw in text_l)
        it["prescore"] = round(score, 1)
        if any(p in it["title"].lower() for p in JUNK_TITLE) or any(p in (it["repo"] or "").lower() for p in JUNK_REPO):
            it["_prefilter"] = "SPAM"
            continue
        if repo_counts[it["repo"]] >= 8:
            it["_prefilter"] = "BOUNTY_FARM"
            continue
        candidates.append(it)
    candidates.sort(key=lambda x: -x["prescore"])
    log(f"after prefilter: {len(candidates)}")

    # ── 3. deep check: repo + PRs competidoras (timeline) sobre el top N ──
    finalists = candidates[:deep_check]
    results = []
    for j, it in enumerate(finalists):
        info = api_repo(it["repo"])
        time.sleep(0.5 if MODE == "invok" else 1.2)
        if not info or "full_name" not in info:
            it["_prefilter"] = "REPO_UNAVAILABLE"
            continue
        repo_info = {
            "stars": info.get("stargazers_count", 0),
            "language": info.get("language", ""),
            "open_issues": info.get("open_issues_count", 0),
            "days_since_push": round(days_ago(info.get("pushed_at", "")), 1),
            "archived": info.get("archived", False),
        }
        if repo_info["archived"] or repo_info["days_since_push"] > 120:
            it["_prefilter"] = "DEAD_REPO"
            continue
        prs = None
        tl = api_timeline(it["repo"], it["number"])
        time.sleep(0.5 if MODE == "invok" else 1.2)
        if isinstance(tl, list):
            prs = len([e for e in tl
                       if e.get("event") == "cross-referenced"
                       and (e.get("source") or {}).get("issue", {}).get("pull_request")])
        ev, amount = it["money_evidence"], it["reward_estimate"]
        cat = categorize(it, repo_info, prs, (ev, amount))
        p_accept = competition_factor(prs, it["comments"], it["is_bounty"])
        if it["task_type"] == "engagement":
            p_accept = round(p_accept * 0.1, 3)
        bot_h = it["bot_hours"]
        bot_hourly = round((amount or 0) * p_accept / bot_h, 2) if bot_h else 0
        auto_attackable = bool(
            it["task_type"] in ("code", "test", "docs")
            and prs is not None and prs <= 2
            and ((amount or 0) >= 10 or ev >= 3)
            and cat in ("REAL_LOW_COMPETITION", "GOOD_OPPORTUNITY")
        )
        results.append({
            "url": it["url"], "repo": it["repo"], "number": it["number"],
            "title": it["title"], "source": it["source"], "category": cat,
            "task_type": it["task_type"],
            "money_evidence": ev, "reward_estimate": amount,
            "competition_prs": prs, "comments": it["comments"],
            "p_accept_base": p_accept, "age_days": it["age_days"],
            "score": it["prescore"], "stack_hits": it["stack_hits"],
            "repo_info": repo_info,
            "bot_hours_estimate": bot_h,
            "bot_hourly_hint": bot_hourly,
            "auto_attackable": auto_attackable,
            "expected_hourly_hint": {
                "reward": amount or 0,
                "p_accept": p_accept,
                "note": "dos metricas: human_hourly = reward x p_accept x automation / human_hours (supervisado) y bot_hourly = reward x p_accept / bot_hours (full-auto). Judge completa automation, human_hours y refina bot_hours",
            },
            "body_snippet": it["body"][:400],
        })
        log(f"[{j+1}/{len(finalists)}] {it['repo']}#{it['number']} -> {cat}/{it['task_type']} (prs={prs}, ev={ev}, bot_h={bot_h}, auto={auto_attackable})")

    results.sort(key=lambda x: -x["score"])

    # ── 4. dataset persistente (JSONL, append) ──
    try:
        os.makedirs(os.path.dirname(dataset_path) or ".", exist_ok=True)
        with open(dataset_path, "a") as f:
            for it in all_items:
                rec = {
                    "ts": datetime.now(timezone.utc).isoformat(),
                    "url": it["url"], "repo": it["repo"], "title": it["title"],
                    "source": it.get("source"), "observed_by": "skill_v0.2",
                    "prefilter": it.get("_prefilter"),
                    "money_evidence": it.get("money_evidence"),
                    "comments": it.get("comments"), "age_days": it.get("age_days"),
                    "score": it.get("prescore"),
                }
                f.write(json.dumps(rec, ensure_ascii=False) + "\n")
        stats["dataset_appended"] = len(all_items)
    except OSError as e:
        log(f"dataset write failed: {e}")
        stats["dataset_appended"] = 0

    stats["finalists"] = len(results)
    print(json.dumps({"stats": stats, "opportunities": results}, ensure_ascii=False, indent=1))


if __name__ == "__main__":
    main()
