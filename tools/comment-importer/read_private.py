import os
import boto3
import argparse
from boto3.dynamodb.conditions import Attr

def main():
    parser = argparse.ArgumentParser(description="Read private comments from DynamoDB.")
    parser.add_argument("--table", default=os.environ.get("DYNAMO_COMMENT_TABLE", os.environ.get("DYNAMO_TABLE_NAME", "comments")), 
                        help="DynamoDB table name (defaults to DYNAMO_COMMENT_TABLE or 'comments')")
    args = parser.parse_args()

    table_name = args.table

    dynamodb = boto3.resource('dynamodb')
    table = dynamodb.Table(table_name)
    
    print(f"Fetching private comments from DynamoDB table '{table_name}'...")
    response = table.scan(
        FilterExpression=Attr('private').eq(True)
    )
    items = response.get('Items', [])
    
    while 'LastEvaluatedKey' in response:
        response = table.scan(
            ExclusiveStartKey=response['LastEvaluatedKey'],
            FilterExpression=Attr('private').eq(True)
        )
        items.extend(response.get('Items', []))
        
    items.sort(key=lambda x: x.get('date', ''))
    
    print(f"\nFound {len(items)} private comments:\n")
    for item in items:
        print(f"Date:    {item.get('date')}")
        print(f"Author:  {item.get('author')}")
        print(f"Page:    {item.get('page')}")
        print(f"Content:\n{item.get('content')}")
        print("-" * 40)

if __name__ == '__main__':
    main()
