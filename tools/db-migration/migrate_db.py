import os
import uuid
import boto3

def get_dynamo_resource():
    endpoint_url = os.environ.get("AWS_ENDPOINT_URL")
    region_name = os.environ.get("AWS_REGION", "us-east-1")
    
    # Check for endpoint_url (e.g. LocalStack http://localhost:4566)
    if endpoint_url:
        print(f"Connecting to DynamoDB local at: {endpoint_url} (region: {region_name})")
        return boto3.resource(
            "dynamodb",
            endpoint_url=endpoint_url,
            region_name=region_name,
            aws_access_key_id=os.environ.get("AWS_ACCESS_KEY_ID", "mock"),
            aws_secret_access_key=os.environ.get("AWS_SECRET_ACCESS_KEY", "mock")
        )
    else:
        print(f"Connecting to production AWS DynamoDB (region: {region_name})")
        return boto3.resource("dynamodb", region_name=region_name)

def scan_all_items(table):
    items = []
    response = table.scan()
    items.extend(response.get('Items', []))
    while 'LastEvaluatedKey' in response:
        response = table.scan(ExclusiveStartKey=response['LastEvaluatedKey'])
        items.extend(response.get('Items', []))
    return items

def migrate():
    db = get_dynamo_resource()
    user_table_name = os.environ.get("DYNAMO_USER_TABLE", "endgameviable_users")
    comment_table_name = os.environ.get("DYNAMO_COMMENT_TABLE", "endgameviable_comments")
    
    user_table = db.Table(user_table_name)
    comment_table = db.Table(comment_table_name)
    
    print(f"Scanning users table: {user_table_name}")
    users = scan_all_items(user_table)
    print(f"Found {len(users)} users.")
    
    # Store mapped users in memory for comments migration lookup
    user_mapping = {}
    
    # 1. Migrate Users
    for user in users:
        author = user.get("author")
        if not author:
            continue
            
        modified = False
        
        # Ensure user has a user_id
        if "user_id" not in user:
            user["user_id"] = str(uuid.uuid4())
            print(f"Generated user_id UUID '{user['user_id']}' for user '{author}'")
            modified = True
            
        # Save changes if any modification was made (do NOT delete 'author_id')
        if modified:
            user_table.put_item(Item=user)
            print(f"Saved user: {author}")
            
        user_mapping[author] = user

    # 2. Migrate Comments
    print(f"Scanning comments table: {comment_table_name}")
    comments = scan_all_items(comment_table)
    print(f"Found {len(comments)} comments.")
    
    for comment in comments:
        comment_id = comment.get("id")
        author = comment.get("author")
        if not comment_id or not author:
            continue
            
        modified = False
        
        # Ensure comment is linked to a user_id and verified status
        if "user_id" not in comment or "verified" not in comment:
            # Look up the user record
            user_record = user_mapping.get(author)
            if not user_record:
                # User does not exist, fetch from DynamoDB to be sure
                try:
                    response = user_table.get_item(Key={"author": author})
                    user_record = response.get("Item")
                except Exception as e:
                    print(f"Error fetching user '{author}': {e}")
                    
            if not user_record:
                print(f"WARNING: Author '{author}' for comment {comment_id} not found in users table. Skipping comment.")
                continue
                
            # Set the foreign key link and verified status
            comment["user_id"] = user_record["user_id"]
            author_id = user_record.get("author_id", "")
            comment["verified"] = author_id.startswith("google:")
            
            print(f"Linked comment {comment_id} by '{author}' to user_id '{comment['user_id']}' (verified: {comment['verified']})")
            modified = True
            
        if modified:
            comment_table.put_item(Item=comment)
            print(f"Saved comment: {comment_id}")
            
    print("Database migration completed successfully!")

if __name__ == "__main__":
    migrate()
