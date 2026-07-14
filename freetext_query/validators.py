from __future__ import annotations

import re

from .config import MAX_ROWS
from .schema import ALLOWED_RELATIONS

try:
    import sqlglot
    from sqlglot import exp
except ImportError:  # pragma: no cover - production installs sqlglot
    sqlglot = None
    exp = None


class UnsafeQuery(ValueError):
    """Raised when generated SQL is not safe to execute."""


_FORBIDDEN = re.compile(
    r"\b(?:insert|update|delete|drop|alter|create|truncate|grant|revoke|comment|"
    r"copy|call|do|execute|prepare|vacuum|analyze|refresh|merge)\b",
    re.IGNORECASE,
)
_FORBIDDEN_FUNCTIONS = re.compile(
    r"\b(?:pg_read_file|pg_read_binary_file|pg_ls_dir|dblink|lo_import|"
    r"lo_export|set_config|current_setting|pg_sleep|nextval|setval|"
    r"pg_advisory_lock|pg_advisory_xact_lock|pg_notify)\s*\(",
    re.IGNORECASE,
)
_FORBIDDEN_LOCKING = re.compile(
    r"\bfor\s+(?:update|no\s+key\s+update|share|key\s+share)\b",
    re.IGNORECASE,
)
_IDENTIFIER = re.compile(r"(?:from|join)\s+([\w.\"]+)", re.IGNORECASE)


def clean_sql(raw: str) -> str:
    sql = str(raw or "").strip()
    sql = re.sub(r"^```(?:sql)?\s*|\s*```$", "", sql, flags=re.IGNORECASE | re.DOTALL).strip()
    return sql


def validate_sql(raw: str) -> str:
    sql = clean_sql(raw)
    if not sql:
        raise UnsafeQuery("generated SQL is empty")
    if "\x00" in sql:
        raise UnsafeQuery("generated SQL contains an invalid character")
    if sql.count(";") > 0:
        raise UnsafeQuery("multiple SQL statements are not allowed")
    if not re.match(r"^(?:select|with)\b", sql, re.IGNORECASE):
        raise UnsafeQuery("only SELECT or WITH queries are allowed")
    if _FORBIDDEN.search(sql):
        raise UnsafeQuery("mutating or administrative SQL is not allowed")
    if _FORBIDDEN_FUNCTIONS.search(sql):
        raise UnsafeQuery("unsafe PostgreSQL functions are not allowed")
    if _FORBIDDEN_LOCKING.search(sql):
        raise UnsafeQuery("locking clauses are not allowed")

    if sqlglot is not None:
        _validate_ast(sql)

    # The allow-list is intentionally lexical in addition to the AST/parser
    # check supplied by the database driver. It rejects unqualified relations
    # so a prompt cannot silently escape the UAT schema.
    cte_names = {name.lower() for name in re.findall(r"\b([A-Za-z_]\w*)\s+AS\s*\(", sql, re.IGNORECASE)}
    saw_allowed_relation = False
    for identifier in _IDENTIFIER.findall(sql):
        relation = identifier.strip('"').lower()
        if relation in cte_names:
            continue
        if relation not in ALLOWED_RELATIONS:
            raise UnsafeQuery(f"relation is not allowed: {identifier}")
        saw_allowed_relation = True
    if not saw_allowed_relation:
        raise UnsafeQuery("query must read an allowed UAT relation")

    return enforce_limit(sql)


def _validate_ast(sql: str) -> None:
    try:
        statements = sqlglot.parse(sql, read="postgres")
    except Exception as exc:
        raise UnsafeQuery("generated SQL could not be parsed") from exc
    if len(statements) != 1:
        raise UnsafeQuery("multiple SQL statements are not allowed")

    tree = statements[0]
    if not isinstance(tree, (exp.Select, exp.Union)):
        raise UnsafeQuery("only SELECT or WITH queries are allowed")
    forbidden_types = (exp.Insert, exp.Update, exp.Delete, exp.Create, exp.Drop, exp.Alter, exp.Command)
    if any(tree.find(node_type) for node_type in forbidden_types):
        raise UnsafeQuery("mutating or administrative SQL is not allowed")
    if hasattr(exp, "Into") and tree.find(exp.Into):
        raise UnsafeQuery("SELECT INTO is not allowed")

    cte_names = {cte.alias_or_name.lower() for cte in tree.find_all(exp.CTE)}
    saw_allowed_relation = False
    for table in tree.find_all(exp.Table):
        table_name = table.name.lower()
        database_name = (table.db or "").lower()
        if not database_name and table_name in cte_names:
            continue
        relation = f"{database_name}.{table_name}" if database_name else table_name
        if relation not in ALLOWED_RELATIONS:
            raise UnsafeQuery(f"relation is not allowed: {relation}")
        saw_allowed_relation = True
    if not saw_allowed_relation:
        raise UnsafeQuery("query must read an allowed UAT relation")


def enforce_limit(sql: str) -> str:
    limit_match = re.search(r"\blimit\s+(\d+)\b", sql, re.IGNORECASE)
    if limit_match:
        requested = int(limit_match.group(1))
        if requested > MAX_ROWS:
            return sql[: limit_match.start()] + f"LIMIT {MAX_ROWS}" + sql[limit_match.end() :]
        return sql
    trailing_comment = re.search(r"(\s*(?:--[^\r\n]*|/\*.*?\*/)\s*)$", sql, re.DOTALL)
    if trailing_comment:
        return f"{sql[:trailing_comment.start()]} LIMIT {MAX_ROWS}{trailing_comment.group(1)}"
    return f"{sql} LIMIT {MAX_ROWS}"
