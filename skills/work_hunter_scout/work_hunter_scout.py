#!/usr/bin/env python3
"""Work Hunter Scout V0.3 — discovery multi-fuente con esquema unificado.

Fuentes (param `sources`):
  - bounties: issues con label bounty en GitHub (filtra granjas/spam/token)
  - orphan:   issues viejas "help wanted"/"good first issue" en repos activos
  - hn:       gigs de los hilos mensuales 'Ask HN: Freelancer? Seeking freelancer?'
  - remoteok: ofertas remotas (contrato) filtradas por stack tech

Toda la red pasa por Invok (MCP JSON-RPC sobre /mcp): las credenciales de GitHub
viven cifradas en Invok, esta skill jamás las ve. Fallback directo a
api.github.com solo con WORK_HUNTER_ALLOW_DIRECT=1.

Métricas V0.3:
  - money_evidence 0-5 + payment_risk (escrow_platform/milestone/trust_only/employer)
  - fees_pct (0 hasta integrar plataformas con comisión)
  - competencia real: PRs cross-referenced (GitHub), frescura (HN)
  - task_type con matriz "boring work": test/docs/deps = boost; features $100+ = decay
  - orphan_sponsored: help wanted en repo activo con FUNDING.yml (repo u org-level)
  - bot_hours_estimate + bot_hourly + auto_attackable (doble umbral con el Judge)
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
ALLOW_DIRECT = os.environ.get("WORK_HUNTER_ALLOW_DIRECT", "0") == "1"
GH_TOKEN = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")

GH_QUERIES = {
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
BORING_RE = re.compile(r"\b(test|coverage|e2e|unit test|jsdoc|openapi|swagger|deprecat|"
                       r"upgrade|bump|dependency|linter|lint|readme)\w*", re.IGNORECASE)
FEATURE_RE = re.compile(r"\b(feature|implement|build|architect|redesign)\b", re.IGNORECASE)
FREELANCER_QUERIES = ["java spring", "typescript react", "go api", "docker ci"]
GIG_SEEKING_RE = re.compile(r"looking for|we('re| are) (looking|seeking|hiring)|need a freelancer|"
                            r"seeking (a )?freelancer|budget (of |is )?\$|pay(ing)? \$|hiring a", re.IGNORECASE)
GIG_OFFERING_RE = re.compile(r"^i('| a)m available|my portfolio|my rates|i offer|hire me", re.IGNORECASE)
DEV_TAGS = {"go", "golang", "typescript", "javascript", "java", "python", "react", "node",
            "devops", "docker", "backend", "frontend", "full-stack", "django", "spring",
            "angular", "kubernetes", "aws", "postgres", "rust"}
MONEY_RE = re.compile(
    r"[$€]\s?([0-9][0-9,]{1,6})|([0-9][0-9,]{1,6})\s?(USD|USDC|USDT|EUR)\b|(?:0\.\d+|[1-9]\d*)\s?(ETH|SOL)\b",
    re.IGNORECASE)
ESCROW_RE = re.compile(r"escrow|paid on merge|paid on approval|first mergeable", re.IGNORECASE)
PLATFORM_RE = re.compile(r"algora\.dev|polar\.sh|gitcoin", re.IGNORECASE)


def log(m):
    print(m, file=sys.stderr, flush=True)


def _parse_mcp_body(body):
    """Extrae el texto de una respuesta JSON-RPC (SSE o JSON puro) y parsea el
    contenido de la tool (Invok envuelve contenido externo en tags que se limpian)."""
    raw = body
    if "data:" in body and body.lstrip().startswith(("event:", "data:")):
        lines = [l[5:].strip() for l in body.splitlines() if l.startswith("data:")]
        raw = lines[-1] if lines else body
    resp = json.loads(raw)
    if "error" in resp:
        log(f"invok rpc error: {str(resp['error'])[:200]}")
        return None
    result = resp.get("result") or {}
    if result.get("isError"):
        return None
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
    """Ejecuta una tool de Invok via MCP. Credenciales: solo en Invok."""
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
    """Fallback directo (sin credenciales salvo GH_TOKEN en env)."""
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


def api_raw(fullname_repo_path):
    """Descarga un archivo crudo. fullname_repo_path = 'owner/repo/branch/path'."""
    if MODE == "invok":
        owner, repo, rest = fullname_repo_path.split("/", 2)
        branch, file_path = rest.split("/", 1)
        return invok_call("github-raw-get-file",
                          {"owner": owner, "repo": repo, "branch": branch,
                           "file_path": file_path})
    return gh_raw_direct(fullname_repo_path)


def gh_raw_direct(fullname_repo_path):
    try:
        with urllib.request.urlopen(f"https://raw.githubusercontent.com/{fullname_repo_path}",
                                    timeout=20) as r:
            return r.read().decode()
    except Exception:
        return None


def days_ago(iso):
    if not iso:
        return 9999.0
    dt = datetime.fromisoformat(iso.replace("Z", "+00:00"))
    return (datetime.now(timezone.utc) - dt).total_seconds() / 86400


def hours_ago(iso):
    if not iso:
        return 9999.0
    dt = datetime.fromisoformat(iso.replace("Z", "+00:00"))
    return (datetime.now(timezone.utc) - dt).total_seconds() / 3600


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
    if prs is None:
        f = 0.5
    elif prs == 0:
        f = 0.9
    elif prs <= 2:
        f = 0.6
    elif prs <= 8:
        f = 0.3
    else:
        f = 0.12
    if is_bounty and comments > 20:
        f *= 0.7
    return round(f, 2)


def payment_risk(source, ev, sponsored=False, escrow=False):
    if source in ("bounties",):
        if ev >= 3:
            return "escrow_platform"
        if ev == 2:
            return "milestone"
        return "trust_only"
    if source == "orphan":
        return "sponsor_dialogue" if sponsored else "trust_only"
    if source == "hn":
        return "trust_only"
    if source == "freelancer":
        return "escrow_platform" if escrow else "milestone"
    if source == "remoteok":
        return "employer"
    return "trust_only"


def task_type(title, body):
    """Clasifica la tarea. engagement = SKIP siempre (ToS + cuenta humana)."""
    t = (title + " " + body).lower()
    if re.search(r"\b(star|fork|watch|follow|subscribe|clap|react to)\b.{0,30}\b(repo|repository|project|us)\b", t) \
            or "pay-per-star" in t or "paga por estrella" in t:
        return "engagement"
    if re.search(r"\b(test|coverage|e2e|unit test)", t):
        return "test"
    if re.search(r"\b(docs|documentation|readme|jsdoc|openapi|swagger|tutorial)", t):
        return "docs"
    if re.search(r"\b(deprecat|upgrade|bump|dependency|linter|lint)", t):
        return "deps"
    if re.search(r"\b(bug|fix|ci|cd|docker|build|api)", t):
        return "code"
    if re.search(r"\b(implement|feature|refactor|architect|redesign)", t):
        return "feature"
    return "unknown"


def bot_hours_estimate(it):
    tt = it.get("task_type", "unknown")
    base = {"docs": 0.4, "test": 0.8, "deps": 0.5, "code": 1.2,
            "feature": 3.0, "engagement": 0.1, "unknown": 1.0}.get(tt, 1.0)
    body = it.get("body", "")
    base *= 1 + min(1.0, len(body) / 4000.0)
    base *= 1 + min(0.5, it.get("comments", 0) / 60)
    return round(base, 2)


def check_sponsor(fullname):
    """True si el repo tiene FUNDING.yml (propio o en el repo org-level .github)."""
    owner, repo = fullname.split("/", 1)
    content = api_raw(f"{owner}/{repo}/main/.github/FUNDING.yml")
    if not isinstance(content, str):
        content = api_raw(f"{owner}/{repo}/master/.github/FUNDING.yml")
    if not isinstance(content, str):
        content = api_raw(f"{owner}/.github/main/FUNDING.yml")
    if not isinstance(content, str):
        content = api_raw(f"{owner}/.github/master/FUNDING.yml")
    return isinstance(content, str) and len(content) > 5 and "404" not in content[:20]


def collect_github(sources, max_per_query, items):
    for src in sources:
        for q in GH_QUERIES.get(src, []):
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


def collect_hn(items, max_per_query):
    """Dos fuentes HN:
    1. Hilos 'Freelancer? Seeking freelancer?' (gigs directos; este mes: supply-saturated)
    2. Hilo 'Who is Hiring?' → comentarios con contract/freelance/part-time = gigs con budget
    No se filtra agresivo acá: el Judge (LLM) clasifica. La skill excluye solo anuncios
    obvios de freelancers ('SEEKING WORK') que son ruido puro."""
    cutoff = int(time.time()) - 30 * 86400

    def fetch_comments(story_id, extra_query="", per_page=50):
        args = {"query": extra_query, "tags": f"comment,story_{story_id}",
                "hitsPerPage": per_page}
        if cutoff:
            args["numericFilters"] = f"created_at_i>{cutoff}"
        return invok_call("hn-search", args)

    # 1. hilo freelance mensual (validar título: el full-text da falsos positivos)
    threads = invok_call("hn-search", {"query": "freelancer seeking freelancer",
                                       "tags": "story", "hitsPerPage": 8})
    if threads and "hits" in threads:
        threads["hits"] = [h for h in threads["hits"]
                           if "freelancer" in (h.get("title") or "").lower()][:2]
        for th in threads["hits"][:2]:
            sid = th.get("story_id") or th.get("objectID")
            if not sid:
                continue
            comments = fetch_comments(sid)
            if not (comments and "hits" in comments):
                continue
            kept = 0
            for c in comments["hits"]:
                text = c.get("comment_text") or ""
                if not text or len(text) < 60:
                    continue
                clean = re.sub(r"<[^>]+>", " ", text)
                if re.search(r"seeking work", clean[:80], re.IGNORECASE):
                    direction = "offering"
                elif GIG_SEEKING_RE.search(clean):
                    direction = "seeking"
                elif GIG_OFFERING_RE.search(clean[:120]):
                    direction = "offering"
                else:
                    direction = "unclear"
                if direction == "offering":
                    continue
                oid = c.get("objectID")
                items[f"hn-{oid}"] = {
                    "url": f"https://news.ycombinator.com/item?id={oid}",
                    "repo": f"HN thread {sid}",
                    "number": oid,
                    "title": (c.get("story_title") or "HN gig")[:90],
                    "labels": [],
                    "comments": 0,
                    "reactions": 0,
                    "created_at": c.get("created_at", ""),
                    "source": "hn",
                    "body": f"[direction={direction}] " + clean[:1500],
                }
                kept += 1
                if kept >= max_per_query:
                    break
            log(f"hn freelance: thread {sid} -> {kept} candidatos (de {len(comments['hits'])})")
        time.sleep(1)

    # 2. Who is Hiring: comentarios de contrato/freelance = gigs con budget
    # 2. Who is Hiring: descubrir el hilo canónico por RELEVANCIA (search_by_date
    # lo entierra bajo menciones recientes del mismo término)
    hiring = invok_call("hn-search", {"query": "who is hiring", "tags": "story",
                                      "hitsPerPage": 30})
    if hiring and "hits" in hiring:
        matches = [h for h in hiring["hits"]
                   if re.search(r"who is hiring\? \(\w+ \d{4}\)",
                                (h.get("title") or ""), re.IGNORECASE)]
        matches.sort(key=lambda h: h.get("created_at") or "", reverse=True)
        hiring["hits"] = matches[:1]
        for th in hiring["hits"][:1]:
            sid = th.get("story_id") or th.get("objectID")
            if not sid:
                continue
            for kw in ("contract", "freelance"):
                comments = fetch_comments(sid, extra_query=kw, per_page=15)
                if not (comments and "hits" in comments):
                    continue
                for c in comments["hits"]:
                    text = c.get("comment_text") or ""
                    if not text or len(text) < 100:
                        continue
                    oid = c.get("objectID")
                    if f"hn-{oid}" in items:
                        continue
                    clean = re.sub(r"<[^>]+>", " ", text)
                    # los gigs reales del hilo tienen formato estructurado:
                    # "Company | Role | Remote | $X" o líneas "Remote: yes"
                    has_pipes = clean.count("|") >= 2
                    has_remote = re.search(r"remote\s*[:\-]", clean, re.IGNORECASE)
                    if not (has_pipes or has_remote) or len(clean) < 200:
                        continue
                    items[f"hn-{oid}"] = {
                        "url": f"https://news.ycombinator.com/item?id={oid}",
                        "repo": f"HN who-is-hiring {sid}",
                        "number": oid,
                        "title": (th.get("title") or "HN hiring")[:90] + f" [{kw}]",
                        "labels": [],
                        "comments": 0,
                        "reactions": 0,
                        "created_at": c.get("created_at", ""),
                        "source": "hn",
                        "body": f"[direction=seeking] [via={kw}] " + clean[:1500],
                    }
                time.sleep(1)
            log(f"hn who-is-hiring: thread {sid} procesado")
    time.sleep(1)


def collect_remoteok(items, max_per_query):
    jobs = invok_call("remoteok-list-jobs", {})
    if not isinstance(jobs, list):
        log("remoteok: sin datos")
        return
    for j in jobs:
        if not isinstance(j, dict) or not j.get("position"):
            continue
        tags = [t.lower() for t in (j.get("tags") or [])]
        if not (set(tags) & DEV_TAGS) and not any(t in (j.get("description") or "").lower() for t in DEV_TAGS):
            continue
        is_gig = any(t in tags for t in ("contract", "freelance", "part-time")) or \
            "freelance" in (j.get("description") or "").lower()
        if not is_gig:
            continue  # V1: solo gigs/contratos, no full-time
        salary = j.get("salary_min") or 0
        jid = j.get("slug") or j.get("id") or j.get("position")
        items[f"remoteok-{jid}"] = {
            "url": j.get("url") or f"https://remoteok.com/remote-jobs/{jid}",
            "repo": j.get("company") or "remoteok",
            "number": jid,
            "title": j.get("position", "")[:90],
            "labels": tags,
            "comments": 0,
            "reactions": 0,
            "created_at": j.get("date", "")[:19] + "Z" if j.get("date") else "",
            "source": "remoteok",
            "body": ((f"salary: {j.get('salary_min')}-{j.get('salary_max')}. " if salary else "")
                     + re.sub(r"<[^>]+>", " ", j.get("description") or ""))[:1500],
        }
        if len([i for i in items.values() if i["source"] == "remoteok"]) >= max_per_query:
            break
    log("remoteok: procesado")


def collect_freelancer(items, max_per_query):
    """Proyectos activos de Freelancer.com via recipe en Invok (PAT del usuario).
    Campos clave: budget.minimum/maximum (presupuesto real), bid_stats.bid_count
    (competencia directa), is_escrow_project (payment_risk)."""
    for q in FREELANCER_QUERIES:
        data = invok_call("freelancer-search-projects",
                          {"query": q, "limit": min(max_per_query, 25),
                           "full_description": True})
        time.sleep(1)
        if not (isinstance(data, dict) and data.get("result", {}).get("projects")):
            continue
        for p in data["result"]["projects"]:
            pid = p.get("id")
            if not pid or f"freelancer-{pid}" in items:
                continue
            budget = p.get("budget") or {}
            bids = (p.get("bid_stats") or {}).get("bid_count") or 0
            desc = re.sub(r"<[^>]+>", " ", p.get("description") or "")
            currency = (p.get("currency") or {}).get("code", "")
            items[f"freelancer-{pid}"] = {
                "url": f"https://www.freelancer.com/projects/{p.get('seo_url')}" if p.get("seo_url")
                       else f"https://www.freelancer.com/projects/{pid}.html",
                "repo": (p.get("owner_info") or {}).get("username") or f"project-{pid}",
                "number": pid,
                "title": (p.get("title") or "")[:90],
                "labels": [j.get("name", "") for j in (p.get("jobs") or []) if isinstance(j, dict)],
                "comments": bids,          # bids = competencia de la plataforma
                "reactions": 0,
                "created_at": datetime.fromtimestamp(p.get("submitdate") or 0, tz=timezone.utc).isoformat() if p.get("submitdate") else "",
                "source": "freelancer",
                "body": (f"[currency={currency} budget={budget.get('minimum')}-{budget.get('maximum')} "
                         f"[escrow={bool(p.get('is_escrow_project'))}] "
                         f"[bids={bids} avg={(p.get('bid_stats') or {}).get('bid_avg')}] ") + desc[:1400],
            }
            # reward y evidencia directa del presupuesto — SOLO USD cuenta para
            # la métrica (INR/EUR etc. distorsionan el bot_hourly: bug visto en producción)
            it = items[f"freelancer-{pid}"]
            bmin = int(budget.get("minimum") or 0)
            if currency == "USD" and bmin > 0:
                it["money_evidence"] = 3 if p.get("is_escrow_project") else 2
                it["reward_estimate"] = bmin
            else:
                it["money_evidence"] = 0
                it["reward_estimate"] = None
            it["_freelancer_budget"] = budget
            it["_freelancer_escrow"] = bool(p.get("is_escrow_project"))
        log(f"freelancer: '{q}' procesado")
    time.sleep(1)


def bid_competition_factor(bids):
    """p_accept base por cantidad de bids en la plataforma."""
    if bids <= 5:
        return 0.7
    if bids <= 20:
        return 0.45
    if bids <= 50:
        return 0.3
    if bids <= 100:
        return 0.2
    return 0.12


def main():
    global MODE, repo_counts
    try:
        args = json.loads(sys.stdin.read() or "{}")
    except json.JSONDecodeError:
        args = {}
    max_per_query = int(args.get("max_per_query", 25))
    deep_check_n = int(args.get("deep_check", 8))
    sources = args.get("sources", ["bounties", "orphan", "hn", "remoteok", "freelancer"])
    dataset_path = args.get("dataset_path", "scratch/work_hunter/dataset/opportunities.jsonl")

    # ── 0. modo de acceso ──
    health = invok_call("github-get-user", {}, retries=1)
    if isinstance(health, dict) and health.get("login"):
        MODE = "invok"
        log(f"modo: invok (autenticado como {health['login']})")
    elif ALLOW_DIRECT:
        MODE = "direct"
        log("modo: direct (fallback, sin credenciales)")
    else:
        stats = {"error": "invok_unavailable",
                 "hint": f"Invok no responde en {INVOK_URL}/mcp. Levantalo o pasa WORK_HUNTER_ALLOW_DIRECT=1."}
        print(json.dumps({"stats": stats, "opportunities": []}, ensure_ascii=False))
        return

    # ── 1. recolectar (dispatch por fuente) ──
    items = {}
    gh_sources = [s for s in sources if s in GH_QUERIES]
    if gh_sources:
        collect_github(gh_sources, max_per_query, items)
    if "hn" in sources:
        collect_hn(items, max_per_query)
    if "remoteok" in sources:
        collect_remoteok(items, max_per_query)
    if "freelancer" in sources:
        collect_freelancer(items, max_per_query)

    all_items = list(items.values())
    repo_counts = {}
    for it in all_items:
        repo_counts[it["repo"]] = repo_counts.get(it["repo"], 0) + 1
    stats = {"collected": len(all_items), "sources": sources, "mode": MODE,
             "github_user": health.get("login") if isinstance(health, dict) else None}
    log(f"collected: {len(all_items)}")

    # ── 2. pre-filtro + pre-score ──
    candidates = []
    for it in all_items:
        text = it["title"] + " " + it["body"]
        ev, amount = money_evidence(text, it["labels"])
        if it["source"] == "freelancer":
            # el collector ya trajo budget/bids reales del API: no pisar
            ev, amount = it["money_evidence"], it["reward_estimate"]
        else:
            it["money_evidence"], it["reward_estimate"] = ev, amount
        is_bounty = any("bounty" in l.lower() or "💰" in l for l in it["labels"])
        it["is_bounty"] = is_bounty
        it["age_days"] = round(days_ago(it["created_at"]), 1)
        it["task_type"] = task_type(it["title"], it["body"])
        it["bot_hours"] = bot_hours_estimate(it)
        score = 20 + (12 if is_bounty else 4) + max(0, 18 - it["age_days"] * 0.35)
        score += min(10, math.log2(it["comments"] + 1) * 2) * (0.5 if is_bounty else 1.0)
        score += ev * 6
        score += min(10, it["reactions"] * 2)
        if it["task_type"] == "engagement":
            score = min(score, 30)
        if it["task_type"] in ("test", "docs", "deps"):
            score += 8   # matriz boring work: nadie humano quiere esto
        if it["task_type"] == "feature" and (amount or 0) >= 100:
            score -= 15  # features grandes: competencia por diseño
        if it["source"] == "hn" and it["age_days"] <= 7:
            score += 15  # gig fresco en HN: velocidad gana
        it["stack_hits"] = [kw for kw, _ in STACK_KW if kw in text.lower()]
        score += sum(p * 0.5 for kw, p in STACK_KW if kw in text.lower())
        it["prescore"] = round(score, 1)
        title_l = it["title"].lower()
        if any(p in title_l for p in JUNK_TITLE) or any(p in (it["repo"] or "").lower() for p in JUNK_REPO):
            it["_prefilter"] = "SPAM"
            continue
        if it["source"] in GH_QUERIES and repo_counts[it["repo"]] >= 8:
            it["_prefilter"] = "BOUNTY_FARM"
            continue
        candidates.append(it)
    candidates.sort(key=lambda x: -x["prescore"])
    log(f"after prefilter: {len(candidates)}")

    # ── 3. deep-check (solo GitHub) + esquema unificado ──
    gh_finalists = [c for c in candidates if c["source"] in GH_QUERIES][:deep_check_n]
    results = []
    deep_checked_urls = set()
    for j, it in enumerate(gh_finalists):
        info = api_repo(it["repo"])
        time.sleep(0.5 if MODE == "invok" else 1.2)
        if not info or "full_name" not in info:
            it["_prefilter"] = "REPO_UNAVAILABLE"
            continue
        if info.get("archived") or days_ago(info.get("pushed_at", "")) > 120:
            it["_prefilter"] = "DEAD_REPO"
            continue
        prs = None
        closed_pr_attempts = None
        tl = api_timeline(it["repo"], it["number"])
        time.sleep(0.5 if MODE == "invok" else 1.2)
        if isinstance(tl, list):
            cross = [e for e in tl
                     if e.get("event") == "cross-referenced"
                     and (e.get("source") or {}).get("issue", {}).get("pull_request")]
            prs = len(cross)
            # gente que intentó y no logró merge: señal de dificultad (o de maintainer duro)
            closed_pr_attempts = len([e for e in cross
                                      if (e["source"]["issue"].get("state") == "closed"
                                          and not e["source"]["issue"]["pull_request"].get("merged_at"))])
        ev, amount = it["money_evidence"], it["reward_estimate"]
        cat = categorize(it, prs, (ev, amount))
        p_accept = competition_factor(prs, it["comments"], it["is_bounty"])
        if it["task_type"] == "engagement":
            p_accept = round(p_accept * 0.1, 3)
        sponsored = it["source"] == "orphan" and check_sponsor(it["repo"])
        time.sleep(0.5 if MODE == "invok" else 1.2)
        bot_h = it["bot_hours"]
        bot_hourly = round((amount or 0) * p_accept / bot_h, 2) if bot_h else 0
        auto_attackable = bool(
            it["task_type"] in ("test", "docs", "deps", "code")
            and prs is not None and prs <= 2
            and ((amount or 0) >= 10 or ev >= 3)
            and cat in ("REAL_LOW_COMPETITION", "GOOD_OPPORTUNITY", "ORPHAN_SPONSORED")
        )
        deep_checked_urls.add(it["url"])
        results.append(_unified(it, cat, prs, p_accept, sponsored, bot_h, bot_hourly,
                                auto_attackable, repo_info={
                                    "stars": info.get("stargazers_count", 0),
                                    "language": info.get("language", ""),
                                    "open_issues": info.get("open_issues_count", 0),
                                    "days_since_push": round(days_ago(info.get("pushed_at", "")), 1)},
                                closed_pr_attempts=closed_pr_attempts))
        log(f"[{j+1}/{len(gh_finalists)}] {it['repo']}#{it['number']} -> {cat}/{it['task_type']} "
            f"(prs={prs}, closed_attempts={closed_pr_attempts}, ev={ev}, sponsor={sponsored}, auto={auto_attackable})")

    # no-GitHub: pasan directo con su esquema
    for it in candidates:
        if it["url"] in deep_checked_urls or it["source"] in GH_QUERIES:
            continue
        if it.get("_prefilter"):
            continue
        if it["source"] == "hn":
            p_accept, cat = 0.7, "GIG_FRESH"
        elif it["source"] == "freelancer":
            p_accept = bid_competition_factor(it["comments"])
            cat = "ESCROW_ACTIVE" if it.get("_freelancer_escrow") else "PROJECT_ACTIVE"
        else:
            p_accept, cat = 0.4, "CONTRACT"
        bot_h = it["bot_hours"]
        fees = 0.10 if it["source"] == "freelancer" else 0.0
        reward = it["reward_estimate"] or 0
        if it["source"] == "freelancer":
            auto = (it["task_type"] in ("test", "docs", "deps", "code")
                    and it["comments"] <= 20 and reward >= 10)
        else:
            auto = it["task_type"] in ("test", "docs", "deps", "code")
        results.append(_unified(it, cat, None, p_accept, False, bot_h,
                                round(reward * p_accept * (1 - fees) / max(bot_h, 0.1), 2),
                                auto, None,
                                extra_abandonment=_abandonment_signals_freelancer(it)))

    results.sort(key=lambda x: -x["score"])

    # ── 4. dataset persistente ──
    try:
        os.makedirs(os.path.dirname(dataset_path) or ".", exist_ok=True)
        with open(dataset_path, "a") as f:
            for it in all_items:
                f.write(json.dumps({
                    "ts": datetime.now(timezone.utc).isoformat(),
                    "url": it["url"], "repo": it["repo"], "title": it["title"],
                    "source": it.get("source"), "observed_by": "skill_v0.3",
                    "prefilter": it.get("_prefilter"),
                    "money_evidence": it.get("money_evidence"),
                    "task_type": it.get("task_type"),
                    "score": it.get("prescore"),
                }, ensure_ascii=False) + "\n")
        stats["dataset_appended"] = len(all_items)
    except OSError as e:
        log(f"dataset write failed: {e}")
        stats["dataset_appended"] = 0

    stats["finalists"] = len(results)
    print(json.dumps({"stats": stats, "opportunities": results[:25]}, ensure_ascii=False, indent=1))


def _abandonment_signals_freelancer(it):
    """Señales de por qué un proyecto de Freelancer no recibe bids/atención."""
    budget = it.get("_freelancer_budget") or {}
    bmin, bmax = budget.get("minimum") or 0, budget.get("maximum") or 0
    return {
        "age_days": it.get("age_days"),
        "bid_count": it.get("comments"),
        "budget_min": bmin, "budget_max": bmax,
        "escrow": it.get("_freelancer_escrow", False),
    }


def _unified(it, category, prs, p_accept, sponsored, bot_h, bot_hourly, auto_attackable, repo_info,
             closed_pr_attempts=None, extra_abandonment=None):
    escrow = bool(it.get("_freelancer_escrow"))
    fees = 0.10 if it["source"] == "freelancer" else 0.0
    abandonment = extra_abandonment or {
        "age_days": it.get("age_days"),
        "prs_competing": prs,
        "closed_pr_attempts": closed_pr_attempts,
        "sponsored": sponsored,
        "repo_days_since_push": (repo_info or {}).get("days_since_push"),
    }
    return {
        "url": it["url"], "repo": it["repo"], "number": str(it["number"]),
        "title": it["title"], "source": it["source"],
        "type": {"bounties": "bounty", "orphan": "orphan_sponsored" if sponsored else "orphan",
                 "hn": "gig", "remoteok": "contract", "freelancer": "project"}.get(it["source"], "unknown"),
        "category": "ORPHAN_SPONSORED" if (sponsored and it["source"] == "orphan") else category,
        "task_type": it["task_type"],
        "money_evidence": it["money_evidence"], "reward_estimate": it["reward_estimate"],
        "payment_risk": payment_risk(it["source"], it["money_evidence"], sponsored, escrow),
        "fees_pct": fees,
        "competition_prs": prs,
        "competition": prs if prs is not None else it["comments"],
        "p_accept_base": p_accept, "age_days": it["age_days"],
        "score": it["prescore"], "stack_hits": it["stack_hits"],
        "repo_info": repo_info,
        "bot_hours_estimate": bot_h, "bot_hourly_hint": bot_hourly,
        "auto_attackable": auto_attackable,
        "abandonment_signals": abandonment,
        "body_snippet": it["body"][:400],
    }


def categorize(it, prs, evidence):
    is_bounty = any("bounty" in l.lower() or "💰" in l for l in it["labels"])
    title_l = it["title"].lower()
    if any(p in title_l for p in JUNK_TITLE) or any(p in (it["repo"] or "").lower() for p in JUNK_REPO):
        return "SPAM"
    if repo_counts.get(it["repo"], 0) >= 8 or (is_bounty and "bounty" in it["repo"].lower()):
        return "BOUNTY_FARM"
    if any(w in title_l or w in it.get("body", "").lower() for w in BAIT_WORDS):
        return "TOKEN_FARM"
    if evidence[0] == 0 and is_bounty:
        return "AMBIGUOUS"
    if len(it["title"]) < 15 or len(it.get("body", "")) < 100:
        return "AMBIGUOUS"
    prs_n = prs if prs is not None else 99
    if is_bounty:
        return "REAL_HIGH_COMPETITION" if (prs_n > 2 or it["comments"] > 20) else "REAL_LOW_COMPETITION"
    if prs_n == 0 and it.get("age_days", 0) > 45:
        return "GOOD_OPPORTUNITY"
    return "AMBIGUOUS"


if __name__ == "__main__":
    main()
