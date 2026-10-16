use anyhow::{Context, Result, ensure};
use rusqlite::{Connection, params};
use serde::Serialize;

use crate::NoteDb;

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct IdTarget {
    pub uid: String,
    #[serde(rename = "noteID")]
    pub note_id: i64,
    pub path: String,
    pub title: String,
    pub is_hub: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct IdCandidate {
    #[serde(rename = "noteID")]
    pub note_id: Option<i64>,
    pub path: String,
    pub title: Option<String>,
    pub is_hub: bool,
    pub reason: String,
}

#[derive(Debug, Serialize)]
#[serde(tag = "status", rename_all = "camelCase")]
pub enum IdExactStatus {
    Resolved { target: IdTarget },
    Conflict { candidates: Vec<IdCandidate> },
    NotFound,
}

#[derive(Debug, Serialize)]
#[serde(tag = "mode", rename_all = "camelCase")]
pub enum IdQueryResult {
    Prefix {
        prefix: String,
        items: Vec<IdTarget>,
        #[serde(rename = "hasMore")]
        has_more: bool,
    },
    Exact {
        uid: String,
        #[serde(flatten)]
        result: IdExactStatus,
    },
}

pub(crate) fn validate_schema(conn: &Connection) -> Result<()> {
    for sql in [
        "SELECT id, path, title FROM notes LIMIT 0",
        "SELECT note_id, note_uid, path, is_hub FROM note_ids LIMIT 0",
        "SELECT note_id, note_uid, path, is_hub, reason FROM note_id_conflicts LIMIT 0",
    ] {
        conn.prepare(sql).context(
            "absent or incompatible notes registry index; explicitly initialize/ingest/register with a compatible engine before querying (no migrations were applied)",
        )?;
    }
    Ok(())
}

impl NoteDb {
    pub fn query_ids_prefix(&mut self, prefix: &str, limit: i64) -> Result<IdQueryResult> {
        ensure!((1..=200).contains(&limit), "prefix limit must be 1..200");
        let tx = self.conn.transaction()?;
        validate_schema(&tx)?;
        let mut items = {
            let mut stmt = tx.prepare(
                "SELECT r.note_uid, r.note_id, r.path, COALESCE(n.title, ''), r.is_hub
                 FROM note_ids r JOIN notes n ON n.id = r.note_id
                 WHERE substr(r.note_uid, 1, length(?1)) = ?1 COLLATE BINARY
                   AND length(r.note_uid) > 0 AND r.note_id > 0 AND length(r.path) > 0
                   AND n.path = r.path COLLATE BINARY
                   AND (SELECT count(*) FROM notes owner WHERE owner.id = r.note_id) = 1
                   AND (SELECT count(*) FROM note_ids owner
                        WHERE owner.note_uid = r.note_uid COLLATE BINARY) = 1
                   AND NOT EXISTS (SELECT 1 FROM note_id_conflicts c
                                   WHERE c.note_uid = r.note_uid COLLATE BINARY)
                 ORDER BY r.note_uid COLLATE BINARY LIMIT ?2",
            )?;
            stmt.query_map(params![prefix, limit + 1], |row| {
                Ok(IdTarget {
                    uid: row.get(0)?,
                    note_id: row.get(1)?,
                    path: row.get(2)?,
                    title: row.get(3)?,
                    is_hub: row.get::<_, bool>(4)?,
                })
            })?
            .collect::<rusqlite::Result<Vec<_>>>()?
        };
        let has_more = items.len() > limit as usize;
        items.truncate(limit as usize);
        tx.commit()?;
        Ok(IdQueryResult::Prefix {
            prefix: prefix.to_owned(),
            items,
            has_more,
        })
    }

    pub fn query_ids_exact(&mut self, uid: &str) -> Result<IdQueryResult> {
        ensure!(
            !uid.trim().is_empty(),
            "--uid must not be empty or whitespace"
        );
        let tx = self.conn.transaction()?;
        validate_schema(&tx)?;
        let mut candidates = {
            let mut stmt = tx.prepare(
                "SELECT c.note_id, c.path, n.title, c.is_hub, c.reason
                 FROM note_id_conflicts c LEFT JOIN notes n ON n.id = c.note_id
                 WHERE c.note_uid = ?1 COLLATE BINARY",
            )?;
            stmt.query_map([uid], |row| {
                Ok(IdCandidate {
                    note_id: row.get(0)?,
                    path: row.get(1)?,
                    title: row.get(2)?,
                    is_hub: row.get(3)?,
                    reason: row.get(4)?,
                })
            })?
            .collect::<rusqlite::Result<Vec<_>>>()?
        };
        let registered = {
            let mut stmt = tx.prepare(
                "SELECT r.note_id, r.path, n.title, r.is_hub, n.path
                 FROM note_ids r LEFT JOIN notes n ON n.id = r.note_id
                 WHERE r.note_uid = ?1 COLLATE BINARY",
            )?;
            stmt.query_map([uid], |row| {
                Ok((
                    IdCandidate {
                        note_id: row.get(0)?,
                        path: row.get(1)?,
                        title: row.get(2)?,
                        is_hub: row.get(3)?,
                        reason: String::new(),
                    },
                    row.get::<_, Option<String>>(4)?,
                ))
            })?
            .collect::<rusqlite::Result<Vec<_>>>()?
        };
        let has_conflicts = !candidates.is_empty();
        let multiple = registered.len() > 1;
        let mut target = None;
        for (mut candidate, note_path) in registered {
            let consistent = candidate.note_id.is_some_and(|id| id > 0)
                && !candidate.path.is_empty()
                && note_path.as_deref() == Some(candidate.path.as_str());
            if consistent && !multiple && !has_conflicts {
                target = Some(IdTarget {
                    uid: uid.to_owned(),
                    note_id: candidate.note_id.unwrap(),
                    path: candidate.path,
                    title: candidate.title.unwrap_or_default(),
                    is_hub: candidate.is_hub,
                });
            } else {
                candidate.reason = if !consistent {
                    "INDEX_INCONSISTENT"
                } else if has_conflicts {
                    "REGISTERED_WITH_CONFLICT"
                } else {
                    "DUPLICATE_ID"
                }
                .to_owned();
                candidates.push(candidate);
            }
        }
        for candidate in &candidates {
            ensure!(
                !candidate.path.is_empty()
                    && !candidate.reason.is_empty()
                    && candidate.note_id.is_none_or(|id| id > 0),
                "inconsistent registry candidate for {uid:?}; repair and explicitly rebuild the index"
            );
        }
        candidates.sort_by(|a, b| {
            (&a.path, &a.reason, a.note_id, &a.title, a.is_hub)
                .cmp(&(&b.path, &b.reason, b.note_id, &b.title, b.is_hub))
        });
        candidates
            .dedup_by(|a, b| a.path == b.path && a.reason == b.reason && a.note_id == b.note_id);
        let result = if !candidates.is_empty() {
            IdExactStatus::Conflict { candidates }
        } else if let Some(target) = target {
            IdExactStatus::Resolved { target }
        } else {
            IdExactStatus::NotFound
        };
        tx.commit()?;
        Ok(IdQueryResult::Exact {
            uid: uid.to_owned(),
            result,
        })
    }
}
