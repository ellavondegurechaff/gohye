import os
import json
import boto3
from pathlib import Path
from datetime import datetime, timezone
from concurrent.futures import ThreadPoolExecutor, as_completed
from multiprocessing import Pool
import re
from collections import Counter
from pymongo import MongoClient
import logging
from bson import ObjectId

DO_SPACES_KEY = 'DO00942YPCJNT62UAZG6'
DO_SPACES_SECRET = 'kk83FHgUwR+MgtKJsOBYL1Zihjj7d9SEaaTqay8lKY8'
DO_SPACES_REGION = 'sfo2'
DO_SPACES_BUCKET = 'hyejoo'

ACCEPTED_EXTENSIONS = {".jpg", ".png", ".jpeg", ".gif"}  # Add .gif to the set of accepted extensions

session = boto3.session.Session()
s3_client = session.client('s3',
                           region_name=DO_SPACES_REGION,
                           aws_access_key_id=DO_SPACES_KEY,
                           aws_secret_access_key=DO_SPACES_SECRET,
                           endpoint_url=f'https://sfo2.digitaloceanspaces.com')

# Add MongoDB connection
client = MongoClient('mongodb+srv://hyejoo2:5x9dp863z0E4NJs7@hyejoo-664149b6.mongo.ondigitalocean.com/hyejoo2?replicaSet=hyejoo&tls=true&authSource=admin')
db = client.hyejoo2

class CardProcessingError(Exception):
    def __init__(self, message, error_code):
        super().__init__(message)
        self.error_code = error_code


class CustomJSONEncoder(json.JSONEncoder):
    def encode(self, obj):
        def float_formatter(o):
            if isinstance(o, float):
                return format(o, ".5f")
            raise TypeError(repr(o) + " is not JSON serializable")

        return super(CustomJSONEncoder, self).encode(self, default=float_formatter)
    
def check_name_duplicates(cards_file):
    print("Checking for name duplicates in the entire 'cards.json' file...")
    with open(cards_file, "r", encoding="utf-8") as f:
        cards_data = json.load(f)

    # Count the occurrences of each card name using a Counter object
    name_counter = Counter(card.get("name") for card in cards_data)

    # Find the set of card names that occur more than once
    duplicates = set(name for name, count in name_counter.items() if count > 1)

    if len(duplicates) > 0:
        print("The following card names have duplicates:")
        for name in duplicates:
            print(f"- {name}")
    else:
        print("No name duplicates were found.")

# def is_valid_filename(filename):
#     # Modify this regular expression to match the correct file name format
#     pattern = re.compile(r'\d+_\w+_\w+\.\w+')
#     return pattern.match(filename) is not None

def is_valid_filename(filename):
    # Modify this regular expression to match file names with one or more words
    # The second word and the underscore before it are now optional
    pattern = re.compile(r'\d+_\w+(\_\w+)?\.\w+')
    return pattern.match(filename) is not None

def rename_files(directory):
    for root, dirs, files in os.walk(directory):
        for file in files:
            file_path = os.path.join(root, file)
            file_name, file_ext = os.path.splitext(file)
            new_file_name = file_name.lower().replace(' ', '_')
            new_file_path = os.path.join(root, new_file_name + file_ext)
            os.rename(file_path, new_file_path)

def create_folder(folder_name, target_directory):
    print(f"Creating folder: cards/{target_directory}/{folder_name}")
    s3_client.put_object(Bucket=DO_SPACES_BUCKET, Key=f"cards/{target_directory}/{folder_name}/")
    print(f"Folder created: cards/{target_directory}/{folder_name}")

def upload_file(directory, file_path, folder_name, target_directory):
    logger = logging.getLogger(__name__)
    relative_path = Path(file_path).relative_to(directory).as_posix()
    
    if not is_valid_filename(relative_path):
        logger.warning(f"Invalid file name '{relative_path}'. Skipping file.")
        return None

    file_name, file_ext = os.path.splitext(relative_path)
    new_file_ext = file_ext if file_ext.lower() == ".gif" else ".jpg"
    relative_path_new_ext = file_name + new_file_ext

    logger.info(f"Uploading file: {file_path}")
    try:
        content_type = "image/gif" if file_ext.lower() == '.gif' else "image/jpeg"
        s3_path = f"cards/{target_directory}/{folder_name}/{relative_path_new_ext}"
        
        logger.debug(f"Uploading to S3 path: {s3_path}")
        s3_client.upload_file(
            file_path,
            DO_SPACES_BUCKET,
            s3_path,
            ExtraArgs={'ACL': 'public-read', 'ContentType': content_type}
        )
        logger.info(f"Successfully uploaded: {file_path}")
        return relative_path
        
    except Exception as e:
        logger.error(f"Error uploading file '{file_path}': {str(e)}", exc_info=True)
        raise CardProcessingError(f"Error uploading file '{file_path}'", 1) from e

def upload_files(directory, folder_name, target_directory):
    print(f"Uploading files to: cards/{target_directory}/{folder_name}")
    valid_files = []
    all_files = [os.path.join(root, file) for root, dirs, files in os.walk(directory) for file in files]

    with ThreadPoolExecutor() as executor:
        futures = {executor.submit(upload_file, directory, file_path, folder_name, target_directory): file_path for file_path in all_files}
        for future in as_completed(futures):
            result = future.result()
            if result is not None:
                valid_files.append(result)

    print(f"Finished uploading {len(valid_files)} files to: cards/{target_directory}/{folder_name}")
    return valid_files

def update_database(directory, target_directory, valid_files):
    try:
        folder_name = os.path.basename(directory)
        
        # Check if collection exists
        existing_collection = db.collections.find_one({'id': folder_name})
        new_collection_added = False

        if not existing_collection:
            new_collection = {
                'id': folder_name,
                'name': folder_name.upper(),
                'origin': None,
                'aliases': [folder_name],
                'promo': False,
                'compressed': True,
                'rarity': -1,
                'tags': [target_directory],
                'added': datetime.now(timezone.utc)
            }
            db.collections.insert_one(new_collection)
            new_collection_added = True
            print(f'Added new collection: {folder_name}')

        for file in valid_files:
            file_name, file_ext = os.path.splitext(file)
            is_animated = file_ext.lower() == '.gif'
            
            card_id = file_name.lower()
            level = int(card_id.split('_')[0])
            name_without_prefix = '_'.join(card_id.split('_')[1:])

            # Get the next available card ID
            last_card = db.cards.find_one(sort=[('id', -1)])
            next_id = (last_card['id'] + 1) if last_card else 0

            # Modified URL construction to match existing format
            base_path = f'/cards/{target_directory}/{folder_name}/{level}_{name_without_prefix}.jpg'
            url = f'https://hyejoo.sfo2.digitaloceanspaces.com{base_path}'
            shorturl = f'https://cards.hyejoobot.com{base_path}'

            new_card = {
                'id': next_id,
                'name': name_without_prefix,
                'level': level,
                'col': folder_name,
                'animated': is_animated,
                'tags': target_directory,
                'added': datetime.now(timezone.utc),
                'url': url,
                'shorturl': shorturl
            }

            db.cards.insert_one(new_card)
            print(f'Added new card: {name_without_prefix} with ID {next_id}')

        return True

    except Exception as e:
        print(f'Error updating database: {e}')
        raise CardProcessingError('Error updating database', 2) from e

def process_directory(input_directory, target_directory):
    logger = logging.getLogger(__name__)
    try:
        folder_name = os.path.basename(input_directory)
        logger.info(f'Starting to process directory: {input_directory}')
        logger.info(f'Collection name: {folder_name}')
        logger.info(f'Target directory: {target_directory}')

        # Create folder
        logger.info(f'Creating folder in DigitalOcean Spaces...')
        create_folder(folder_name, target_directory)
        
        # Rename files
        logger.info(f'Renaming files in directory...')
        rename_files(input_directory)
        
        # Upload files
        logger.info(f'Starting file upload process...')
        valid_files = upload_files(input_directory, folder_name, target_directory)
        logger.info(f'Successfully uploaded {len(valid_files)} files')
        
        # Update database
        logger.info(f'Updating database with new cards...')
        new_collection_added = update_database(input_directory, target_directory, valid_files)
        
        if new_collection_added:
            logger.info(f'Added new collection: {folder_name}')
        
        logger.info(f'Finished processing directory: {input_directory}')
        return valid_files, new_collection_added
        
    except CardProcessingError as e:
        logger.error(f'Error processing directory {input_directory}: {str(e)}', exc_info=True)
        return [], False
    except Exception as e:
        logger.error(f'Unexpected error processing directory {input_directory}: {str(e)}', exc_info=True)
        return [], False


    

# Add this new function to write the output file
def write_output_file(output_file, input_directory, valid_files, target_directory, error_messages):
    collection = os.path.basename(input_directory)
    
    with open(output_file, 'a') as f:
        f.write(f'Group Type: {target_directory}\n')
        f.write(f'Collection Name: {collection}\n')
        f.write(f'Number of Cards: {len(valid_files)}\n')
        f.write('Uploaded Files:\n')
        for file in valid_files:
            f.write(f'  {file}\n')

        # Get first and last card IDs from database
        first_card = db.cards.find_one({'col': collection}, sort=[('id', 1)])
        last_card = db.cards.find_one({'col': collection}, sort=[('id', -1)])
        
        f.write(f"First Card ID: {first_card['id'] if first_card else None}\n")
        f.write(f"Last Card ID: {last_card['id'] if last_card else None}\n")

        if error_messages:
            f.write('Errors:\n')
            for error_message in error_messages:
                f.write(f'  {error_message}\n')
        f.write('\n')

def setup_logging():
    # Create logs directory if it doesn't exist
    if not os.path.exists('logs'):
        os.makedirs('logs')
    
    # Setup logging with timestamp in filename
    timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(levelname)s - %(message)s',
        handlers=[
            logging.FileHandler(f'logs/process_{timestamp}.log'),
            logging.StreamHandler()  # This will print to console as well
        ]
    )
    return logging.getLogger(__name__)

def main(input_directories, output_file):
    logger = logging.getLogger(__name__)
    logger.info('Starting card processing script')
    logger.info(f'Output file: {output_file}')
    
    for target_directory in ['girlgroups', 'boygroups']:
        logger.info(f'Processing {target_directory}...')
        logger.info(f'Found {len(input_directories[target_directory])} directories to process')
        
        for input_directory in input_directories[target_directory]:
            logger.info(f'Processing directory: {input_directory}')
            error_messages = []
            
            try:
                valid_files, new_collection_added = process_directory(input_directory, target_directory)
                write_output_file(output_file, input_directory, valid_files, target_directory, error_messages)
                logger.info(f'Successfully processed directory: {input_directory}')
                
            except Exception as e:
                error_message = f'Error in process for {input_directory}: {str(e)}'
                logger.error(error_message, exc_info=True)
                error_messages.append(error_message)
                continue
    
    logger.info('Card processing script completed')


if __name__ == "__main__":
    import sys
    
    # Setup logging
    logger = setup_logging()
    
    if len(sys.argv) != 4:
        logger.error("Incorrect number of arguments!")
        logger.info("Usage: python automate.py [girlgroups_directory] [boygroups_directory] [output_file]")
        sys.exit(1)
        
    girlgroups_directory = sys.argv[1]
    boygroups_directory = sys.argv[2]
    output_file = sys.argv[3]
    
    logger.info(f'Starting script with:')
    logger.info(f'Girl Groups Directory: {girlgroups_directory}')
    logger.info(f'Boy Groups Directory: {boygroups_directory}')
    logger.info(f'Output File: {output_file}')

    input_directories = {
        'girlgroups': [os.path.join(girlgroups_directory, folder) 
                      for folder in os.listdir(girlgroups_directory) 
                      if os.path.isdir(os.path.join(girlgroups_directory, folder))],
        'boygroups': [os.path.join(boygroups_directory, folder) 
                     for folder in os.listdir(boygroups_directory) 
                     if os.path.isdir(os.path.join(boygroups_directory, folder))],
    }

    main(input_directories, output_file)


