import boto3
import yaml
import argparse
import sys
import os

def mark_comments_as_imported(yaml_filepath, table_name):
    # Check if file exists
    if not os.path.exists(yaml_filepath):
        print(f"Error: File '{yaml_filepath}' not found.")
        sys.exit(1)

    print(f"Reading comment IDs from '{yaml_filepath}'...")
    with open(yaml_filepath, 'r') as file:
        comments_data = yaml.safe_load(file)

    if not comments_data:
        print("No comments found in the YAML file.")
        return

    # Extract all comment IDs
    comment_ids = []
    for page, comments_list in comments_data.items():
        if isinstance(comments_list, list):
            for comment in comments_list:
                comment_id = comment.get('id')
                if comment_id:
                    comment_ids.append(comment_id)

    total_comments = len(comment_ids)
    if total_comments == 0:
        print("No comments with an 'id' attribute were found in the file.")
        return

    print(f"Found {total_comments} comments to mark as imported in DynamoDB table '{table_name}'.")
    
    # Initialize boto3 DynamoDB resource
    dynamodb = boto3.resource('dynamodb')
    table = dynamodb.Table(table_name)

    print("Updating DynamoDB items...")
    success_count = 0
    error_count = 0

    for index, comment_id in enumerate(comment_ids, 1):
        try:
            print(f"[{index}/{total_comments}] Marking comment ID: {comment_id}...", end="", flush=True)
            table.update_item(
                Key={'id': comment_id},
                UpdateExpression="SET imported = :val",
                ExpressionAttributeValues={':val': True}
            )
            print(" Success.")
            success_count += 1
        except Exception as e:
            print(f" Error: {e}")
            error_count += 1

    print("\nUpdate summary:")
    print(f"  Successfully marked: {success_count}")
    print(f"  Failed: {error_count}")
    print(f"  Total: {total_comments}")

def main():
    parser = argparse.ArgumentParser(description="Mark comments from a static YAML file as imported in DynamoDB.")
    parser.add_argument("yaml_file", help="Path to the static comments YAML file (e.g., dynamodb_data.yaml)")
    parser.add_argument("--table", default=os.environ.get("DYNAMO_COMMENT_TABLE", os.environ.get("DYNAMO_TABLE_NAME", "comments")), 
                        help="DynamoDB table name (defaults to DYNAMO_COMMENT_TABLE or 'comments')")
    args = parser.parse_args()

    mark_comments_as_imported(args.yaml_file, args.table)

if __name__ == '__main__':
    main()
