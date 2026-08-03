import os
from boto3.dynamodb.conditions import Attr
import boto3
import yaml
import argparse

# Function to connect to DynamoDB and read all items from the table
def read_dynamodb_table(table_name):
    # Initialize the DynamoDB resource using Boto3
    dynamodb = boto3.resource('dynamodb')
    
    # Connect to the specified table
    table = dynamodb.Table(table_name)
    
    projection_expression = '#pg, #dt, #au, #co, #id, #pv'
    expression_attribute_names = {
        "#pg": "page",
        "#dt": "date",
        "#au": "author",
        "#co": "content",
        "#id": "id",
        "#pv": "private",
    }
    
    # Scan the table to get all items (scan reads the entire table)
    response = table.scan(
        ProjectionExpression=projection_expression,
        ExpressionAttributeNames=expression_attribute_names,
    )
    data = response.get('Items', [])
    
    # Handling pagination if there are more items
    while 'LastEvaluatedKey' in response:
        response = table.scan(
            ExclusiveStartKey=response['LastEvaluatedKey'],
            ProjectionExpression=projection_expression,
            ExpressionAttributeNames=expression_attribute_names,
        )
        data.extend(response.get('Items', []))
    
    return data

# Function to save the data to a YAML file
def save_to_yaml_file(data, filename):
    with open(filename, 'w') as file:
        yaml.dump(data, file, default_flow_style=False, sort_keys=False)

# Main function
def main():
    # Setup command line argument parsing
    parser = argparse.ArgumentParser(description="Backup comments from DynamoDB prior to a specified cutoff date.")
    parser.add_argument("cutoff_date", help="Cutoff date in YYYY-MM-DD format (comments prior to this date will be imported)")
    parser.add_argument("--table", default=os.environ.get("DYNAMO_COMMENT_TABLE", os.environ.get("DYNAMO_TABLE_NAME", "comments")), 
                        help="DynamoDB table name (defaults to DYNAMO_COMMENT_TABLE or 'comments')")
    parser.add_argument("--output", default="dynamodb_data.yaml", help="Destination YAML file path")
    args = parser.parse_args()

    cutoff_date = args.cutoff_date
    table_name = args.table
    output_file = args.output

    print(f"Importing comments dated prior to {cutoff_date} from table '{table_name}'...")

    # Read all items from the DynamoDB table
    raw_data = read_dynamodb_table(table_name)
    
    # Filter comments by date and ensure they are not private
    filtered_data = [item for item in raw_data if item.get('date', '') < cutoff_date and not item.get('private', False)]
    print(f"Found {len(filtered_data)} comments prior to {cutoff_date} (out of {len(raw_data)} total).")

    # Sort all filtered comments chronologically (ascending by date) in their entirety
    filtered_data.sort(key=lambda x: x.get('date', ''))

    comments = dict()
    last_date = ''
    for comment in filtered_data:
        date_str = comment.get('date', '')
        if date_str > last_date:
            last_date = date_str
            
        page = comment.get('page', '')
        if not page:
            continue
            
        if page not in comments:
            comments[page] = []
            
        comments[page].append({
            'id': comment.get('id', ''),
            'date': date_str,
            'author': comment.get('author', ''),
            'comment': comment.get('content', ''),
        })

    # Save the data to a local YAML file
    save_to_yaml_file(comments, output_file)
    
    print(f"Data from DynamoDB table '{table_name}' has been saved to '{output_file}'.")
    if last_date:
        print(f"Most recent imported comment date: {last_date}")
    
if __name__ == '__main__':
    main()
