use std::{collections::BTreeMap, fs, path::Path};

use crate::read_meta_id_from_content;

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct RegistryEntry {
    pub id: String,
    pub path: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct Registry {
    pub entries: BTreeMap<String, RegistryEntry>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Duplicate {
    pub id: String,
    pub paths: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct BuildResult {
    pub registry: Registry,
    pub duplicates: Vec<Duplicate>,
    pub missing_hub: Vec<String>,
    pub missing_meta: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ScannedRegistryEntry {
    pub id: String,
    pub path: String,
    pub is_hub: bool,
}

#[derive(Debug, Clone)]
struct ScannedNote {
    rel_path: String,
    is_hub: bool,
    content: String,
}

pub fn build_registry(notes_root: impl AsRef<Path>) -> std::io::Result<BuildResult> {
    let root = notes_root.as_ref();
    let root_base = root
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or_default()
        .to_string();

    let mut result = BuildResult::default();
    let mut seen: BTreeMap<String, Vec<String>> = BTreeMap::new();

    for note in walk_markdown_notes(root)? {
        let Some(id) = extract_id(&note, &mut result, &root_base) else {
            continue;
        };
        let id = id.trim().to_string();
        if id.is_empty() {
            continue;
        }
        seen.entry(id).or_default().push(note.rel_path);
    }

    let duplicate_ids = detect_duplicates(&mut result, &mut seen);
    populate_registry(&mut result, &seen, &duplicate_ids);
    result.missing_hub.sort();
    result.missing_meta.sort();

    Ok(result)
}

pub fn is_hub_note_path(path: &str) -> bool {
    path.replace('\\', "/")
        .rsplit('/')
        .next()
        .is_some_and(|base| base.eq_ignore_ascii_case("hub.md"))
}

pub fn derive_hub_id_from_path(rel_path: &str, notes_root_base: &str) -> String {
    let clean_rel = clean_slash_path(rel_path.trim());
    if clean_rel == "." || clean_rel == "/" || clean_rel.is_empty() {
        return String::new();
    }

    let parent_dir = clean_rel
        .rsplit_once('/')
        .map(|(dir, _)| dir)
        .unwrap_or_default();
    let parent = parent_dir.rsplit('/').next().unwrap_or_default().trim();
    if !parent.is_empty() && parent != "." && parent != "/" {
        return parent.to_string();
    }

    let root = notes_root_base.trim();
    if root.is_empty() || root == "." || root == "/" {
        return String::new();
    }
    root.to_string()
}

fn walk_markdown_notes(root: &Path) -> std::io::Result<Vec<ScannedNote>> {
    let mut out = Vec::new();
    walk_markdown_notes_into(root, root, &mut out)?;
    out.sort_by(|a, b| a.rel_path.cmp(&b.rel_path));
    Ok(out)
}

fn walk_markdown_notes_into(
    root: &Path,
    dir: &Path,
    out: &mut Vec<ScannedNote>,
) -> std::io::Result<()> {
    for entry in fs::read_dir(dir)? {
        let entry = entry?;
        let path = entry.path();
        let file_type = entry.file_type()?;
        if file_type.is_dir() {
            if entry.file_name() == ".git" {
                continue;
            }
            walk_markdown_notes_into(root, &path, out)?;
            continue;
        }
        if !file_type.is_file()
            || !path
                .extension()
                .and_then(|ext| ext.to_str())
                .is_some_and(|ext| ext.eq_ignore_ascii_case("md"))
        {
            continue;
        }

        let rel_path = path
            .strip_prefix(root)
            .unwrap_or(&path)
            .to_string_lossy()
            .replace('\\', "/");
        let content = fs::read_to_string(&path)?;
        let is_hub = is_hub_note_path(&rel_path);
        out.push(ScannedNote {
            rel_path,
            is_hub,
            content,
        });
    }
    Ok(())
}

fn extract_id(
    note: &ScannedNote,
    result: &mut BuildResult,
    notes_root_base: &str,
) -> Option<String> {
    let (id, has_meta) = read_meta_id_from_content(&note.content);
    let id = id.trim().to_string();

    if note.is_hub {
        if !has_meta || id.is_empty() {
            let derived = derive_hub_id_from_path(&note.rel_path, notes_root_base);
            if derived.is_empty() {
                result.missing_hub.push(note.rel_path.clone());
                return None;
            }
            if !has_meta {
                result.missing_meta.push(note.rel_path.clone());
            }
            return Some(derived);
        }
        return Some(id);
    }

    if !has_meta || id.is_empty() {
        let file_name = note.rel_path.rsplit('/').next().unwrap_or(&note.rel_path);
        let fallback = file_name
            .rsplit_once('.')
            .map(|(stem, _)| stem)
            .unwrap_or(file_name)
            .trim()
            .to_string();
        if !has_meta {
            result.missing_meta.push(note.rel_path.clone());
        }
        return Some(fallback);
    }

    Some(id)
}

fn detect_duplicates(
    result: &mut BuildResult,
    seen: &mut BTreeMap<String, Vec<String>>,
) -> BTreeMap<String, bool> {
    let mut duplicate_ids = BTreeMap::new();
    for (id, paths) in seen.iter_mut() {
        if paths.len() <= 1 {
            continue;
        }
        paths.sort();
        duplicate_ids.insert(id.clone(), true);
        result.duplicates.push(Duplicate {
            id: id.clone(),
            paths: paths.clone(),
        });
    }
    result.duplicates.sort_by(|a, b| a.id.cmp(&b.id));
    duplicate_ids
}

fn populate_registry(
    result: &mut BuildResult,
    seen: &BTreeMap<String, Vec<String>>,
    duplicate_ids: &BTreeMap<String, bool>,
) {
    for (id, paths) in seen {
        if duplicate_ids.contains_key(id) || paths.is_empty() {
            continue;
        }
        result.registry.entries.insert(
            id.clone(),
            RegistryEntry {
                id: id.clone(),
                path: paths[0].clone(),
            },
        );
    }
}

fn clean_slash_path(path: &str) -> String {
    let absolute = path.starts_with('/');
    let mut stack: Vec<&str> = Vec::new();
    let normalized = path.replace('\\', "/");
    for part in normalized.split('/') {
        match part {
            "" | "." => {}
            ".." => {
                if stack.pop().is_none() && !absolute {
                    stack.push("..");
                }
            }
            _ => stack.push(part),
        }
    }
    let joined = stack.join("/");
    if absolute {
        format!("/{joined}")
    } else if joined.is_empty() {
        ".".to_string()
    } else {
        joined
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn derives_hub_id_from_parent_folder_when_missing() {
        let root = tempfile::tempdir().unwrap();
        write_file(root.path().join("projects/hub.md"), "# Projects\n");

        let result = build_registry(root.path()).unwrap();
        let entry = result.registry.entries.get("projects").unwrap();

        assert_eq!(entry.path, "projects/hub.md");
        assert!(result.missing_hub.is_empty());
    }

    #[test]
    fn derives_root_hub_id_from_notes_root_folder() {
        let root = tempfile::tempdir().unwrap();
        let root_base = root
            .path()
            .file_name()
            .unwrap()
            .to_string_lossy()
            .to_string();
        write_file(root.path().join("hub.md"), "# Root Hub\n");

        let result = build_registry(root.path()).unwrap();
        let entry = result.registry.entries.get(&root_base).unwrap();
        assert_eq!(entry.path, "hub.md");
    }

    #[test]
    fn preserves_explicit_hub_id() {
        let root = tempfile::tempdir().unwrap();
        write_file(
            root.path().join("projects/hub.md"),
            "---\nid: custom-id\n---\n\n# Projects\n",
        );

        let result = build_registry(root.path()).unwrap();
        assert!(!result.registry.entries.contains_key("projects"));
        assert_eq!(
            result.registry.entries.get("custom-id").unwrap().path,
            "projects/hub.md"
        );
    }

    #[test]
    fn detects_duplicates_and_skips_duplicate_registry_entries() {
        let root = tempfile::tempdir().unwrap();
        write_file(root.path().join("a.md"), "---\nid: same\n---\n");
        write_file(root.path().join("b.md"), "---\nid: same\n---\n");

        let result = build_registry(root.path()).unwrap();
        assert!(!result.registry.entries.contains_key("same"));
        assert_eq!(result.duplicates.len(), 1);
        assert_eq!(result.duplicates[0].paths, vec!["a.md", "b.md"]);
    }

    fn write_file(path: impl AsRef<Path>, content: &str) {
        let path = path.as_ref();
        fs::create_dir_all(path.parent().unwrap()).unwrap();
        fs::write(path, content).unwrap();
    }
}
