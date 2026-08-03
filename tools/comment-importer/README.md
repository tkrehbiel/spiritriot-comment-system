# Comment Importer Utilities

This directory contains Python scripts for migrating and importing historical comments from various sources (such as WordPress/GraphComment WXR exports or CommentBox backups) into the DynamoDB comment storage backend, as well as checking database contents.

## Prerequisites

These scripts require Python 3 and the `boto3` and `pyyaml` packages.

You can set up a local virtual environment and install the required dependencies:

```bash
# Create virtual environment
python3 -m venv .venv

# Activate virtual environment
source .venv/bin/activate

# Install dependencies
pip install boto3 pyyaml
```

Make sure your terminal has AWS credentials configured (or `AWS_ENDPOINT_URL` set if testing against LocalStack) before running scripts that interact with DynamoDB.

---

## Script Overviews & Usage

### 1. WXR XML Importer (`import_wxr.py`)
Parses a WordPress/GraphComment WXR XML export and converts the comments into a structured YAML file.

```bash
python3 import_wxr.py <wxr_file_path> <output_yaml_path> [options]
```

* **Arguments**:
  - `wxr_file`: Path to the source `.xml` WXR file.
  - `output_file`: Path to write the structured `.yaml` output.
* **Options**:
  - `--prefix`: Prepend a URL path prefix to post links (defaults to `/gaming/{year}`). Useful if your old URLs differed from current ones.

---

### 2. CommentBox JSON Converter (`import_commentbox.py`)
Reads a directory containing CommentBox JSON export files and compiles them into a single structured YAML file.

```bash
python3 import_commentbox.py <json_dir_path> <output_yaml_path>
```

* **Arguments**:
  - `json_dir`: Directory containing the `.json` export files.
  - `yaml_file`: Path to write the structured `.yaml` output.

---

### 3. DynamoDB Comment Backuper (`import_dynamo.py`)
Scans a DynamoDB comments table and exports public comments dated prior to a specified cutoff date into a local YAML file.

```bash
python3 import_dynamo.py <cutoff_date> [options]
```

* **Arguments**:
  - `cutoff_date`: Cutoff date in `YYYY-MM-DD` format (only comments older than this date are exported).
* **Options**:
  - `--table`: DynamoDB table name (defaults to `DYNAMO_COMMENT_TABLE` environment variable, or falls back to `comments`).
  - `--output`: Output YAML file path (defaults to `dynamodb_data.yaml`).

---

### 4. Mark Comments as Imported (`mark_imported.py`)
Reads comment IDs from a YAML comments file and updates their records in DynamoDB, setting the `imported` flag to `True`.

```bash
python3 mark_imported.py <yaml_file_path> [options]
```

* **Arguments**:
  - `yaml_file`: Path to the local comments `.yaml` file.
* **Options**:
  - `--table`: DynamoDB table name (defaults to `DYNAMO_COMMENT_TABLE` environment variable, or falls back to `comments`).

---

### 5. Read Private Comments (`read_private.py`)
Scans the DynamoDB comments table and prints all comments that are marked as private (`private = True`) in chronological order.

```bash
python3 read_private.py [options]
```

* **Options**:
  - `--table`: DynamoDB table name (defaults to `DYNAMO_COMMENT_TABLE` environment variable, or falls back to `comments`).
