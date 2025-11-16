# GORM Migration CLI

GORM provides a new migration workflow based on **schema diffing**.
It supports generating models from a database, generating migration files from models,
inspecting differences, and applying migrations safely.


## Migration Execution

```bash
gorm migrate init
gorm migrate up
gorm migrate down
gorm migrate status
```

* **init** – sets up the migration directory and runner entrypoint
* **up** – apply pending migrations
* **down** – rollback the last migration
* **status** – show applied and pending migrations


## Reflect Schema (DB → Model)

```bash
gorm migrate reflect
gorm migrate reflect --dry-run
```

Generate or update GORM model code based on the current database schema.

* **reflect** – updates model files using schema diff
* **--dry-run** – show changes without writing files
* **--table** (repeatable) – limit to specific tables


## Create Migration (Model → Migration File)

```bash
gorm migrate create <name>
gorm migrate create <name> --dry-run
gorm migrate create <name> --auto   # requires DB adapter
```

Generate a migration file by comparing your Go models with the actual database schema.

* **create** – writes a new migration file (name is positional)
* **--dry-run** – preview diff/file without generating
* **--auto** – compute from model ↔ DB diff (DB adapter required)


## Compare Model & Database

```bash
gorm migrate diff
```

Show differences between your models and the current database schema.
No files are created.


## Summary

| Command                      | Description                                   |
| ---------------------------- | --------------------------------------------- |
| `gorm migrate init`          | Initialize migration directory and entrypoint |
| `gorm migrate up/down`       | Apply or rollback migrations                  |
| `gorm migrate status`        | Show migration execution status               |
| `gorm migrate reflect`       | Generate/update model code from DB schema     |
| `gorm migrate create <name>` | Generate migration file from model diff       |
| `gorm migrate diff`          | Show schema differences                       |
