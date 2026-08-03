import json
import yaml
import glob
import os
import argparse
from urllib.parse import urlparse
from datetime import datetime, timezone

def read_comments(comments, filename):
    with open(filename, 'r') as f:
        js = json.load(f)

    for comment in js.get('data', []):
        post_link = urlparse(comment['url']).path
        # Avoid deprecated utcfromtimestamp in python 3.12+
        dt = datetime.fromtimestamp(int(int(comment['createdAt'])/1000), tz=timezone.utc)
        comment_date = dt.isoformat().replace("+00:00", "Z")
        comment_data = {
            'id': comment['id'],
            'author': comment['userDisplayName'],
            'email': comment['userId'],
            'date': comment_date,
            'comment': comment['body'],
        }
        if not post_link in comments:
            comments[post_link] = []
        comments[post_link].append(comment_data)

def main():
    parser = argparse.ArgumentParser(description="Convert CommentBox JSON export files to YAML comments format.")
    parser.add_argument("json_dir", help="Directory containing the CommentBox JSON export files")
    parser.add_argument("yaml_file", help="Path to the destination YAML file")
    
    args = parser.parse_args()

    comments = dict()
    json_files = glob.glob(os.path.join(args.json_dir, '*.json'))

    if not json_files:
        print(f"No JSON files found in directory '{args.json_dir}'.")
        return

    # Loop through each JSON file
    for json_file in json_files:
        read_comments(comments, json_file)
        print(f"Loaded data from {json_file}")

    # Write the data to a YAML file
    with open(args.yaml_file, 'w') as f:
        yaml.dump(comments, f, default_flow_style=False, sort_keys=False)

    print(f"JSON data has been written to {args.yaml_file} in YAML format.")

if __name__ == "__main__":
    main()
