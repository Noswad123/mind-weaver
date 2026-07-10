pub mod domain_validation;
pub mod format;
pub mod links;
pub mod metadata;
pub mod parser;
pub mod recipes;
pub mod registry;
pub mod todos;

pub use domain_validation::{
    DomainValidationResult, DomainViolation, canonical_domain_name, validate_domain_notes,
};
pub use format::{FormatStats, format_notes, normalize_markdown_headings};
pub use links::{
    Link, LinkType, ParseContext, is_external_link_target, normalize_wiki_link_target, parse_links,
    parse_links_with_context, resolve_internal_link, resolve_markdown_local_link,
    resolve_markdown_local_link_with_context,
};
pub use metadata::{
    Metadata, ensure_meta_block, ensure_meta_block_and_id, ensure_meta_id, extract_metadata,
    has_meta_block, parse_list_like_value, read_meta_id_from_content, validate_meta_id,
};
pub use parser::{ParsedNote, parse_note, parse_note_with_context};
pub use recipes::{IngredientMention, RecipeProjection, extract_recipe_projection};
pub use registry::{
    BuildResult, Duplicate, Registry, RegistryEntry, ScannedRegistryEntry, build_registry,
    derive_hub_id_from_path, is_hub_note_path,
};
pub use todos::{
    ArchiveStats, SyncStats, TaskIndexTodo, TodoMetadata, TodoUpdateParams,
    archive_completed_to_life_log, get_active_task_index_todo, list_active_task_index_todos,
    refresh_dashboard_from_task_index_notes, sync_dashboard_from_task_index_notes,
    toggle_task_index_todo, update_task_index_todos,
};
