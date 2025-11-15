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


## Generate Model (DB → Model)

```bash
gorm migrate gen model
gorm migrate gen model --dry-run
```

Generate or update GORM model code based on the current database schema.

* **gen model** – updates model files using schema diff
* **--dry-run** – show changes without writing files

Useful when onboarding an existing database or keeping models in sync.


## Generate Migration (Model → Migration File)

```bash
gorm migrate gen migration
gorm migrate gen migration --dry-run
```

Generate a migration file by comparing your Go models with the actual database schema.

* **gen migration** – writes a new migration file
* **--dry-run** – preview diff without generating anything


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
| `gorm migrate gen model`     | Generate/update model code from DB schema     |
| `gorm migrate gen migration` | Generate migration file from model diff       |
| `gorm migrate diff`          | Show schema differences                       |
