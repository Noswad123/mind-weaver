# Hive Sync Backup & Recovery

This guide establishes data safety for `hive-sync-api` (Cloud SQL Postgres).

## What this covers

1. Enable Cloud SQL automated backups + PITR
2. Create backup export bucket + retention lifecycle
3. Run on-demand SQL export backups
4. Restore procedure reference

## 1) Enable backup baseline

```bash
PROJECT_ID="hive-mind-492419" \
REGION="us-east1" \
CLOUD_SQL_INSTANCE="hive-sync-pg" \
scripts/cloud/setup-hive-sync-backups.sh
```

Default behavior:

- backup window: `03:00` UTC
- retained automated backups: `14`
- retained transaction logs (PITR): `7` days
- backup bucket: `gs://<project>-hive-sync-backups`
- object lifecycle delete age: `30` days

## 2) Run manual backup export

```bash
PROJECT_ID="hive-mind-492419" \
CLOUD_SQL_INSTANCE="hive-sync-pg" \
CLOUD_SQL_DB_NAME="hive_sync" \
BACKUP_BUCKET="hive-mind-492419-hive-sync-backups" \
scripts/cloud/export-hive-sync-backup.sh
```

This writes a timestamped dump like:

`gs://hive-mind-492419-hive-sync-backups/sql-exports/hive-sync-pg-hive_sync-<timestamp>.sql.gz`

### If export fails with `HTTPError 412` bucket permissions

Grant Cloud SQL service account access to the backup bucket:

```bash
PROJECT_ID="hive-mind-492419"
CLOUD_SQL_INSTANCE="hive-sync-pg"
BACKUP_BUCKET="hive-mind-492419-hive-sync-backups"

CLOUD_SQL_SA="$(gcloud sql instances describe "$CLOUD_SQL_INSTANCE" --project "$PROJECT_ID" --format='value(serviceAccountEmailAddress)')"

gcloud storage buckets add-iam-policy-binding "gs://$BACKUP_BUCKET" \
  --project "$PROJECT_ID" \
  --member "serviceAccount:$CLOUD_SQL_SA" \
  --role "roles/storage.objectAdmin"
```

Then rerun export command.

## 3) Restore from a backup

```bash
gcloud sql import sql hive-sync-pg \
  gs://hive-mind-492419-hive-sync-backups/sql-exports/<backup-file>.sql.gz \
  --database=hive_sync \
  --project=hive-mind-492419
```

## 4) Recovery checklist

1. Confirm target instance and database.
2. Export a fresh pre-restore backup before importing.
3. Import selected backup file.
4. Run `mw sync doctor --skip-remote` and remote doctor check.
5. Verify API health and sample client sync cycle.

## 5) Optional scheduling

For extra safety beyond automated backups, schedule `export-hive-sync-backup.sh` daily (e.g. cron/launchd/Cloud Scheduler runner) to keep portable SQL dumps.
