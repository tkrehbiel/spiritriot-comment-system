import xml.etree.ElementTree as ET
import yaml
import html
import argparse
from urllib.parse import urlparse
from datetime import datetime

def extract_comments_from_wxr(wxr_file, prefix_template):
    # Parse the XML WXR file
    tree = ET.parse(wxr_file)
    root = tree.getroot()

    # Namespace dictionary to handle XML namespaces in WordPress WXR files
    ns = {
        'wp': 'http://wordpress.org/export/1.2/'
    }

    # List to store all the comments
    comments = dict()

    # Loop through each item (post/page) in the XML
    for item in root.findall('channel/item'):
        post_title = item.find('title').text
        post_link_base = urlparse(item.find('link').text).path
 
        # Find all comments associated with the post
        for comment in item.findall('wp:comment', ns):
            date_string = comment.find('wp:comment_date_gmt', ns).text
            dt = datetime.strptime(date_string, "%Y-%m-%d %H:%M:%S")
            
            # Format the link prefix dynamically (e.g. including the publication year)
            prefix = prefix_template.format(year=dt.year)
            post_link = f'{prefix}{post_link_base}'
            comment_date = dt.isoformat() + "Z"
            
            comment_data = {
                'post_title': html.unescape(post_title),
                'author': comment.find('wp:comment_author', ns).text,
                'email': comment.find('wp:comment_author_email', ns).text,
                'date': comment_date,
                'comment': html.unescape(comment.find('wp:comment_content', ns).text),
            }
            if not post_link in comments:
                comments[post_link] = []
            comments[post_link].append(comment_data)
    
    return comments

def write_comments_to_yaml(comments, output_file):
    with open(output_file, 'w') as yaml_file:
        yaml.dump(comments, yaml_file, default_flow_style=False, sort_keys=False)

# Main function to extract comments and save them to YAML
def main():
    parser = argparse.ArgumentParser(description="Extract comments from a WordPress/GraphComment WXR XML export and write to YAML.")
    parser.add_argument("wxr_file", help="Path to the source WXR XML file")
    parser.add_argument("output_file", help="Path to the destination YAML file")
    parser.add_argument("--prefix", default="/gaming/{year}", help="Prefix format template for post URLs (e.g. '/gaming/{year}')")
    
    args = parser.parse_args()

    # Extract comments from the WXR file
    comments = extract_comments_from_wxr(args.wxr_file, args.prefix)
    
    # Write comments to a YAML file
    write_comments_to_yaml(comments, args.output_file)
    print(f"Comments successfully written to {args.output_file}")

if __name__ == "__main__":
    main()
